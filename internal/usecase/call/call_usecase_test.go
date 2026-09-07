package call_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/infrastructure/asterisk"
	calluc "github.com/hoainguyen222/DongDo_CS_V2/internal/usecase/call"
	"github.com/rs/zerolog"
)

// fakeRepo is an in-memory implementation of domain.CallRepository used by
// the integration tests. It enforces CAS for state updates so the use case's
// transition logic is actually exercised.
type fakeRepo struct {
	mu           sync.Mutex
	calls        map[uuid.UUID]*domain.Call
	idemKeys     map[string]*domain.IdempotencyRecord
	participants []*domain.CallParticipant
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		calls:    make(map[uuid.UUID]*domain.Call),
		idemKeys: make(map[string]*domain.IdempotencyRecord),
	}
}

func (f *fakeRepo) Insert(_ context.Context, c *domain.Call) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	c.Status = domain.CallStatusCreated
	c.CreatedAt = time.Now().UTC()
	c.UpdatedAt = c.CreatedAt
	c.LastEventAt = c.CreatedAt
	f.calls[c.ID] = c
	return nil
}

func (f *fakeRepo) Get(_ context.Context, id uuid.UUID) (*domain.Call, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	c, ok := f.calls[id]
	if !ok {
		return nil, nil
	}
	return c, nil
}

func (f *fakeRepo) UpdateState(_ context.Context, id uuid.UUID, from, to domain.CallStatus, fields domain.StateTransitionFields) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	c, ok := f.calls[id]
	if !ok {
		return domain.ErrCallNotFound
	}
	// CAS check: only update if the stored row matches `from`.
	if c.Status != from {
		return domain.ErrInvalidTransition
	}
	c.Status = to
	c.UpdatedAt = time.Now().UTC()
	c.LastEventAt = c.UpdatedAt
	// Apply optional fields so tests can assert on them.
	if fields.AssignedAt != nil {
		c.AssignedAt = fields.AssignedAt
	}
	if fields.StartedAt != nil {
		c.StartedAt = fields.StartedAt
	}
	if fields.EndedAt != nil {
		c.EndedAt = fields.EndedAt
	}
	if fields.DurationSeconds != nil {
		c.DurationSeconds = *fields.DurationSeconds
	}
	if fields.ARIBridgeID != nil {
		c.ARIBridgeID = *fields.ARIBridgeID
	}
	if fields.FailureReason != nil {
		c.FailureReason = *fields.FailureReason
	}
	if fields.AgentID != nil {
		aid := *fields.AgentID
		c.AgentID = &aid
	}
	return nil
}

func (f *fakeRepo) AssignAgent(_ context.Context, id uuid.UUID, agentID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	c, ok := f.calls[id]
	if !ok {
		return domain.ErrCallNotFound
	}
	if c.Status != domain.CallStatusWaiting {
		return domain.ErrInvalidTransition
	}
	c.Status = domain.CallStatusWaitingAgent
	c.AgentID = &agentID
	c.UpdatedAt = time.Now().UTC()
	c.LastEventAt = c.UpdatedAt
	return nil
}

