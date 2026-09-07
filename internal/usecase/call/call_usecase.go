// Package call implements the CallUseCase: the brain of the call subsystem.
//
// All state transitions go through the in-package state machines and through
// CallRepo's CAS-guarded UpdateState, so the DB and the in-memory domain.Call
// can never drift.
package call

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/infrastructure/asterisk"
	"github.com/rs/zerolog"
)

// UseCase is the call orchestrator. One per process.
type UseCase struct {
	repo     domain.CallRepository
	queue    domain.CallQueueManager
	gateway  domain.AsteriskGateway
	hub      WebsocketBroadcaster
	caseRepo domain.CaseRepository // optional — used to mirror the call into
	// the chat-case list (the admin UI reads /api/admin/cases). May be nil
	// during early startup wiring or in tests that don't exercise the case
	// pipeline; mirror helpers must therefore nil-check before use.
	logger zerolog.Logger

	ringTimeout      time.Duration
	maxDuration      time.Duration
	queueTTL         time.Duration
	recordingEnabled bool

	mu      sync.Mutex
	pending map[uuid.UUID]pendingARI // callID → ARI in-flight info

	// rootCtx / rootCancel bind long-running goroutines (ring timeouts) to
	// the server lifecycle. When rootCancel is called the goroutines exit.
	rootCtx    context.Context
	rootCancel context.CancelFunc
}

// pendingARI tracks which channels we have outstanding for a call during
// CONNECTING/RINGING so we know when both are answered.
type pendingARI struct {
	customerCh string
	agentCh    string
	bridgeID   string
	answered   int // number of channels that have answered
}

// WebsocketBroadcaster is the narrow contract the use case needs from the WS hub.
// We accept an interface to keep delivery layer decoupled.
type WebsocketBroadcaster interface {
	BroadcastToSession(sessionID string, event *domain.WSEvent)
	BroadcastToChannel(channel string, event *domain.WSEvent)
}

// Config bundles runtime knobs.
type Config struct {
	RingTimeout      time.Duration
	MaxDuration      time.Duration
	QueueTTL         time.Duration
	RecordingEnabled bool
}

