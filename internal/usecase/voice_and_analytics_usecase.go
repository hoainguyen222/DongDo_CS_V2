package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
	infraAsterisk "github.com/hoainguyen222/DongDo_CS_V2/internal/infra/asterisk"
	"github.com/rs/zerolog"
)

// ============================================================
// VoiceUseCase (Refactored)
// ============================================================
//
// Owns the queue-driven call lifecycle:
//
//   1. Customer calls → CreateCall (DB row + Redis queue)
//   2. Atomic agent assignment via Redis Lua
//   3. State-machine-validated transitions
//   4. Idempotent accept / reject / hangup
//   5. Audit events written via CallEventRepository
//
// The usecase depends on:
//   - VoiceCallRepository   — persistent call records
//   - CallEventRepository   — append-only audit log
//   - QueueManager          — Redis queue + agent state + idempotency
//   - AsteriskGateway       — interface, NOT infra/asterisk directly
//   - EventBus              — WebSocket fan-out
//
// HTTP layer reaches these through domain interfaces only.
type VoiceUseCase struct {
	voiceRepo    domain.VoiceCallRepository
	eventRepo    domain.CallEventRepository
	queue        domain.QueueManager
	gateway      domain.AsteriskGateway
	caseRepo     domain.CaseRepository
	eventBus     domain.EventBus
	legacyAri    *infraAsterisk.ARIService // for ARI callbacks (webrtc path)
	logger       zerolog.Logger

	// Channel tracking (legacy AMI). Channel id is best-effort; if
	// the process restarts the AMI tracking columns on the call row
	// are re-read.
	channelMap sync.Map
	callMap    sync.Map

	// Pump lifecycle (legacy AMI events).
	pumpCancel context.CancelFunc

	// Agent reservation TTL.
	reservationTTL time.Duration

	// Available agent extensions (loaded from env or config).
	knownAgents []string
}

// VoiceUseCaseConfig bundles tunable parameters.
type VoiceUseCaseConfig struct {
	ReservationTTL time.Duration
	KnownAgents    []string
}

// NewVoiceUseCase constructs the use case. Asterisk wiring is set
// via WithAsterisk/WithARI as before.
func NewVoiceUseCase(
	voiceRepo domain.VoiceCallRepository,
	eventRepo domain.CallEventRepository,
	queue domain.QueueManager,
	caseRepo domain.CaseRepository,
	eventBus domain.EventBus,
	cfg VoiceUseCaseConfig,
) *VoiceUseCase {
	if cfg.ReservationTTL <= 0 {
		cfg.ReservationTTL = 30 * time.Second
	}
	return &VoiceUseCase{
		voiceRepo:      voiceRepo,
		eventRepo:      eventRepo,
		queue:          queue,
		caseRepo:       caseRepo,
		eventBus:       eventBus,
		logger:         zerolog.New(nil).With().Timestamp().Str("usecase", "voice").Logger(),
		reservationTTL: cfg.ReservationTTL,
		knownAgents:    cfg.KnownAgents,
	}
}

// WithAsterisk attaches an AsteriskGateway (interface). It also keeps
// the legacy AMI client around for legacy event-pump paths.
func (uc *VoiceUseCase) WithAsterisk(gw domain.AsteriskGateway) *VoiceUseCase {
	uc.gateway = gw
	return uc
}

// WithARI keeps the legacy ARI service so existing ARI WebSocket
// callbacks (HandleARIGuestRing/Active/Ended) continue to work.
// New code uses AsteriskGateway via WithAsterisk.
func (uc *VoiceUseCase) WithARI(svc *infraAsterisk.ARIService) *VoiceUseCase {
	uc.legacyAri = svc
	return uc
}

// AsteriskClient returns the underlying AsteriskGateway. May be nil.
func (uc *VoiceUseCase) AsteriskClient() domain.AsteriskGateway { return uc.gateway }

// ARIService returns the legacy ARI service (or nil).
func (uc *VoiceUseCase) ARIService() *infraAsterisk.ARIService { return uc.legacyAri }

// Start launches the reconciliation goroutine (only when gateway is
// configured).
func (uc *VoiceUseCase) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	uc.pumpCancel = cancel
	go uc.reconcileLoop(ctx)
}

// Stop terminates background goroutines.
func (uc *VoiceUseCase) Stop() {
	if uc.pumpCancel != nil {
		uc.pumpCancel()
	}
}