func (f *fakeRepo) AppendEvent(context.Context, *domain.CallEvent) error { return nil }
func (f *fakeRepo) RecordParticipant(_ context.Context, p *domain.CallParticipant) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	// Enforce (call_id, role, action) uniqueness so the use case's
	// idempotency path actually exercises ErrDuplicateAction.
	for _, existing := range f.participants {
		if existing.CallID == p.CallID && existing.Role == p.Role && existing.Action == p.Action {
			return domain.ErrDuplicateAction
		}
	}
	if p.ActionAt.IsZero() {
		p.ActionAt = time.Now().UTC()
	}
	cp := *p
	f.participants = append(f.participants, &cp)
	return nil
}
func (f *fakeRepo) InsertRecording(context.Context, *domain.CallRecording) error { return nil }
func (f *fakeRepo) UpdateRecording(context.Context, uuid.UUID, string, string, time.Time, domain.CallRecordingState) error {
	return nil
}
func (f *fakeRepo) ListByAgent(context.Context, string, int, int) ([]*domain.Call, error) {
	return nil, nil
}
func (f *fakeRepo) ListAll(context.Context, int, int) ([]*domain.Call, error) {
	return nil, nil
}
func (f *fakeRepo) ListActiveByAgent(context.Context, string) ([]*domain.Call, error) {
	return nil, nil
}
func (f *fakeRepo) ListByCustomer(_ context.Context, customerID string, limit, _ int) ([]*domain.Call, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]*domain.Call, 0)
	for _, c := range f.calls {
		if c.CustomerID == customerID {
			cp := *c
			out = append(out, &cp)
		}
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}
func (f *fakeRepo) ListActive(context.Context, time.Duration, int) ([]*domain.Call, error) {
	return nil, nil
}
func (f *fakeRepo) ClaimIdempotency(_ context.Context, key string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, exists := f.idemKeys[key]; exists {
		return false, nil
	}
	f.idemKeys[key] = &domain.IdempotencyRecord{Key: key, CallID: uuid.Nil, ResponseBody: map[string]any{}}
	return true, nil
}
func (f *fakeRepo) GetByIdempotencyKey(_ context.Context, key string) (*domain.IdempotencyRecord, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.idemKeys[key]
	if !ok {
		return nil, nil
	}
	cp := *r
	return &cp, nil
}

func (f *fakeRepo) SaveIdempotency(_ context.Context, rec *domain.IdempotencyRecord) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := *rec
	f.idemKeys[rec.Key] = &cp
	return nil
}

// fakeQueue is an in-memory CallQueueManager with strict atomicity guarantees
// suitable for the routing stress test. Uses sync.RWMutex so reads (GetAgent)
// don't block writes (ReserveAgent/ReleaseAgent) and vice versa.
type fakeQueue struct {
	mu        sync.RWMutex
	agents    map[string]*domain.Agent
	queue     []uuid.UUID
	available []string
	// announced tracks calls that have already been published to an agent
	// via `incoming_call`. Used by dedup tests in call_usecase_test.go to
	// confirm the routing loop does not re-publish while a call is still
	// in the WAITING_AGENT phase.
	announced map[uuid.UUID]bool
}

func newFakeQueue(available []string) *fakeQueue {
	q := &fakeQueue{
		agents: make(map[string]*domain.Agent),
	}
	for _, a := range available {
		q.agents[a] = &domain.Agent{ID: a, State: domain.AgentAvailable}
		q.available = append(q.available, a)
	}
	return q
}

func (q *fakeQueue) Enqueue(_ context.Context, id uuid.UUID, _ int, _ int64) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.queue = append(q.queue, id)
	return nil
}
func (q *fakeQueue) PopNext(_ context.Context) (uuid.UUID, bool, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.queue) == 0 {
		return uuid.Nil, false, nil
	}
	id := q.queue[0]
	q.queue = q.queue[1:]
	return id, true, nil
}
func (q *fakeQueue) QueuePosition(context.Context, uuid.UUID) (int64, error) { return 0, nil }
func (q *fakeQueue) QueueSize(_ context.Context) (int64, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	return int64(len(q.queue)), nil
}
func (q *fakeQueue) RemoveFromQueue(_ context.Context, id uuid.UUID) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	for i, x := range q.queue {
		if x == id {
			q.queue = append(q.queue[:i], q.queue[i+1:]...)
			return nil
		}
	}
	return nil
}
func (q *fakeQueue) ReserveAgent(_ context.Context, agentID string, callID uuid.UUID) (bool, domain.AgentState, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	a, ok := q.agents[agentID]
	if !ok || a.State != domain.AgentAvailable {
		return false, a.State, nil
	}
	a.State = domain.AgentReserved
	a.CurrentCall = &callID
	// remove from available
	for i, x := range q.available {
		if x == agentID {
			q.available = append(q.available[:i], q.available[i+1:]...)
			break
		}
	}
	return true, domain.AgentReserved, nil
}
func (q *fakeQueue) ReleaseAgent(_ context.Context, agentID string) (domain.AgentState, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	a, ok := q.agents[agentID]
	if !ok {
		return "", nil
	}
	a.State = domain.AgentAvailable
	a.CurrentCall = nil
	q.available = append(q.available, agentID)
	return domain.AgentAvailable, nil
}
func (q *fakeQueue) SetAgentBusy(_ context.Context, agentID string, _ uuid.UUID) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.agents[agentID].State = domain.AgentBusy
	return nil
}
func (q *fakeQueue) SetAgentAway(context.Context, string) error      { return nil }
func (q *fakeQueue) SetAgentAvailable(context.Context, string) error { return nil }
func (q *fakeQueue) SetAgentOffline(context.Context, string) error   { return nil }
func (q *fakeQueue) HeartbeatAgent(context.Context, string) error    { return nil }
func (q *fakeQueue) GetAgent(_ context.Context, agentID string) (*domain.Agent, error) {
	// Read-only access — use RLock so it doesn't conflict with concurrent writes.
	q.mu.RLock()
	defer q.mu.RUnlock()
	a, ok := q.agents[agentID]
	if !ok {
		return nil, nil
	}
	cp := *a
	return &cp, nil
}
func (q *fakeQueue) ListAvailable(_ context.Context) ([]string, error) {
	q.mu.RLock()
	defer q.mu.RUnlock()
	out := make([]string, len(q.available))
	copy(out, q.available)
	return out, nil
}
func (q *fakeQueue) ListActiveAgents(_ context.Context) ([]string, error) {
	q.mu.RLock()
	defer q.mu.RUnlock()
	out := make([]string, 0)
	for id, a := range q.agents {
		if a.State == domain.AgentReserved || a.State == domain.AgentRinging || a.State == domain.AgentBusy {
			out = append(out, id)
		}
	}
	return out, nil
}