// NewUseCase constructs the orchestrator. The returned use case owns a
// background context for long-running helpers (ring timeouts); callers MUST
// invoke Shutdown() during graceful shutdown to stop those goroutines.
//
// caseRepo is optional — pass nil if the chat-case pipeline is not in use.
// When provided, the use case mirrors incoming call events into the
// admin's case list (so a guest pressing the call button shows up at
// /api/admin/cases without having to send a chat message first).
func NewUseCase(
	repo domain.CallRepository,
	queue domain.CallQueueManager,
	gw domain.AsteriskGateway,
	hub WebsocketBroadcaster,
	caseRepo domain.CaseRepository,
	cfg Config,
	logger zerolog.Logger,
) *UseCase {
	if cfg.RingTimeout <= 0 {
		cfg.RingTimeout = 30 * time.Second
	}
	if cfg.MaxDuration <= 0 {
		cfg.MaxDuration = 30 * time.Minute
	}
	if cfg.QueueTTL <= 0 {
		cfg.QueueTTL = 10 * time.Minute
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &UseCase{
		repo:             repo,
		queue:            queue,
		gateway:          gw,
		hub:              hub,
		caseRepo:         caseRepo,
		ringTimeout:      cfg.RingTimeout,
		maxDuration:      cfg.MaxDuration,
		queueTTL:         cfg.QueueTTL,
		recordingEnabled: cfg.RecordingEnabled,
		logger:           logger.With().Str("usecase", "call").Logger(),
		pending:          make(map[uuid.UUID]pendingARI),
		rootCtx:          ctx,
		rootCancel:       cancel,
	}
}

// Shutdown cancels all background goroutines spawned by the use case
// (ring timeouts, etc.). It is safe to call multiple times.
func (uc *UseCase) Shutdown(_ context.Context) error {
	uc.rootCancel()
	return nil
}

// ----------------------------------------------------------------
// API 1: Customer requests a call
// ----------------------------------------------------------------

// RequestCallInput is the input for POST /api/calls.
type RequestCallInput struct {
	CustomerID     string
	Priority       int
	IdempotencyKey string // optional, from header
}

// RequestCallOutput is the response body for POST /api/calls.
type RequestCallOutput struct {
	CallID   string `json:"call_id"`
	Status   string `json:"status"`
	Position int64  `json:"queue_position"`
	Replay   bool   `json:"idempotent_replay,omitempty"`
}

// RequestCall is the entry point for customer-initiated calls.
//
// Flow:
//  1. Atomically claim the idempotency key. If another concurrent request
//     already claimed it, replay the cached response.
//  2. Create call row (CREATED → WAITING) and enqueue.
//  3. Attempt immediate agent assignment.
//  4. Best-effort routing trigger (wakeup the queue worker).
//  5. Update the idempotency record with the actual response.
func (uc *UseCase) RequestCall(ctx context.Context, in RequestCallInput) (*RequestCallOutput, error) {
	if in.CustomerID == "" {
		return nil, errors.New("customer_id is required")
	}

	// 1. Atomic idempotency claim. This prevents two concurrent retries from
	//    both creating separate calls — exactly one will win the claim, the
	//    others will replay the cached response (filled in by the winner).
	if in.IdempotencyKey != "" {
		claimed, err := uc.repo.ClaimIdempotency(ctx, in.IdempotencyKey)
		if err != nil {
			uc.logger.Warn().Err(err).Msg("idempotency claim failed; falling through to creation")
		} else if !claimed {
			// Someone else is creating (or has created) this call. Poll briefly
			// for the winner to populate the response_body, then replay it.
			return uc.replayIdempotent(ctx, in.IdempotencyKey)
		}
	}

	// 2. Persist call (CREATED).
	c := &domain.Call{
		ID:         uuid.New(),
		CustomerID: in.CustomerID,
		Priority:   in.Priority,
		Status:     domain.CallStatusCreated,
	}
	if err := uc.repo.Insert(ctx, c); err != nil {
		return nil, fmt.Errorf("create call: %w", err)
	}

	// CREATED → WAITING
	if err := uc.transition(ctx, c, domain.CallStatusWaiting); err != nil {
		return nil, err
	}

	// 3. Enqueue (score = priority*1e13 + now_ms)
	now := time.Now().UnixMilli()
	if err := uc.queue.Enqueue(ctx, c.ID, c.Priority, now); err != nil {
		uc.logger.Warn().Err(err).Str("call_id", c.ID.String()).Msg("enqueue failed; call remains in WAITING")
	}

	// Wake up the routing worker.
	uc.notifyRouter(c.ID)

	// 4. Compute current position (best effort).
	pos, _ := uc.queue.QueuePosition(ctx, c.ID)

	out := &RequestCallOutput{
		CallID:   c.ID.String(),
		Status:   string(domain.CallStatusWaiting),
		Position: pos,
	}

	// Cache response for idempotency.
	if in.IdempotencyKey != "" {
		_ = uc.repo.SaveIdempotency(ctx, &domain.IdempotencyRecord{
			Key:          in.IdempotencyKey,
			CallID:       c.ID,
			ResponseBody: map[string]any{"call_id": out.CallID, "status": out.Status, "queue_position": out.Position},
		})
	}

	return out, nil
}

// ----------------------------------------------------------------
// Routing: worker pulls the next call and tries to reserve an agent
// ----------------------------------------------------------------

// TryRouteNext is invoked by the routing worker (or triggered after
// an agent becomes AVAILABLE). It pops the head of the queue and tries
// to find an available agent.
func (uc *UseCase) TryRouteNext(ctx context.Context) error {
	cid, ok, err := uc.queue.PopNext(ctx)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	return uc.AssignAgentForCall(ctx, cid)
}

// AssignAgentForCall picks the first AVAILABLE agent and reserves them.
// If no agent is available, the call is re-enqueued at the head.
//
// Dedup: if the call was already announced (IsAnnounced=true — set by a
// previous routing attempt that successfully reserved an agent), we skip
// re-publishing `incoming_call` to admins. The agent state still gets the
// ring timeout reset on retries (because the timer was reset when the call
// was re-enqueued) but the admin's banner does NOT reappear every
// ring-timeout cycle.
func (uc *UseCase) AssignAgentForCall(ctx context.Context, callID uuid.UUID) error {
	// Find any available agent. (For v1, simple ZRANGE; future: skill-based.)
	agents, err := uc.queue.ListAvailable(ctx)
	if err != nil {
		return fmt.Errorf("list available: %w", err)
	}
	if len(agents) == 0 {
		// Re-enqueue at head: use score = 0 to keep it ahead of newer calls.
		_ = uc.queue.Enqueue(ctx, callID, 0, time.Now().UnixMilli()-1)
		return nil
	}

	for _, agentID := range agents {
		ok, state, err := uc.queue.ReserveAgent(ctx, agentID, callID)
		if err != nil {
			uc.logger.Warn().Err(err).Str("agent", agentID).Msg("reserve failed; trying next")
			continue
		}
		if !ok {
			uc.logger.Debug().Str("agent", agentID).Str("state", string(state)).Msg("agent not available")
			continue
		}
		// Won the race → persist the assignment.
		if err := uc.repo.AssignAgent(ctx, callID, agentID); err != nil {
			// Roll back Redis state.
			_, _ = uc.queue.ReleaseAgent(ctx, agentID)
			return err
		}

		// Mirror the incoming call into the chat-case pipeline so the
		// guest shows up at GET /api/admin/cases. The legacy InitiateCall
		// path did this eagerly; here we wait until we have a confirmed
		// reservation so we don't create ghost cases for un-routable calls.
		uc.mirrorCallToCase(ctx, callID, agentID)

		// Publish the incoming_call event exactly once per WAITING_AGENT phase.
		announceTTL := uc.ringTimeout + 30*time.Second
		already, _ := uc.queue.IsAnnounced(ctx, callID)
		if !already {
			uc.publishAgentEvent(callID, agentID, "incoming_call")
			// Mark immediately (best effort). If this fails the worst case is
			// the next retry publishes again — same behaviour as before.
			if err := uc.queue.MarkAnnounced(ctx, callID, announceTTL); err != nil {
				uc.logger.Warn().Err(err).Str("call_id", callID.String()).Msg("MarkAnnounced failed")
			}
		}
		uc.armRingTimeout(callID, agentID)

		return nil
	}
	return nil
}

// armRingTimeout schedules a goroutine that reverts the agent to AVAILABLE
// and routes to the next call if the agent doesn't accept within the timeout.
//
// The goroutine binds to the use case's root context so it exits cleanly on
// Shutdown() and never leaks when the server stops.
func (uc *UseCase) armRingTimeout(callID uuid.UUID, agentID string) {
	go func() {
		timer := time.NewTimer(uc.ringTimeout)
		defer timer.Stop()
		select {
		case <-uc.rootCtx.Done():
			return
		case <-timer.C:
		}

		// Use a short-lived context so we don't hang on context.Background().
		ctx, cancel := context.WithTimeout(uc.rootCtx, 5*time.Second)
		defer cancel()

		// Was the agent still RESERVED for this call?
		a, err := uc.queue.GetAgent(ctx, agentID)
		if err != nil || a == nil || a.State != domain.AgentReserved || (a.CurrentCall != nil && *a.CurrentCall != callID) {
			return // already accepted or moved on
		}

		// Revert to AVAILABLE.
		_, _ = uc.queue.ReleaseAgent(ctx, agentID)
		// Record MISSED + retry.
		_ = uc.repo.AppendEvent(ctx, &domain.CallEvent{
			CallID:     callID,
			EventType:  "call.timeout",
			Source:     domain.CallEventSourceSystem,
			AgentID:    agentID,
			Payload:    map[string]any{"reason": "ring_timeout"},
			OccurredAt: time.Now().UTC(),
		})

		// Move call back to WAITING and re-enqueue.
		_ = uc.transitionRaw(ctx, callID, domain.CallStatusWaitingAgent, domain.CallStatusWaiting, nil)
		_ = uc.queue.RemoveFromQueue(ctx, callID)
		// After timeout the previous announce should NOT carry over — the
		// agent missed, so we want the next attempt to fire fresh
		// `incoming_call` to admin/agent if a different agent picks it up.
		_ = uc.queue.ClearAnnounced(ctx, callID)
		_ = uc.queue.Enqueue(ctx, callID, 0, time.Now().UnixMilli())

		uc.notifyRouter(callID)
	}()
}

// ----------------------------------------------------------------
// API 2: Agent accepts
// ----------------------------------------------------------------

// AcceptCallInput for POST /api/calls/{id}/accept.
type AcceptCallInput struct {
	CallID  uuid.UUID
	AgentID string
}

// AcceptCall transitions RESERVED → RINGING and asks ARI to originate channels.
func (uc *UseCase) AcceptCall(ctx context.Context, in AcceptCallInput) error {
	if in.AgentID == "" || in.CallID == uuid.Nil {
		return errors.New("call_id and agent_id are required")
	}

	// DB-level idempotency.
	err := uc.repo.RecordParticipant(ctx, &domain.CallParticipant{
		CallID:        in.CallID,
		ParticipantID: in.AgentID,
		Role:          domain.CallPartyAgent,
		Action:        domain.CallActionAccept,
	})
	if errors.Is(err, domain.ErrDuplicateAction) {
		return nil // already accepted, idempotent success
	}
	if err != nil {
		return err
	}

	// Verify the agent is still RESERVED for this call.
	//
	// In some races the reservation has already been released (e.g. the
	// ring-timeout goroutine fired, the agent's WS heartbeat stalled and
	// triggered reconciliation, or a different assignment picked this
	// agent for another call after the banner was already painted).
	// When that happens the agent's banner UI may still be showing this
	// call id, so the operator hits Accept and gets a confusing
	// "agent reservation belongs to another call" error even though
	// nothing has actually gone wrong.
	//
	// Self-heal: if the agent is currently AVAILABLE we silently
	// (re-)reserve them for this call and proceed. Only when the agent
	// is RESERVED for *some other* call (i.e. someone else legitimately
	// owns them) do we surface the stale-reservation error.
	a, err := uc.queue.GetAgent(ctx, in.AgentID)
	if err != nil {
		return err
	}
	if a.State == domain.AgentAvailable {
		// Reservation expired / was released — re-acquire it for this
		// call so the operator's click is honoured instead of returning
		// a stale-reservation error.
		ok, _, rerr := uc.queue.ReserveAgent(ctx, in.AgentID, in.CallID)
		if rerr != nil {
			return rerr
		}
		if !ok {
			return domain.ErrAgentStaleReservation
		}
	} else if a.State != domain.AgentReserved || a.CurrentCall == nil || *a.CurrentCall != in.CallID {
		// Some other call has this agent; the operator's banner is
		// stale. Surface the error so they reload.
		return domain.ErrAgentStaleReservation
	}

	// WAITING_AGENT → CONNECTING
	c, err := uc.repo.Get(ctx, in.CallID)
	if err != nil || c == nil {
		return domain.ErrCallNotFound
	}
	if c.Status != domain.CallStatusWaitingAgent {
		return domain.ErrInvalidTransition
	}
	if err := uc.transition(ctx, c, domain.CallStatusConnecting); err != nil {
		return err
	}

	// Pre-flight ARI.
	if err := uc.gateway.HealthCheck(ctx); err != nil {
		// Fail the call gracefully.
		uc.failCall(ctx, in.CallID, "asterisk_unavailable")
		return err
	}

	// Create bridge + originate both channels.
	bridgeID, err := uc.gateway.CreateBridge(ctx, "mixing")
	if err != nil {
		uc.failCall(ctx, in.CallID, "bridge_create_failed")
		return err
	}
	customerCh, err := uc.gateway.OriginateCustomer(ctx, in.CallID, asterisk.CustomerEndpoint(c.CustomerID))
	if err != nil {
		uc.failCall(ctx, in.CallID, "customer_originate_failed")
		return err
	}
	agentCh, err := uc.gateway.OriginateAgent(ctx, in.CallID, in.AgentID)
	if err != nil {
		_ = uc.gateway.Hangup(ctx, in.CallID)
		uc.failCall(ctx, in.CallID, "agent_originate_failed")
		return err
	}

	// Persist ARI bridge and pending channels.
	uc.mu.Lock()
	uc.pending[in.CallID] = pendingARI{
		customerCh: customerCh,
		agentCh:    agentCh,
		bridgeID:   bridgeID,
	}
	uc.mu.Unlock()

	bridgeStr := bridgeID
	_ = uc.repo.UpdateState(ctx, in.CallID, domain.CallStatusConnecting, domain.CallStatusConnecting,
		domain.StateTransitionFields{ARIBridgeID: &bridgeStr})

	// Call has moved past WAITING_AGENT — clear dedup so any future
	// routing (e.g. retry-on-failure) would re-announce cleanly.
	_ = uc.queue.ClearAnnounced(ctx, in.CallID)

	uc.publishCallEvent(c, "call_connecting", nil)

	return nil
}

// ----------------------------------------------------------------
// API 3: Agent rejects
// ----------------------------------------------------------------

// RejectCallInput for POST /api/calls/{id}/reject.
type RejectCallInput struct {
	CallID  uuid.UUID
	AgentID string
}

// RejectCall reverts the agent to AVAILABLE and re-queues the call.
func (uc *UseCase) RejectCall(ctx context.Context, in RejectCallInput) error {
	if in.AgentID == "" || in.CallID == uuid.Nil {
		return errors.New("call_id and agent_id are required")
	}

	// Idempotency.
	err := uc.repo.RecordParticipant(ctx, &domain.CallParticipant{
		CallID:        in.CallID,
		ParticipantID: in.AgentID,
		Role:          domain.CallPartyAgent,
		Action:        domain.CallActionReject,
	})
	if errors.Is(err, domain.ErrDuplicateAction) {
		return nil
	}
	if err != nil {
		return err
	}

	if _, err := uc.queue.ReleaseAgent(ctx, in.AgentID); err != nil {
		uc.logger.Warn().Err(err).Msg("release agent on reject failed")
	}

	c, _ := uc.repo.Get(ctx, in.CallID)
	if c == nil {
		return nil
	}
	if c.Status.IsTerminal() {
		return nil
	}

	// Check whether there are OTHER available agents before deciding to
	// re-queue. Without this guard, when only one agent is online and
	// that agent rejects, the router immediately re-picks the same agent
	// (now AVAILABLE again) and fires `incoming_call` again — an infinite
	// reject→ring loop visible to the admin as the banner "bouncing back"
	// a second after rejection.
	othersAvailable, _ := uc.queue.ListAvailable(ctx)
	hasOtherAgent := false
	for _, a := range othersAvailable {
		if a != in.AgentID {
			hasOtherAgent = true
			break
		}
	}

	if hasOtherAgent {
		// WAITING_AGENT → WAITING and re-enqueue at head for the next agent.
		if c.Status == domain.CallStatusWaitingAgent {
			_ = uc.transition(ctx, c, domain.CallStatusWaiting)
		}
		// Allow the next routing attempt to publish a fresh incoming_call.
		_ = uc.queue.ClearAnnounced(ctx, in.CallID)
		// Re-enqueue at head.
		_ = uc.queue.Enqueue(ctx, in.CallID, 0, time.Now().UnixMilli()-1)
		uc.notifyRouter(in.CallID)
		return nil
	}

	// No other agent available — terminate the call so the customer
	// also stops ringing. This matches user expectation that "reject"
	// ends the call for the customer when there's nobody else to pick up.
	uc.logger.Info().
		Str("call_id", in.CallID.String()).
		Str("agent", in.AgentID).
		Msg("reject with no other agents; cancelling call for customer")

	// Move to CANCELLED via the same path CancelCall uses, but driven by
	// the system on behalf of the customer. We keep the call row for
	// audit but flip to terminal so ARI / hub stop firing for it.
	if c.Status != domain.CallStatusCancelled {
		_ = uc.queue.RemoveFromQueue(ctx, in.CallID)
		_ = uc.queue.ClearAnnounced(ctx, in.CallID)
		_ = uc.transition(ctx, c, domain.CallStatusCancelled)
	}
	// Mirror the cancel to the customer's channel so the phone stops
	// ringing, and to admin_inbox so the banner clears. publishCallEvent
	// already handles both channels.
	uc.publishCallEvent(c, "call_ended_v2", nil)
	uc.notifyRouter(in.CallID)
	return nil
}

// ----------------------------------------------------------------
// API 4: Customer or agent hangs up
// ----------------------------------------------------------------

// HangupCallInput for POST /api/calls/{id}/hangup.
type HangupCallInput struct {
	CallID  uuid.UUID
	ByAgent bool
	UserID  string
}

// HangupCall ends the call gracefully. Safe to call when already terminal.
func (uc *UseCase) HangupCall(ctx context.Context, in HangupCallInput) error {
	if in.CallID == uuid.Nil {
		return errors.New("call_id is required")
	}
	role := domain.CallPartyCustomer
	if in.ByAgent {
		role = domain.CallPartyAgent
	}
	_ = uc.repo.RecordParticipant(ctx, &domain.CallParticipant{
		CallID:        in.CallID,
		ParticipantID: in.UserID,
		Role:          role,
		Action:        domain.CallActionHangup,
	})

	c, err := uc.repo.Get(ctx, in.CallID)
	if err != nil {
		return err
	}
	if c == nil || c.Status.IsTerminal() {
		return nil // idempotent no-op
	}
	if c.StartedAt != nil && !c.StartedAt.IsZero() {
		c.DurationSeconds = int(time.Since(*c.StartedAt).Seconds())
	}
	endedAt := time.Now().UTC()
	if err := uc.transitionWithFields(ctx, c, domain.CallStatusEnded, domain.StateTransitionFields{EndedAt: &endedAt, DurationSeconds: &c.DurationSeconds}); err != nil {
		return err
	}

	// Tell ARI.
	_ = uc.gateway.StopRecording(ctx, in.CallID)
	_ = uc.gateway.Hangup(ctx, in.CallID)

	// Free the agent.
	if c.AgentID != nil {
		_, _ = uc.queue.ReleaseAgent(ctx, *c.AgentID)
	}
	_ = uc.queue.ClearAnnounced(ctx, in.CallID)

	uc.publishCallEvent(c, "call_ended_v2", nil)

	// Wake up routing for next call.
	uc.notifyRouter(in.CallID)
	return nil
}

// ----------------------------------------------------------------
// API 5: Customer cancels while in queue / waiting for agent
// ----------------------------------------------------------------

// CancelCallInput for POST /api/calls/{id}/cancel.
type CancelCallInput struct {
	CallID     uuid.UUID
	CustomerID string
}

// CancelCall removes a call from the queue.
func (uc *UseCase) CancelCall(ctx context.Context, in CancelCallInput) error {
	if in.CallID == uuid.Nil || in.CustomerID == "" {
		return errors.New("call_id and customer_id are required")
	}

	_ = uc.repo.RecordParticipant(ctx, &domain.CallParticipant{
		CallID:        in.CallID,
		ParticipantID: in.CustomerID,
		Role:          domain.CallPartyCustomer,
		Action:        domain.CallActionCancel,
	})

	c, err := uc.repo.Get(ctx, in.CallID)
	if err != nil {
		return err
	}
	if c == nil || c.Status.IsTerminal() {
		return nil
	}
	if c.CustomerID != in.CustomerID {
		return errors.New("permission denied: not call owner")
	}

	_ = uc.queue.RemoveFromQueue(ctx, in.CallID)
	_ = uc.queue.ClearAnnounced(ctx, in.CallID)
	_ = uc.transition(ctx, c, domain.CallStatusCancelled)

	if c.AgentID != nil {
		_, _ = uc.queue.ReleaseAgent(ctx, *c.AgentID)
	}

	uc.publishCallEvent(c, "call_ended_v2", nil)
	return nil
}

// CustomerDisconnected is invoked by the WS layer when a customer's
// WebSocket drops while they have an in-flight call. For WAITING calls it
// removes them from the queue and cancels the call. For calls already
// past CONNECTING the browser/Asterisk flow drives the rest — we leave
// the call alone so ARI StasisEnd will end it normally.
//
// This is the implementation of the architecture rule:
// "Customer disconnect khi đang WAITING → Remove from Redis Queue, Call → CANCELLED".
func (uc *UseCase) CustomerDisconnected(ctx context.Context, customerID string) {
	if customerID == "" {
		return
	}
	// Cancel any active non-terminal calls owned by this customer.
	// We try Cancel for each via the repository — it is a no-op for terminal
	// or already-cancelled calls so it is safe to call repeatedly.
	// The repo exposes ListByCustomer; reuse it.
	if uc.repo == nil {
		return
	}
	calls, err := uc.repo.ListByCustomer(ctx, customerID, 50, 0)
	if err != nil {
		uc.logger.Warn().Err(err).Str("customer", customerID).Msg("customer disconnect: list calls failed")
		return
	}
	for _, c := range calls {
		if c == nil || c.Status.IsTerminal() {
			continue
		}
		// Only auto-cancel while still waiting. Past CONNECTING, ARI owns the call.
		if c.Status != domain.CallStatusWaiting && c.Status != domain.CallStatusWaitingAgent &&
			c.Status != domain.CallStatusCreated {
			continue
		}
		if err := uc.CancelCall(ctx, CancelCallInput{CallID: c.ID, CustomerID: customerID}); err != nil {
			uc.logger.Warn().Err(err).Str("call_id", c.ID.String()).Msg("customer disconnect: cancel failed")
		}
	}
}

// ----------------------------------------------------------------
// ARI event handler (called by the WS consumer)
// ----------------------------------------------------------------

// HandleARIEvent is the entry point invoked by asterisk.WSConsumer.
//
// Every event also produces a `call_event` audit row. Out-of-order delivery
// is tolerated by checking the current state machine position before
// attempting any transition — invalid transitions are silently dropped so
// the system never regresses a state.
func (uc *UseCase) HandleARIEvent(ctx context.Context, ev domain.ARIEvent) {
	callIDStr, _ := ev.Variables["CALL_ID"].(string)
	if callIDStr == "" {
		return
	}
	callID, err := uuid.Parse(callIDStr)
	if err != nil {
		return
	}

	switch ev.Type {
	case "StasisStart":
		uc.handleStasisStart(ctx, callID, ev)
	case "ChannelStateChange":
		uc.handleChannelStateChange(ctx, callID, ev)
	case "StasisEnd":
		uc.handleStasisEnd(ctx, callID, ev)
	case "ChannelEnteredBridge":
		// We rely on ChannelStateChange for answered detection. No-op here.
	case "RecordingStarted":
		_ = uc.repo.AppendEvent(ctx, &domain.CallEvent{
			CallID:     callID,
			EventType:  "ari.recording_started",
			Source:     domain.CallEventSourceARI,
			Payload:    map[string]any{"recording": ev.Recording},
			OccurredAt: ev.Timestamp,
		})
	case "RecordingFinished":
		uc.handleRecordingFinished(ctx, callID, ev)
	case "BridgeDestroyed":
		// Just log.
		_ = uc.repo.AppendEvent(ctx, &domain.CallEvent{
			CallID:     callID,
			EventType:  "ari.bridge_destroyed",
			Source:     domain.CallEventSourceARI,
			Payload:    map[string]any{"bridge_id": ev.BridgeID},
			OccurredAt: ev.Timestamp,
		})
	}

	// Every event becomes an audit row. ON CONFLICT on
	// (call_id, event_type, occurred_at) means duplicate ARI events are
	// stored once — no duplicate rows.
	_ = uc.repo.AppendEvent(ctx, &domain.CallEvent{
		CallID:     callID,
		EventType:  "ari." + ev.Type,
		Source:     domain.CallEventSourceARI,
		Payload:    map[string]any{"event": ev},
		OccurredAt: ev.Timestamp,
	})
}

func (uc *UseCase) handleStasisStart(ctx context.Context, callID uuid.UUID, ev domain.ARIEvent) {
	c, err := uc.repo.Get(ctx, callID)
	if err != nil || c == nil {
		return
	}
	// StasisStart arrives after the channel has entered the Stasis app. The
	// legal transitions from here are:
	//   CONNECTING → RINGING : the channel is up but not yet answered.
	// If the call has already progressed past RINGING we ignore the event
	// (out-of-order duplicate).
	if c.Status == domain.CallStatusConnecting {
		_ = uc.transition(ctx, c, domain.CallStatusRinging)
		uc.publishCallEvent(c, "call_ringing_v2", nil)
		return
	}
	// If ARI delivers StasisStart after the call is already IN_PROGRESS
	// (e.g. bridge redial) we keep going. Anything else (terminal state
	// or pre-CONNECTING) is silently ignored.
}

func (uc *UseCase) handleChannelStateChange(ctx context.Context, callID uuid.UUID, ev domain.ARIEvent) {
	if ev.Channel == nil {
		return
	}
	if ev.Channel.State != "Up" {
		return
	}
	uc.mu.Lock()
	p := uc.pending[callID]
	// Identify which channel answered.
	if ev.Channel.ID == p.customerCh {
		p.answered |= 1
	}
	if ev.Channel.ID == p.agentCh {
		p.answered |= 2
	}
	uc.pending[callID] = p
	uc.mu.Unlock()

	// If both answered → IN_PROGRESS. Only transition from RINGING (the
	// state machine's allowed path). CONNECTING → IN_PROGRESS is also
	// allowed in the state map for the case where StasisStart was missed.
	if p.answered == 3 {
		c, err := uc.repo.Get(ctx, callID)
		if err != nil || c == nil {
			return
		}
		// Idempotent guard: if we're already IN_PROGRESS, no-op.
		if c.Status == domain.CallStatusInProgress {
			return
		}
		if c.Status != domain.CallStatusRinging && c.Status != domain.CallStatusConnecting {
			return
		}
		now := time.Now().UTC()
		if err := uc.transitionWithFields(ctx, c, domain.CallStatusInProgress,
			domain.StateTransitionFields{StartedAt: &now}); err != nil {
			return
		}
		if c.AgentID != nil {
			_ = uc.queue.SetAgentBusy(ctx, *c.AgentID, callID)
		}
		if uc.recordingEnabled && p.bridgeID != "" {
			_ = uc.gateway.StartRecording(ctx, callID, p.bridgeID)
			_ = uc.repo.InsertRecording(ctx, &domain.CallRecording{
				CallID: callID, State: domain.CallRecordingPending,
			})
		}
		uc.publishCallEvent(c, "call_started", nil)
	}
}

func (uc *UseCase) handleStasisEnd(ctx context.Context, callID uuid.UUID, ev domain.ARIEvent) {
	c, err := uc.repo.Get(ctx, callID)
	if err != nil || c == nil {
		return
	}
	// Idempotent: ignore if already terminal.
	if c.Status.IsTerminal() {
		return
	}

	// Free agent first.
	if c.AgentID != nil {
		_, _ = uc.queue.ReleaseAgent(ctx, *c.AgentID)
	}

	dur := 0
	if c.StartedAt != nil && !c.StartedAt.IsZero() {
		dur = int(time.Since(*c.StartedAt).Seconds())
	}
	endedAt := time.Now().UTC()
	_ = uc.transitionWithFields(ctx, c, domain.CallStatusEnded,
		domain.StateTransitionFields{EndedAt: &endedAt, DurationSeconds: &dur})

	_ = uc.gateway.StopRecording(ctx, callID)
	_ = uc.gateway.Hangup(ctx, callID)

	uc.publishCallEvent(c, "call_ended_v2", nil)
	uc.notifyRouter(callID)

	// Clear pending entry + dedup state.
	uc.queue.ClearAnnounced(ctx, callID)
	uc.mu.Lock()
	delete(uc.pending, callID)
	uc.mu.Unlock()
}

func (uc *UseCase) handleRecordingFinished(ctx context.Context, callID uuid.UUID, ev domain.ARIEvent) {
	name := ""
	if ev.Recording != nil {
		name = ev.Recording.Name
	}
	storage := "/var/spool/asterisk/recordings/" + name + ".wav"
	_ = uc.repo.UpdateRecording(ctx, callID, name, storage, ev.Timestamp, domain.CallRecordingStopped)
}

// replayIdempotent waits briefly for the winner of an idempotency claim to
// populate the cached response, then returns it. If the winner has not
// finished in time it returns an error so the client can retry.
func (uc *UseCase) replayIdempotent(ctx context.Context, key string) (*RequestCallOutput, error) {
	deadline := time.Now().Add(2 * time.Second)
	for {
		rec, err := uc.repo.GetByIdempotencyKey(ctx, key)
		if err != nil {
			return nil, err
		}
		if rec != nil && rec.CallID != uuid.Nil && len(rec.ResponseBody) > 0 {
			out := &RequestCallOutput{Replay: true}
			if v, ok := rec.ResponseBody["call_id"].(string); ok {
				out.CallID = v
			}
			if v, ok := rec.ResponseBody["status"].(string); ok {
				out.Status = v
			}
			if v, ok := rec.ResponseBody["queue_position"].(float64); ok {
				out.Position = int64(v)
			}
			if out.CallID == "" {
				out.CallID = rec.CallID.String()
			}
			return out, nil
		}
		if time.Now().After(deadline) {
			return nil, errors.New("idempotent request still in flight; retry")
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(20 * time.Millisecond):
		}
	}
}

// GatewayForTest returns the underlying AsteriskGateway. Tests may cast the
// returned value to a concrete type (e.g. *asterisk.MockGateway) to drive
// ARI behaviour deterministically. Production code must NOT use this.
func (uc *UseCase) GatewayForTest() domain.AsteriskGateway { return uc.gateway }

// SetBroadcasterForTest swaps the WS broadcaster. Used by dedup tests to
// count how many times `incoming_call` is fired. Production code must NOT
// use this — broadcaster wiring is done at construction time.
func (uc *UseCase) SetBroadcasterForTest(h WebsocketBroadcaster) { uc.hub = h }

// GetCall returns the authoritative state of a call.
func (uc *UseCase) GetCall(ctx context.Context, id uuid.UUID) (*domain.Call, error) {
	return uc.repo.Get(ctx, id)
}

// GetActiveCallsForAgent returns non-terminal calls assigned to an agent.
// Used after a WebSocket reconnect so the admin layout can re-show the
// ringing banner for a call that was announced before the page reload.
// Returns at most 5 calls (most recently updated first).
func (uc *UseCase) GetActiveCallsForAgent(ctx context.Context, agentID string) ([]*domain.Call, error) {
	if agentID == "" {
		return nil, nil
	}
	return uc.repo.ListActiveByAgent(ctx, agentID)
}

// ListCalls returns a paginated slice of all calls (Call v2). Used by the
// admin call history view. Returns the rows in most-recently-updated
// order; the page is keyed by the standard `limit, offset` pair.
func (uc *UseCase) ListCalls(ctx context.Context, limit, offset int) ([]*domain.Call, error) {
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	return uc.repo.ListAll(ctx, limit, offset)
}

// ----------------------------------------------------------------
// Agent-side helpers (called by HTTP handler)
// ----------------------------------------------------------------

// HeartbeatAgentPublic refreshes the agent's "last seen" timestamp.
func (uc *UseCase) HeartbeatAgentPublic(ctx context.Context, agentID string) error {
	return uc.queue.HeartbeatAgent(ctx, agentID)
}

// SetAgentStatusPublic applies an explicit AVAILABLE / AWAY / OFFLINE change.
func (uc *UseCase) SetAgentStatusPublic(ctx context.Context, agentID string, next domain.AgentState) error {
	switch next {
	case domain.AgentAvailable:
		return uc.queue.SetAgentAvailable(ctx, agentID)
	case domain.AgentAway:
		return uc.queue.SetAgentAway(ctx, agentID)
	case domain.AgentOffline:
		return uc.queue.SetAgentOffline(ctx, agentID)
	default:
		return errors.New("invalid status; use AVAILABLE, AWAY, OFFLINE")
	}
}

// ----------------------------------------------------------------
// Helpers
// ----------------------------------------------------------------

func (uc *UseCase) transition(ctx context.Context, c *domain.Call, next domain.CallStatus) error {
	return uc.transitionWithFields(ctx, c, next, domain.StateTransitionFields{})
}

func (uc *UseCase) transitionWithFields(ctx context.Context, c *domain.Call, next domain.CallStatus, f domain.StateTransitionFields) error {
	from := c.Status
	// Validate without mutation: CanTransition is the same check TransitionTo
	// does, but we keep `c.Status` unchanged until the DB update succeeds.
	if from.IsTerminal() || !domain.CanTransition(from, next) {
		return domain.ErrInvalidTransition
	}
	if err := uc.repo.UpdateState(ctx, c.ID, from, next, f); err != nil {
		return err
	}
	// Persisted → reflect in-memory.
	c.Status = next
	return nil
}

// transitionRaw does the DB transition without an in-memory Call pointer.
func (uc *UseCase) transitionRaw(ctx context.Context, id uuid.UUID, from, to domain.CallStatus, f *domain.StateTransitionFields) error {
	if f == nil {
		f = &domain.StateTransitionFields{}
	}
	return uc.repo.UpdateState(ctx, id, from, to, *f)
}

func (uc *UseCase) failCall(ctx context.Context, callID uuid.UUID, reason string) {
	c, _ := uc.repo.Get(ctx, callID)
	if c == nil {
		return
	}
	if c.Status.IsTerminal() {
		return
	}
	_ = uc.transitionWithFields(ctx, c, domain.CallStatusFailed,
		domain.StateTransitionFields{FailureReason: &reason})
	if c.AgentID != nil {
		_, _ = uc.queue.ReleaseAgent(ctx, *c.AgentID)
	}
	uc.publishCallEvent(c, "call_failed", map[string]any{"reason": reason})
}

// publishCallEvent emits a WS event to the customer session channel.
func (uc *UseCase) publishCallEvent(c *domain.Call, eventType string, extra map[string]any) {
	if uc.hub == nil {
		return
	}
	payload := map[string]any{
		"call_id":     c.ID.String(),
		"status":      string(c.Status),
		"customer_id": c.CustomerID,
	}
	for k, v := range extra {
		payload[k] = v
	}
	ev := &domain.WSEvent{
		Type:      domain.WSEventType(eventType),
		SessionID: c.CustomerID,
		Payload:   payload,
		Timestamp: time.Now().UTC(),
	}
	uc.hub.BroadcastToSession(c.CustomerID, ev)
	if c.AgentID != nil {
		uc.hub.BroadcastToSession("agent:"+*c.AgentID, ev)
	}
}

func (uc *UseCase) publishAgentEvent(callID uuid.UUID, agentID string, eventType string) {
	if uc.hub == nil {
		return
	}
	payload := map[string]any{
		"call_id": callID.String(),
	}
	// Best-effort enrichment: pull the call row so admin and agent
	// subscribers receive the customer/session context needed to render
	// the ringing banner. Without this, AdminSidebar's listener bails
	// out on `if (!sID) return;` and no banner ever appears.
	if uc.repo != nil {
		if c, err := uc.repo.Get(context.Background(), callID); err == nil && c != nil {
			payload["customer_id"] = c.CustomerID
			// session_id: frontend reads this for Call v2. Without it, the
			// banner silently fails to appear because event.session_id is
			// "agent:{username}" (not the customer session).
			payload["session_id"] = c.CustomerID
			if c.Priority > 0 {
				payload["priority"] = c.Priority
			}
		} else if err != nil {
			uc.logger.Debug().Err(err).Str("call_id", callID.String()).Msg("publishAgentEvent: enrich failed")
		}
	}

	ev := &domain.WSEvent{
		Type:      domain.WSEventType(eventType),
		SessionID: "agent:" + agentID,
		Payload:   payload,
		Timestamp: time.Now().UTC(),
	}
	// Broadcast to the agent's personal channel. The hub's whitelist
	// (see ws.Hub.BroadcastToSessionExcept) ALSO mirrors Call v2 events
	// to `admin_inbox` so every connected admin sees the banner even if
	// the agent's WS is offline.
	uc.hub.BroadcastToSession("agent:"+agentID, ev)
}

// notifyRouter pokes the routing goroutine. In this in-process implementation
// we use a buffered channel the routing worker is reading from.
func (uc *UseCase) notifyRouter(_ uuid.UUID) {
	// The routing worker is started in worker package; it polls every 200ms
	// AND reads from this channel for immediate wakeup.
	select {
	case routerSignal <- struct{}{}:
	default:
	}
}

// customerSIPEndpoint maps customer_id to its PJSIP endpoint.
// Deprecated: kept only for test compatibility — use asterisk.CustomerEndpoint.
func customerSIPEndpoint(customerID string) string {
	return "PJSIP/customer-" + customerID
}

// mirrorCallToCase upserts a chat-case row for the given call so the admin
// inbox (/api/admin/cases) reflects the guest as soon as a call is
// assigned. Without this the call row lives in `voice_calls` but the
// case-management UI never sees it — the legacy InitiateCall path did
// this synchronously inside the use case, so we mirror that here.
//
// The operation is best-effort: a failure is logged but does NOT roll
// back the call, because the call pipeline is the source of truth and
// the case row is just an inbox projection.
func (uc *UseCase) mirrorCallToCase(ctx context.Context, callID uuid.UUID, agentID string) {
	if uc.caseRepo == nil {
		return
	}
	c, err := uc.repo.Get(ctx, callID)
	if err != nil || c == nil {
		uc.logger.Debug().Err(err).Str("call_id", callID.String()).Msg("mirrorCallToCase: call not found")
		return
	}

	// Re-use customer_id as session_id for the case row. This matches
	// what InitiateCall does on the legacy voice path — the chat
	// session_id is the customer identifier.
	sessionID := c.CustomerID
	if sessionID == "" {
		return
	}

	// Don't trample an existing case row. If the guest has already
	// chatted, their case carries real context (last message, phone,
	// etc.). We only update last_message to reflect the call event and
	// bump status to HUMAN_CS_ACTIVE once an agent is engaged.
	existing, _ := uc.caseRepo.Get(ctx, sessionID)
	customerName := ""
	customerPhone := ""
	var guestID *uuid.UUID
	if existing != nil {
		customerName = existing.CustomerName
		customerPhone = existing.CustomerPhone
		guestID = existing.GuestID
	}

	// Promote to HUMAN_CS_ACTIVE if a specific agent is now on the call,
	// otherwise leave at NEEDS_HUMAN_CS (waiting in queue).
	status := domain.StatusNeedsHumanCS
	if agentID != "" {
		status = domain.StatusHumanCSActive
	}
	lastMsg := "📞 Đang yêu cầu cuộc gọi thoại..."
	if existing != nil && existing.LastMessage != "" {
		// Append a call-event marker so admins scrolling the list see
		// the new call even when there's older chat history.
		lastMsg = existing.LastMessage + "\n📞 Đang yêu cầu cuộc gọi thoại..."
	}

	if _, err := uc.caseRepo.Upsert(ctx, sessionID, guestID, customerName, customerPhone, status, agentID, lastMsg); err != nil {
		uc.logger.Warn().Err(err).Str("call_id", callID.String()).Msg("mirrorCallToCase upsert failed")
	} else {
		// Tell admin_inbox to refresh so the new case shows up without a
		// manual reload. This mirrors the case_update event that
		// InitiateCall / chat paths emit.
		uc.hub.BroadcastToSession("admin_inbox", &domain.WSEvent{
			Type:      domain.WSEventCaseUpdate,
			SessionID: "admin_inbox",
			Payload: map[string]any{
				"session_id":     sessionID,
				"customer_name":  customerName,
				"customer_phone": customerPhone,
				"status":         string(status),
				"assigned_cs":    agentID,
				"call_id":        callID.String(),
			},
			Timestamp: time.Now().UTC(),
		})
	}
}

// routerSignal is a process-global wake-up channel for the routing worker.
var routerSignal = make(chan struct{}, 64)

// RouterSignal exposes the channel to the worker package.
func RouterSignal() <-chan struct{} { return routerSignal }

// ----------------------------------------------------------------
// Reconciliation worker entry points
// ----------------------------------------------------------------

// Reconcile runs a single pass; the worker invokes it on a ticker.
func (uc *UseCase) Reconcile(ctx context.Context) error {
	// 1. Find agents in non-terminal busy states whose call is gone.
	active, err := uc.queue.ListActiveAgents(ctx)
	if err != nil {
		return err
	}
	for _, agentID := range active {
		a, _ := uc.queue.GetAgent(ctx, agentID)
		if a == nil || a.CurrentCall == nil {
			_, _ = uc.queue.ReleaseAgent(ctx, agentID)
			continue
		}
		c, _ := uc.repo.Get(ctx, *a.CurrentCall)
		if c == nil || c.Status.IsTerminal() {
			_, _ = uc.queue.ReleaseAgent(ctx, agentID)
			continue
		}
		// 2. Calls stuck without events for too long.
		if time.Since(c.LastEventAt) > uc.maxDuration {
			uc.failCall(ctx, c.ID, "max_duration_exceeded")
			_, _ = uc.queue.ReleaseAgent(ctx, agentID)
		}
	}

	// 3. Drop WAITING calls past queue TTL.
	stuck, err := uc.repo.ListActive(ctx, uc.queueTTL, 100)
	if err == nil {
		for _, c := range stuck {
			if c.Status == domain.CallStatusWaiting && time.Since(c.LastEventAt) > uc.queueTTL {
				_ = uc.queue.RemoveFromQueue(ctx, c.ID)
				f := domain.StateTransitionFields{FailureReason: ptrStr("queue_ttl_exceeded")}
				_ = uc.transitionWithFields(ctx, c, domain.CallStatusTimeout, f)
			}
		}
	}
	return nil
}

func ptrStr(s string) *string { return &s }

// ----------------------------------------------------------------
// Misc utilities
// ----------------------------------------------------------------

// NewID returns a random hex id (used for tests; production uses uuid).
func NewID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// jsonOrEmpty marshals v, falling back to {}.
func jsonOrEmpty(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}