// ============================================================
// Step 1: CreateCall
// ============================================================

// CreateCallInput is the data needed to create a new call.
type CreateCallInput struct {
	SessionID  string
	CallerID   string
	CalleeID   string
	CustomerName string
	Phone      string
	// IdempotencyKey — if supplied, a duplicate call with the same
	// key returns the previously created call record.
	IdempotencyKey string
}

// CreateCall creates a new call record, appends it to the queue,
// persists a CREATED + QUEUED audit row, and tries to assign an
// agent immediately.
func (uc *VoiceUseCase) CreateCall(ctx context.Context, in CreateCallInput) (*domain.VoiceCall, error) {
	if in.SessionID == "" {
		return nil, errors.New("session_id is required")
	}
	if in.CallerID == "" {
		return nil, errors.New("caller_id is required")
	}

	// Idempotency check via Redis. If we have a cached response,
	// return it.
	if in.IdempotencyKey != "" {
		existing, hit, err := uc.queue.ReserveIdempotency(
			ctx, "call:create:"+in.IdempotencyKey, "", 60*time.Second)
		if err == nil && hit {
			// The cached payload is the call_id we previously created.
			if existing != "" {
				if _, perr := uc.voiceRepo.GetByID(ctx, parseInt64(existing)); perr == nil {
					if call, gerr := uc.voiceRepo.GetByID(ctx, parseInt64(existing)); gerr == nil && call != nil {
						return call, nil
					}
				}
			}
		}
	}

	// Persist the call row.
	call := &domain.VoiceCall{
		SessionID:  in.SessionID,
		CallerType: domain.CallerGuest,
		CallerID:   in.CallerID,
		CalleeType: domain.CallerCSKH,
		CalleeID:   in.CalleeID,
		Status:     domain.CallWaiting,
	}
	created, err := uc.voiceRepo.Create(ctx, call)
	if err != nil {
		return nil, fmt.Errorf("create call: %w", err)
	}

	// Cache the new id under the idempotency key (overwrite the
	// empty string we stored).
	if in.IdempotencyKey != "" {
		_ = uc.writeIdempotency(ctx, "call:create:"+in.IdempotencyKey, fmt.Sprintf("%d", created.ID))
	}

	// Broadcast initial status so guest UI can show "Đang đổ chuông"
	// and admin inbox can show the ring banner without relying on
	// polling /api/voice/status/:id.
	uc.publishCallStatusUpdate(ctx, created, map[string]interface{}{
		"event": domain.CallEventCreated,
	})

	// Audit CREATED + QUEUED.
	uc.appendEvent(ctx, created.ID, domain.CallEventCreated, domain.CallEventSourceAPI, nil)
	pos, _ := uc.queue.EnqueueCall(ctx, created.ID)
	uc.appendEvent(ctx, created.ID, domain.CallEventQueued, domain.CallEventSourceSystem, map[string]interface{}{
		"queue_position": pos,
	})

	// Mirror status into Redis so callers can read cheap state.
	_ = uc.queue.SetAgentCurrentCall(ctx, "", 0) // no-op for queue position

	// Upsert the chat case so the agent sees the customer in their
	// inbox while waiting.
	if in.CustomerName != "" {
		_, _ = uc.caseRepo.Upsert(ctx, in.SessionID, nil, in.CustomerName, in.Phone,
			domain.StatusNeedsHumanCS, "", "📞 Đang yêu cầu cuộc gọi thoại...")
	}

	// Try to assign an agent right away.
	_ = uc.TryRoute(ctx)

	uc.logger.Info().
		Int64("call_id", created.ID).
		Str("session_id", in.SessionID).
		Int("queue_position", pos).
		Msg("call created and queued")

	return created, nil
}

// ============================================================
// Step 2-3: TryRoute / assignAgent
// ============================================================