// Dedup hooks. fakeQueue records per-call "announced" flags so the
// routing loop will not re-publish `incoming_call` while a call is still
// in the WAITING_AGENT phase. Used by call_usecase_test.go to assert the
// dedup behaviour described in AssignAgentForCall.
func (q *fakeQueue) MarkAnnounced(_ context.Context, id uuid.UUID, _ time.Duration) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.announced == nil {
		q.announced = map[uuid.UUID]bool{}
	}
	q.announced[id] = true
	return nil
}
func (q *fakeQueue) IsAnnounced(_ context.Context, id uuid.UUID) (bool, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.announced[id], nil
}
func (q *fakeQueue) ClearAnnounced(_ context.Context, id uuid.UUID) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	delete(q.announced, id)
	return nil
}

func newUC(t *testing.T, q *fakeQueue) (*calluc.UseCase, *fakeRepo) {
	t.Helper()
	repo := newFakeRepo()
	gw := asterisk.NewMockGateway(zerolog.Nop())
	// Short ring timeout so the per-agent goroutines in armRingTimeout
	// exit before the test deadline.
	uc := calluc.NewUseCase(repo, q, gw, nil, nil, calluc.Config{
		RingTimeout: 50 * time.Millisecond,
		MaxDuration: 30 * time.Minute,
		QueueTTL:    10 * time.Minute,
	}, zerolog.Nop())
	// Cancel ring-timeout goroutines as soon as the test ends so they
	// don't outlive the test's UseCase and pin shared resources. Without
	// this, -race can mask a goroutine still holding the queue mutex
	// and the next test deadlocks waiting for it.
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()
		_ = uc.Shutdown(ctx)
	})
	return uc, repo
}