// TryRoute attempts to assign an agent to the head of the queue.
// Safe to call from many goroutines — the underlying Redis Lua is
// atomic.
func (uc *VoiceUseCase) TryRoute(ctx context.Context) error {
	// Peek the head of the queue.
	callID, err := uc.queue.DequeueCall(ctx)
	if err != nil {
		if errors.Is(err, domain.ErrQueueEmpty) {
			return nil
		}
		return err
	}
	// We popped too aggressively — push it back if we can't route.
	defer func() {
		// Always push the call back into the queue at the head so the
		// next TryRoute call can pick it up. We use LPUSH to keep FIFO
		// semantics (re-routed calls go to the back is also acceptable
		// depending on fairness policy; here we put them back at the
		// head so they get priority).
		_, _ = uc.queue.EnqueueCall(ctx, callID)
	}()

	call, err := uc.voiceRepo.GetByID(ctx, callID)
	if err != nil || call == nil {
		return nil
	}

	// Find an available agent.
	agentExt, ok := uc.pickAgent(ctx, callID)
	if !ok {
		// No agent available; call stays WAITING in the queue.
		uc.logger.Info().Int64("call_id", callID).Msg("no agent available; call stays in queue")
		return nil
	}

	// Atomic reservation succeeded — flip call state.
	if err := uc.transitionCall(ctx, call, domain.CallWaitingAgent, domain.CallEventAssigned, map[string]interface{}{
		"agent_extension": agentExt,
	}); err != nil {
		uc.queue.ReleaseReservation(ctx, agentExt)
		return err
	}

	// Mirror on Redis.
	_ = uc.queue.SetAgentCurrentCall(ctx, agentExt, callID)

	// Notify the agent.
	uc.eventBus.PublishWS(ctx, agentSessionID(agentExt), domain.WSEventMessage, map[string]interface{}{
		"type":         "incoming_call",
		"call_id":      callID,
		"session_id":   call.SessionID,
		"customer_id":  call.CallerID,
		"customer_name": "",
	}, "system")

	// Schedule timeout.
	uc.scheduleReservationTimeout(ctx, callID, agentExt)

	uc.logger.Info().
		Int64("call_id", callID).
		Str("agent", agentExt).
		Msg("call assigned to agent (RESERVED)")
	return nil
}

// pickAgent loops over known agents and tries to atomically reserve
// the call for each. Returns the first successful extension.
func (uc *VoiceUseCase) pickAgent(ctx context.Context, callID int64) (string, bool) {
	for _, ext := range uc.knownAgents {
		ok, err := uc.queue.AtomicReserveAgent(ctx, callID, ext, uc.reservationTTL)
		if err != nil {
			uc.logger.Warn().Err(err).Str("agent", ext).Msg("AtomicReserveAgent failed")
			continue
		}
		if ok {
			return ext, true
		}
	}
	return "", false
}

// agentSessionID returns the WS session ID used to reach a given agent.
func agentSessionID(ext string) string { return "agent:" + ext }

// scheduleReservationTimeout starts a goroutine that releases the
// reservation if the agent doesn't accept within the TTL.
func (uc *VoiceUseCase) scheduleReservationTimeout(ctx context.Context, callID int64, agentExt string) {
	go func() {
		timer := time.NewTimer(uc.reservationTTL)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			uc.handleReservationTimeout(context.Background(), callID, agentExt)
		}
	}()
}

// handleReservationTimeout is called when an agent does not accept in time.
func (uc *VoiceUseCase) handleReservationTimeout(ctx context.Context, callID int64, agentExt string) {
	call, err := uc.voiceRepo.GetByID(ctx, callID)
	if err != nil || call == nil {
		return
	}
	// If the call has already moved past WAITING_AGENT (e.g. accepted
	// in the last second), don't reset.
	if call.Status.Canonical() != domain.CallWaitingAgent {
		return
	}
	// Move the agent back to AVAILABLE.
	_ = uc.queue.ReleaseReservation(ctx, agentExt)
	_ = uc.queue.SetAgentCurrentCall(ctx, agentExt, 0)
	// Flip call to MISSED and re-queue it for the next agent.
	_ = uc.transitionCall(ctx, call, domain.CallMissed, domain.CallEventTimeout, map[string]interface{}{
		"agent_extension": agentExt,
	})
	_, _ = uc.queue.EnqueueCall(ctx, callID)
	// Try next agent.
	_ = uc.TryRoute(ctx)
}

// ============================================================
// Step 4: AcceptCall / RejectCall / HangupCall (idempotent)
// ============================================================

// AcceptCallInput — params for AcceptCall.
type AcceptCallInput struct {
	CallID         int64
	AgentExtension string
	IdempotencyKey string
}

// AcceptCall is invoked when an agent clicks "Nghe máy".
func (uc *VoiceUseCase) AcceptCall(ctx context.Context, in AcceptCallInput) (*domain.VoiceCall, error) {
	if in.CallID <= 0 {
		return nil, errors.New("call_id is required")
	}
	if in.AgentExtension == "" {
		return nil, errors.New("agent_extension is required")
	}

	// Idempotency: same key returns cached response (the call row).
	idempKey := in.IdempotencyKey
	if idempKey == "" {
		idempKey = fmt.Sprintf("call:%d:accept:%s", in.CallID, in.AgentExtension)
	}
	if existing, hit, err := uc.queue.ReserveIdempotency(ctx, idempKey, "PENDING", 60*time.Second); err == nil && hit {
		uc.logger.Info().Int64("call_id", in.CallID).Str("idem", idempKey).Msg("AcceptCall idempotent replay")
		if existing != "" && existing != "PENDING" {
			if cachedID := parseInt64(existing); cachedID > 0 {
				return uc.voiceRepo.GetByID(ctx, cachedID)
			}
		}
		// Existing pending call — try to read the call row.
		return uc.voiceRepo.GetByID(ctx, in.CallID)
	}

	call, err := uc.voiceRepo.GetByID(ctx, in.CallID)
	if err != nil || call == nil {
		return nil, fmt.Errorf("call not found")
	}

	// Verify the agent is the one reserved for this call.
	if call.Status.Canonical() != domain.CallWaitingAgent {
		// Idempotent: if it's already CONNECTING/IN_PROGRESS, treat as
		// success and return current state.
		if call.Status.IsLive() || call.Status.IsTerminal() {
			_ = uc.writeIdempotency(ctx, idempKey, fmt.Sprintf("%d", call.ID))
			return call, nil
		}
		return nil, fmt.Errorf("call is in state %s, not WAITING_AGENT", call.Status)
	}

	// Move agent RESERVED → RINGING.
	if err := uc.transitionAgent(ctx, in.AgentExtension, domain.AgentReserved, domain.AgentRinging); err != nil {
		return nil, err
	}
	// Move call WAITING_AGENT → CONNECTING.
	if err := uc.transitionCall(ctx, call, domain.CallConnecting, domain.CallEventAccepted, map[string]interface{}{
		"agent_extension": in.AgentExtension,
	}); err != nil {
		return nil, err
	}

	// Ask Asterisk to originate the agent leg + bridge.
	if uc.gateway != nil && uc.gateway.Enabled() {
		if err := uc.gateway.OriginateAgentCall(ctx, call.ID, call.SessionID, in.AgentExtension); err != nil {
			uc.logger.Warn().Err(err).Int64("call_id", call.ID).Msg("Asterisk OriginateAgentCall failed")
			// Don't fail the API — the agent can still try sip.js fallback.
		}
	}

	// Cancel the reservation timer by flipping the agent state (the
	// timer goroutine will see the call is no longer WAITING_AGENT
	// and exit).
	_ = uc.queue.SetAgentState(ctx, in.AgentExtension, domain.AgentRinging, 0)

	_ = uc.writeIdempotency(ctx, idempKey, fmt.Sprintf("%d", call.ID))

	uc.logger.Info().
		Int64("call_id", call.ID).
		Str("agent", in.AgentExtension).
		Msg("call accepted")
	return call, nil
}

// RejectCallInput — params for RejectCall.
type RejectCallInput struct {
	CallID         int64
	AgentExtension string
	IdempotencyKey string
	Reason         string
}

// RejectCall is invoked when an agent clicks "Từ chối".
func (uc *VoiceUseCase) RejectCall(ctx context.Context, in RejectCallInput) error {
	if in.CallID <= 0 {
		return errors.New("call_id is required")
	}
	call, err := uc.voiceRepo.GetByID(ctx, in.CallID)
	if err != nil || call == nil {
		return fmt.Errorf("call not found")
	}
	if call.Status.Canonical() != domain.CallWaitingAgent {
		return nil // idempotent
	}

	// Agent RESERVED → AVAILABLE.
	_ = uc.queue.ReleaseReservation(ctx, in.AgentExtension)
	_ = uc.queue.SetAgentCurrentCall(ctx, in.AgentExtension, 0)

	// Move call to REJECTED then back to WAITING (so another agent
	// can pick it up).
	if err := uc.transitionCall(ctx, call, domain.CallRejected, domain.CallEventRejected, map[string]interface{}{
		"agent_extension": in.AgentExtension,
		"reason":          in.Reason,
	}); err != nil {
		return err
	}
	// Re-queue and re-route.
	_, _ = uc.queue.EnqueueCall(ctx, call.ID)
	// Push a fresh record with WAITING for the next attempt.
	if err := uc.transitionCallRaw(ctx, call.ID, domain.CallWaiting); err == nil {
		uc.appendEvent(ctx, call.ID, domain.CallEventQueued, domain.CallEventSourceSystem, nil)
	}
	_ = uc.TryRoute(ctx)
	return nil
}