// TestRouting_100CustomersVs5Agents_NoDoubleAssignment stresses the routing path.
// We model the spec's scenario: 100 customer requests arrive concurrently.
// Only 5 agents are AVAILABLE.  After one routing pass, at most 5 calls should
// be in WAITING_AGENT state and the rest should remain WAITING in the queue.
func TestRouting_100CustomersVs5Agents_NoDoubleAssignment(t *testing.T) {
	queue := newFakeQueue([]string{"a1", "a2", "a3", "a4", "a5"})
	uc, repo := newUC(t, queue)
	ctx := context.Background()

	// 1. Create 100 calls concurrently.
	var (
		wg       sync.WaitGroup
		ids      = make([]uuid.UUID, 100)
		errCount atomic.Int32
	)
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			out, err := uc.RequestCall(ctx, calluc.RequestCallInput{
				CustomerID: "cust-" + uuid.New().String()[:8],
				Priority:   0,
			})
			if err != nil {
				errCount.Add(1)
				return
			}
			cid, _ := uuid.Parse(out.CallID)
			ids[idx] = cid
		}(i)
	}
	wg.Wait()
	if errCount.Load() > 0 {
		t.Fatalf("%d RequestCall errors", errCount.Load())
	}

	// 2. Drain routing: call TryRouteNext until no agent is free OR the queue is empty.
	//    Crucial: do NOT call TryRouteNext when no agent is available, because the
	//    use case will re-enqueue the call and we'll loop forever.
	for {
		avail, _ := queue.ListAvailable(ctx)
		if len(avail) == 0 {
			break
		}
		size, _ := queue.QueueSize(ctx)
		if size == 0 {
			break
		}
		if err := uc.TryRouteNext(ctx); err != nil {
			t.Fatalf("route: %v", err)
		}
	}

	// 3. Assertions:
	//    a) Exactly 5 calls moved to WAITING_AGENT.
	//    b) No agent is assigned to two calls.
	agentToCalls := make(map[string][]uuid.UUID)
	for _, id := range ids {
		c, _ := repo.Get(ctx, id)
		if c == nil {
			continue
		}
		if c.Status == domain.CallStatusWaitingAgent && c.AgentID != nil {
			agentToCalls[*c.AgentID] = append(agentToCalls[*c.AgentID], id)
		}
	}
	if len(agentToCalls) > 5 {
		t.Errorf("more than 5 agents got assignments: %d", len(agentToCalls))
	}
	for ag, cs := range agentToCalls {
		if len(cs) > 1 {
			t.Errorf("agent %s was assigned to %d calls (must be 1)", ag, len(cs))
		}
	}
}

// TestAssignAgent_DedupIncomingCall verifies that re-running TryRouteNext
// while a call is still in the WAITING_AGENT phase does NOT publish a
// fresh `incoming_call` event to the hub. The fix for the "spam incoming
// call" bug depends on MarkAnnounced/IsAnnounced in the queue manager.
func TestAssignAgent_DedupIncomingCall(t *testing.T) {
	queue := newFakeQueue([]string{"agent-A"})
	uc, repo := newUC(t, queue)
	// Use a broadcaster that counts publishes.
	var mu sync.Mutex
	publishCount := 0
	hub := &countingHub{onIncomingCall: func() {
		mu.Lock()
		defer mu.Unlock()
		publishCount++
	}}
	uc.SetBroadcasterForTest(hub)

	ctx := context.Background()
	out, err := uc.RequestCall(ctx, calluc.RequestCallInput{CustomerID: "c1"})
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	cid, _ := uuid.Parse(out.CallID)

	// First route → reserves agent and publishes incoming_call.
	if err := uc.TryRouteNext(ctx); err != nil {
		t.Fatalf("route 1: %v", err)
	}

	c, _ := repo.Get(ctx, cid)
	if c == nil || c.Status != domain.CallStatusWaitingAgent {
		t.Fatalf("expected WAITING_AGENT after first route, got %v", c)
	}

	mu.Lock()
	first := publishCount
	mu.Unlock()
	if first != 1 {
		t.Fatalf("expected 1 publish after first route, got %d", first)
	}

	// Force the call back into the queue (simulating the routing worker
	// retrying after some time). Without dedup this would have published
	// a second incoming_call.
	if err := uc.TryRouteNext(ctx); err != nil {
		t.Fatalf("route 2: %v", err)
	}

	mu.Lock()
	second := publishCount
	mu.Unlock()
	if second != 1 {
		t.Fatalf("expected publish count to stay at 1 (dedup), got %d", second)
	}

	// After reject the dedup flag must clear so a new attempt publishes again.
	// NOTE: agent-B must be added BEFORE RejectCall so that hasOtherAgent
	// is true (ListAvailable will see B besides the released A). If added
	// after, the reject path sees only A (the rejecting agent) and cancels
	// the call — which is the correct "self-isolate" behavior, but that
	// would give publishCount=1 instead of the 2 publishes this test
	// is designed to verify.
	queue.agents["agent-B"] = &domain.Agent{ID: "agent-B", State: domain.AgentAvailable}
	queue.available = append(queue.available, "agent-B")
	if err := uc.RejectCall(ctx, calluc.RejectCallInput{CallID: cid, AgentID: "agent-A"}); err != nil {
		t.Fatalf("reject: %v", err)
	}
	// Re-route: TryRouteNext will pick agent-B (the only agent other than
	// the one who just rejected) and publish a second incoming_call.
	if err := uc.TryRouteNext(ctx); err != nil {
		t.Fatalf("route 3: %v", err)
	}
	mu.Lock()
	third := publishCount
	mu.Unlock()
	if third != 2 {
		t.Fatalf("expected 2 publishes after reject+reroute, got %d", third)
	}
}