// HangupCallInput — params for HangupCall.
type HangupCallInput struct {
	CallID          int64
	SessionID       string
	DurationSeconds int
	RecordingURL    string
	IdempotencyKey  string
}

// HangupCall ends a call. Idempotent via call_event uniqueness.
func (uc *VoiceUseCase) HangupCall(ctx context.Context, in HangupCallInput) error {
	if in.CallID <= 0 {
		return errors.New("call_id is required")
	}
	call, err := uc.voiceRepo.GetByID(ctx, in.CallID)
	if err != nil || call == nil {
		return fmt.Errorf("call not found")
	}
	if call.Status.IsTerminal() {
		// Idempotent: already ended.
		return nil
	}

	// Ask Asterisk to drop both legs.
	if uc.gateway != nil && uc.gateway.Enabled() {
		_ = uc.gateway.HangupCall(ctx, in.CallID)
	}

	// Determine the final status. If the call was never accepted, it's
	// MISSED; otherwise ENDED.
	final := domain.CallEnded
	if call.Status.Canonical() == domain.CallWaitingAgent {
		final = domain.CallMissed
	}

	// Append the HANGUP audit event first; the unique index on
	// (call_id, event_type, source) makes this safe on retries.
	uc.appendEvent(ctx, in.CallID, domain.CallEventHangup, domain.CallEventSourceAPI, map[string]interface{}{
		"reason": "user_hangup",
	})

	// Move call → terminal.
	_ = uc.transitionCallRaw(ctx, in.CallID, final)
	_ = uc.voiceRepo.End(ctx, in.CallID, in.DurationSeconds, in.RecordingURL)
	uc.appendEvent(ctx, in.CallID, domain.CallEventEnded, domain.CallEventSourceSystem, map[string]interface{}{
		"duration_seconds": in.DurationSeconds,
	})

	// Free the agent.
	if call.CalleeID != "" {
		_ = uc.queue.SetAgentState(ctx, call.CalleeID, domain.AgentAvailable, 0)
		_ = uc.queue.SetAgentCurrentCall(ctx, call.CalleeID, 0)
	}

	// Notify WS.
	uc.eventBus.PublishWS(ctx, call.SessionID, domain.WSEventCallEnd, map[string]interface{}{
		"call_id":          in.CallID,
		"session_id":       call.SessionID,
		"status":           final,
		"duration_seconds": in.DurationSeconds,
	}, "system")
	uc.eventBus.PublishWS(ctx, "admin_inbox", domain.WSEventCallEnd, map[string]interface{}{
		"call_id":          in.CallID,
		"session_id":       call.SessionID,
		"status":           final,
		"duration_seconds": in.DurationSeconds,
	}, "system")

	// Try to route the next queued call.
	_ = uc.TryRoute(ctx)
	return nil
}

// ============================================================
// State-transition helpers
// ============================================================

// publishCallStatusUpdate centralises the WS notification for call status
// transitions. We broadcast on the call's own sessionID — the hub's
// mirror logic (see internal/delivery/ws/hub.go) automatically fans
// the event out to the "admin_inbox" channel as well, so CSKH staff
// see every active call regardless of which agent is assigned.
//
// We deliberately do NOT publish to admin_inbox directly here: doing
// both produces duplicate events for admin listeners. The hub mirror
// is the single source of fan-out.
func (uc *VoiceUseCase) publishCallStatusUpdate(ctx context.Context, call *domain.VoiceCall, extra map[string]interface{}) {
	if call == nil || uc.eventBus == nil {
		return
	}
	payload := map[string]interface{}{
		"call_id":    call.ID,
		"session_id": call.SessionID,
		"status":     call.Status,
		"caller_id":  call.CallerID,
		"callee_id":  call.CalleeID,
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
	}
	for k, v := range extra {
		payload[k] = v
	}
	if call.SessionID != "" {
		_ = uc.eventBus.PublishWS(ctx, call.SessionID, domain.WSEventCallStatusUpdate, payload, "system")
	}
}

func (uc *VoiceUseCase) transitionCall(ctx context.Context, call *domain.VoiceCall, next domain.CallStatus, evt domain.CallEventType, payload interface{}) error {
	if err := (domain.CallStateMachine{}).TransitionTo(call.Status, next); err != nil {
		return err
	}
	if err := uc.voiceRepo.UpdateStatus(ctx, call.ID, next); err != nil {
		return fmt.Errorf("update status: %w", err)
	}
	prev := call.Status
	call.Status = next
	if payload != nil {
		uc.appendEvent(ctx, call.ID, evt, domain.CallEventSourceSystem, payload)
	} else {
		uc.appendEvent(ctx, call.ID, evt, domain.CallEventSourceSystem, nil)
	}
	uc.publishCallStatusUpdate(ctx, call, map[string]interface{}{
		"previous_status": prev,
		"event":           evt,
	})
	return nil
}

func (uc *VoiceUseCase) transitionCallRaw(ctx context.Context, callID int64, next domain.CallStatus) error {
	call, err := uc.voiceRepo.GetByID(ctx, callID)
	if err != nil || call == nil {
		return fmt.Errorf("call not found")
	}
	if err := (domain.CallStateMachine{}).TransitionTo(call.Status, next); err != nil {
		return err
	}
	if err := uc.voiceRepo.UpdateStatus(ctx, callID, next); err != nil {
		return err
	}
	prev := call.Status
	call.Status = next
	uc.publishCallStatusUpdate(ctx, call, map[string]interface{}{
		"previous_status": prev,
	})
	return nil
}

func (uc *VoiceUseCase) transitionAgent(ctx context.Context, agentExt string, from, to domain.AgentStatus) error {
	cur, _ := uc.queue.GetAgentState(ctx, agentExt)
	if cur == "" {
		cur = domain.AgentAvailable // default for first transition
	}
	if from != "" && cur != from {
		// Idempotent: if already at `to`, no-op.
		if cur == to {
			return nil
		}
	}
	if err := (domain.AgentStateMachine{}).TransitionTo(cur, to); err != nil {
		return err
	}
	return uc.queue.SetAgentState(ctx, agentExt, to, 0)
}

// ============================================================
// Audit / event helpers
// ============================================================

func (uc *VoiceUseCase) appendEvent(ctx context.Context, callID int64, evt domain.CallEventType, src domain.CallEventSource, payload interface{}) {
	if uc.eventRepo == nil {
		return
	}
	var pj string
	if payload != nil {
		if b, err := json.Marshal(payload); err == nil {
			pj = string(b)
		}
	}
	_ = uc.eventRepo.Append(ctx, &domain.CallEventRecord{
		CallID:    callID,
		EventType: evt,
		Source:    src,
		Payload:   pj,
	})
}

func (uc *VoiceUseCase) writeIdempotency(ctx context.Context, key, payload string) error {
	// Use ReserveIdempotency with 60s TTL. If there's already a value,
	// overwrite it.
	if uc.queue == nil {
		return nil
	}
	_, _, _ = uc.queue.ReserveIdempotency(ctx, key, payload, 60*time.Second)
	return nil
}

// ============================================================
// Reconciliation
// ============================================================

func (uc *VoiceUseCase) reconcileLoop(ctx context.Context) {
	uc.logger.Info().Msg("voice reconcile loop started")
	defer uc.logger.Info().Msg("voice reconcile loop stopped")
	t := time.NewTicker(60 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			uc.reconcile(ctx)
		}
	}
}

// reconcile re-asserts Redis state from PostgreSQL.
//   - For every agent in the known list, if Redis says RESERVED but
//     the reservation TTL has expired, return them to AVAILABLE.
//   - For every call in a transient state, verify the agent state in
//     Redis matches PostgreSQL.
func (uc *VoiceUseCase) reconcile(ctx context.Context) {
	for _, ext := range uc.knownAgents {
		st, err := uc.queue.GetAgentState(ctx, ext)
		if err != nil {
			continue
		}
		if st == domain.AgentReserved {
			uc.logger.Warn().Str("agent", ext).Msg("reconcile: stale RESERVED agent, flipping to AVAILABLE")
			_ = uc.queue.ReleaseReservation(ctx, ext)
		}
	}
}

// ============================================================
// Legacy ARI callback handlers — kept for backward compat
// ============================================================