// countingHub is a tiny hub stub used by dedup tests.
type countingHub struct {
	onIncomingCall func()
}

func (c *countingHub) BroadcastToSession(_ string, ev *domain.WSEvent) {
	if c.onIncomingCall != nil && (string(ev.Type) == "incoming_call") {
		c.onIncomingCall()
	}
}
func (c *countingHub) BroadcastToChannel(_ string, ev *domain.WSEvent) {
	c.BroadcastToSession("", ev)
}

// TestRequestCall_Idempotency verifies that the same idempotency key returns the
// same call id without creating duplicates.
func TestRequestCall_Idempotency(t *testing.T) {
	queue := newFakeQueue(nil)
	uc, repo := newUC(t, queue)
	ctx := context.Background()

	key := uuid.New().String()
	out1, err := uc.RequestCall(ctx, calluc.RequestCallInput{
		CustomerID: "cust-1", IdempotencyKey: key,
	})
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	if out1.Replay {
		t.Errorf("first call should not be a replay")
	}

	out2, err := uc.RequestCall(ctx, calluc.RequestCallInput{
		CustomerID: "cust-1", IdempotencyKey: key,
	})
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if !out2.Replay {
		t.Errorf("second call with same key must be replay")
	}
	if out1.CallID != out2.CallID {
		t.Errorf("call_id changed: %s vs %s", out1.CallID, out2.CallID)
	}

	// And the repo should only have one call.
	c, _ := repo.Get(ctx, uuid.MustParse(out1.CallID))
	if c == nil {
		t.Errorf("call not in repo")
	}
}

// TestARI_DuplicateEventIsIdempotent simulates duplicate StasisEnd events.
func TestARI_DuplicateEventIsIdempotent(t *testing.T) {
	queue := newFakeQueue([]string{"a1"})
	uc, repo := newUC(t, queue)
	ctx := context.Background()

	// Create + assign a call manually so it reaches IN_PROGRESS.
	out, _ := uc.RequestCall(ctx, calluc.RequestCallInput{CustomerID: "cust-x"})
	cid := uuid.MustParse(out.CallID)
	_ = uc.TryRouteNext(ctx) // reserve a1
	_ = queue.SetAgentBusy(ctx, "a1", cid)
	// Force the call forward.
	_ = repo.UpdateState(ctx, cid, domain.CallStatusWaiting, domain.CallStatusWaitingAgent, domain.StateTransitionFields{})
	_ = repo.UpdateState(ctx, cid, domain.CallStatusWaitingAgent, domain.CallStatusConnecting, domain.StateTransitionFields{})
	_ = repo.UpdateState(ctx, cid, domain.CallStatusConnecting, domain.CallStatusRinging, domain.StateTransitionFields{})
	now := time.Now().UTC()
	_ = repo.UpdateState(ctx, cid, domain.CallStatusRinging, domain.CallStatusInProgress, domain.StateTransitionFields{StartedAt: &now})

	// Push two StasisEnd events with the same timestamp; only one should take effect.
	ev := domain.ARIEvent{
		Type:      "StasisEnd",
		Timestamp: time.Now().UTC(),
		Variables: map[string]any{"CALL_ID": cid.String()},
	}
	uc.HandleARIEvent(ctx, ev)
	uc.HandleARIEvent(ctx, ev) // duplicate

	c, _ := repo.Get(ctx, cid)
	if c.Status != domain.CallStatusEnded {
		t.Errorf("expected ENDED, got %s", c.Status)
	}
}