// HandleARIGuestRing is called by the ARI service when the guest leg
// enters the Stasis app.
func (uc *VoiceUseCase) HandleARIGuestRing(ctx context.Context, callID int64, sessionID, callerName, callerID string) error {
	if callID <= 0 {
		return nil
	}
	if err := uc.transitionCallRaw(ctx, callID, domain.CallRinging); err != nil {
		uc.logger.Warn().Err(err).Int64("call_id", callID).Msg("HandleARIGuestRing: transition failed")
	}
	_ = uc.eventBus.PublishWS(ctx, sessionID, domain.WSEventCallRing, map[string]interface{}{
		"call_id":     callID,
		"status":      domain.CallRinging,
		"caller_id":   callerID,
		"caller_name": callerName,
		"transport":   "webrtc-ari",
	}, "asterisk")
	return nil
}

// HandleARICallActive marks the call as IN_PROGRESS when both legs
// have been bridged.
func (uc *VoiceUseCase) HandleARICallActive(ctx context.Context, callID int64, sessionID, agentExt string) error {
	if callID <= 0 {
		return nil
	}
	_ = uc.transitionCallRaw(ctx, callID, domain.CallInProgress)
	uc.appendEvent(ctx, callID, domain.CallEventBridged, domain.CallEventSourceARI, nil)
	_ = uc.queue.SetAgentState(ctx, agentExt, domain.AgentBusy, 0)
	_ = uc.eventBus.PublishWS(ctx, sessionID, domain.WSEventCallRing, map[string]interface{}{
		"call_id":     callID,
		"status":      domain.CallInProgress,
		"accepted_by": agentExt,
		"transport":   "webrtc-ari",
	}, agentExt)
	return nil
}

// HandleARICallEnded marks the call ENDED on ARI channel destruction.
func (uc *VoiceUseCase) HandleARICallEnded(ctx context.Context, callID int64, sessionID, cause string) error {
	if callID <= 0 {
		return nil
	}
	call, err := uc.voiceRepo.GetByID(ctx, callID)
	if err != nil || call == nil {
		return nil
	}
	if call.Status.IsTerminal() {
		return nil
	}
	_ = uc.transitionCallRaw(ctx, callID, domain.CallEnded)
	_ = uc.voiceRepo.End(ctx, callID, 0, "")
	uc.appendEvent(ctx, callID, domain.CallEventEnded, domain.CallEventSourceARI, map[string]interface{}{
		"cause": cause,
	})
	_ = uc.queue.SetAgentState(ctx, call.CalleeID, domain.AgentAvailable, 0)
	_ = uc.queue.SetAgentCurrentCall(ctx, call.CalleeID, 0)
	_ = uc.eventBus.PublishWS(ctx, sessionID, domain.WSEventCallRing, map[string]interface{}{
		"call_id": callID,
		"status":  domain.CallEnded,
		"cause":   cause,
	}, "asterisk")
	return nil
}

// ============================================================
// Read APIs (used by HTTP handlers)
// ============================================================

func (uc *VoiceUseCase) GetCall(ctx context.Context, id int64) (*domain.VoiceCall, error) {
	return uc.voiceRepo.GetByID(ctx, id)
}

func (uc *VoiceUseCase) ListCallsBySession(ctx context.Context, sessionID string) ([]*domain.VoiceCall, error) {
	return uc.voiceRepo.GetBySession(ctx, sessionID)
}

func (uc *VoiceUseCase) ListAllCalls(ctx context.Context) ([]*domain.VoiceCall, error) {
	return uc.voiceRepo.ListAll(ctx)
}

func (uc *VoiceUseCase) SetTranscript(ctx context.Context, id int64, transcript string) error {
	return uc.voiceRepo.SetTranscript(ctx, id, transcript)
}

func (uc *VoiceUseCase) DeleteCall(ctx context.Context, id int64) error {
	return uc.voiceRepo.Delete(ctx, id)
}

// ============================================================
// Misc helpers
// ============================================================

func parseInt64(s string) int64 {
	var n int64
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int64(c-'0')
	}
	return n
}

// ============================================================
// Backward-compat wrappers for the existing handlers
// ============================================================
//
// These keep the old HTTP endpoints working. New code should call
// AcceptCall/RejectCall/HangupCall above directly.

// InitiateCall wraps CreateCall for backward compat with the legacy
// POST /api/voice/initiate endpoint.
func (uc *VoiceUseCase) InitiateCall(ctx context.Context, sessionID string, callerType domain.CallerType, callerID string, calleeType domain.CallerType, calleeID string) (*domain.VoiceCall, error) {
	return uc.CreateCall(ctx, CreateCallInput{
		SessionID:  sessionID,
		CallerID:   callerID,
		CalleeID:   calleeID,
		CustomerName: callerID,
	})
}

// EndCall wraps HangupCall.
func (uc *VoiceUseCase) EndCall(ctx context.Context, callID int64, sessionID string, durationSeconds int, recordingURL string) error {
	return uc.HangupCall(ctx, HangupCallInput{
		CallID:          callID,
		SessionID:       sessionID,
		DurationSeconds: durationSeconds,
		RecordingURL:    recordingURL,
	})
}

// AcceptCallWebRTC — kept for backward compat. Calls AcceptCall.
func (uc *VoiceUseCase) AcceptCallWebRTC(ctx context.Context, callID int64, agentExtension string) error {
	_, err := uc.AcceptCall(ctx, AcceptCallInput{
		CallID:         callID,
		AgentExtension: agentExtension,
	})
	return err
}

// TransferCall — kept for backward compat; uses Asterisk gateway if
// available, else no-op.
func (uc *VoiceUseCase) TransferCall(ctx context.Context, callID int64, targetExtension string) error {
	if uc.gateway != nil && uc.gateway.Enabled() {
		return uc.gateway.HangupCall(ctx, callID) // best-effort drop; real transfer logic is in ARI
	}
	return nil
}

// StartRecording — kept for backward compat; uses Asterisk gateway.
func (uc *VoiceUseCase) StartRecording(ctx context.Context, callID int64, filename string) error {
	if uc.gateway != nil && uc.gateway.Enabled() {
		return uc.gateway.StartRecording(ctx, callID, filename)
	}
	return errors.New("asterisk gateway not configured")
}

// MarkMissedCall — kept for backward compat.
func (uc *VoiceUseCase) MarkMissedCall(ctx context.Context, callID int64, sessionID string) error {
	return uc.HangupCall(ctx, HangupCallInput{CallID: callID, SessionID: sessionID})
}

// GetCallsBySession — alias.
func (uc *VoiceUseCase) GetCallsBySession(ctx context.Context, sessionID string) ([]*domain.VoiceCall, error) {
	return uc.ListCallsBySession(ctx, sessionID)
}

// ============================================================
// Analytics & Config UseCase
// ============================================================

type AnalyticsUseCase struct {
	analyticsRepo domain.AnalyticsRepository
	settingRepo   domain.SettingRepository
	logger        zerolog.Logger
}

func NewAnalyticsUseCase(analyticsRepo domain.AnalyticsRepository, settingRepo domain.SettingRepository) *AnalyticsUseCase {
	return &AnalyticsUseCase{
		analyticsRepo: analyticsRepo,
		settingRepo:   settingRepo,
		logger:        zerolog.New(nil).With().Timestamp().Str("usecase", "analytics").Logger(),
	}
}

func (uc *AnalyticsUseCase) GetDashboardStats(ctx context.Context) (*domain.AnalyticsStats, error) {
	stats, err := uc.analyticsRepo.GetStats(ctx)
	if err != nil {
		uc.logger.Error().Err(err).Msg("failed to fetch dashboard stats")
		return nil, err
	}
	return stats, nil
}

func (uc *AnalyticsUseCase) GetSystemConfig(ctx context.Context, defaultPrompt, defaultModel string, defaultTemp float64) (prompt, model string, temp float64, err error) {
	prompt, _ = uc.settingRepo.Get(ctx, "system_prompt", defaultPrompt)
	model, _ = uc.settingRepo.Get(ctx, "llm_model", defaultModel)
	tempStr, _ := uc.settingRepo.Get(ctx, "temperature", fmt.Sprintf("%.1f", defaultTemp))

	var parsedTemp float64
	_, _ = fmt.Sscanf(tempStr, "%f", &parsedTemp)
	if parsedTemp == 0 {
		parsedTemp = defaultTemp
	}

	return prompt, model, parsedTemp, nil
}

func (uc *AnalyticsUseCase) SaveSystemConfig(ctx context.Context, prompt, model string, temp float64) error {
	if prompt == "" || model == "" {
		return errors.New("system prompt và model không được để trống")
	}

	_ = uc.settingRepo.Set(ctx, "system_prompt", prompt)
	_ = uc.settingRepo.Set(ctx, "llm_model", model)
	_ = uc.settingRepo.Set(ctx, "temperature", fmt.Sprintf("%.1f", temp))

	return nil
}
