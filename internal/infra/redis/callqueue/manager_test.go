package callqueue_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/infra/redis/callqueue"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

// newTestManager connects to a Redis at REDIS_TEST_ADDR or skips the test
// when none is configured. This lets CI run against a real Redis side-car
// while local dev can `docker run -d -p 6379:6379 redis:7-alpine`.
//
// Each call also wipes the queue key so tests are independent.
func newTestManager(t *testing.T) (*callqueue.Manager, func()) {
	t.Helper()
	addr := "localhost:6379"
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skipf("redis at %s not reachable (%v) — skipping redis integration test", addr, err)
	}
	// Flush leftover state to keep tests deterministic.
	_ = rdb.Del(context.Background(), "call:queue:waiting").Err()
	// We can't easily enumerate agent:* keys here; tests use unique IDs.
	return callqueue.New(rdb, zerolog.Nop()), func() { _ = rdb.Close() }
}

// TestEnqueuePopNext asserts FIFO order and that PopNext returns false on empty.
func TestEnqueuePopNext(t *testing.T) {
	m, cleanup := newTestManager(t)
	defer cleanup()
	ctx := context.Background()

	c1, c2, c3 := uuid.New(), uuid.New(), uuid.New()
	if err := m.Enqueue(ctx, c1, 0, time.Now().UnixMilli()); err != nil {
		t.Fatalf("enqueue c1: %v", err)
	}
	time.Sleep(2 * time.Millisecond)
	if err := m.Enqueue(ctx, c2, 0, time.Now().UnixMilli()); err != nil {
		t.Fatalf("enqueue c2: %v", err)
	}
	time.Sleep(2 * time.Millisecond)
	if err := m.Enqueue(ctx, c3, 0, time.Now().UnixMilli()); err != nil {
		t.Fatalf("enqueue c3: %v", err)
	}

	for i, want := range []uuid.UUID{c1, c2, c3} {
		got, ok, err := m.PopNext(ctx)
		if err != nil || !ok {
			t.Fatalf("[%d] pop: ok=%v err=%v", i, ok, err)
		}
		if got != want {
			t.Errorf("[%d] order: got %s want %s", i, got, want)
		}
	}

	if _, ok, _ := m.PopNext(ctx); ok {
		t.Errorf("expected empty queue")
	}
}

// TestEnqueue_Priority ensures a higher-priority (lower-score) call pops first.
func TestEnqueue_Priority(t *testing.T) {
	m, cleanup := newTestManager(t)
	defer cleanup()
	ctx := context.Background()

	normal := uuid.New()
	urgent := uuid.New()

	now := time.Now().UnixMilli()
	_ = m.Enqueue(ctx, normal, 0, now)
	_ = m.Enqueue(ctx, urgent, 10, now+1) // priority 10 → very low score

	got, ok, err := m.PopNext(ctx)
	if err != nil || !ok {
		t.Fatalf("pop: %v", err)
	}
	if got != urgent {
		t.Errorf("expected urgent %s first, got %s", urgent, got)
	}
}

// TestReserveAgent_Atomicity is the critical test: two goroutines attempt to reserve
// the same AVAILABLE agent. Exactly one must win.
func TestReserveAgent_Atomicity(t *testing.T) {
	m, cleanup := newTestManager(t)
	defer cleanup()
	ctx := context.Background()
	agent := "agent-1"
	_ = m.SetAgentAvailable(ctx, agent)

	// Take it down first so we have a clean AVAILABLE state for both goroutines.
	if ok, st, err := m.ReserveAgent(ctx, agent, uuid.New()); err != nil || !ok {
		t.Fatalf("init reserve: ok=%v st=%s err=%v", ok, st, err)
	}
	// Release back to AVAILABLE.
	if _, err := m.ReleaseAgent(ctx, agent); err != nil {
		t.Fatalf("release: %v", err)
	}

	// Now run 2 concurrent reserves.
	c1, c2 := uuid.New(), uuid.New()
	type res struct {
		ok    bool
		state domain.AgentState
		err   error
	}
	out := make(chan res, 2)
	go func() { ok, st, err := m.ReserveAgent(ctx, agent, c1); out <- res{ok, st, err} }()
	go func() { ok, st, err := m.ReserveAgent(ctx, agent, c2); out <- res{ok, st, err} }()

	wins := 0
	for i := 0; i < 2; i++ {
		r := <-out
		if r.err != nil {
			t.Fatalf("reserve err: %v", r.err)
		}
		if r.ok {
			wins++
		}
	}
	if wins != 1 {
		t.Fatalf("expected exactly 1 winner, got %d", wins)
	}
	a, _ := m.GetAgent(ctx, agent)
	if a.State != domain.AgentReserved {
		t.Errorf("expected RESERVED, got %s", a.State)
	}
}

// TestSetAgentBusyOnlyFromReserved guards the state-machine rule.
func TestSetAgentBusyOnlyFromReserved(t *testing.T) {
	m, cleanup := newTestManager(t)
	defer cleanup()
	ctx := context.Background()
	agent := "agent-busy"

	// From AVAILABLE → setBusy must fail.
	_ = m.SetAgentAvailable(ctx, agent)
	if err := m.SetAgentBusy(ctx, agent, uuid.New()); err == nil {
		t.Errorf("expected error setting BUSY from AVAILABLE")
	}

	// Reserve → setBusy must succeed.
	_, _, _ = m.ReserveAgent(ctx, agent, uuid.New())
	if err := m.SetAgentBusy(ctx, agent, uuid.New()); err != nil {
		t.Errorf("set busy after reserve: %v", err)
	}
	a, _ := m.GetAgent(ctx, agent)
	if a.State != domain.AgentBusy {
		t.Errorf("expected BUSY, got %s", a.State)
	}
}

// TestReleaseAgent_FreesBinding verifies after release the agent has no current call.
func TestReleaseAgent_FreesBinding(t *testing.T) {
	m, cleanup := newTestManager(t)
	defer cleanup()
	ctx := context.Background()
	agent := "agent-free"
	callID := uuid.New()

	_ = m.SetAgentAvailable(ctx, agent)
	ok, _, err := m.ReserveAgent(ctx, agent, callID)
	if err != nil || !ok {
		t.Fatalf("reserve: ok=%v err=%v", ok, err)
	}
	if _, err := m.ReleaseAgent(ctx, agent); err != nil {
		t.Fatalf("release: %v", err)
	}
	a, _ := m.GetAgent(ctx, agent)
	if a.State != domain.AgentAvailable {
		t.Errorf("expected AVAILABLE, got %s", a.State)
	}
	if a.CurrentCall != nil {
		t.Errorf("expected no current call, got %s", *a.CurrentCall)
	}
}

// TestQueuePosition asserts enqueue ordering produces increasing rank.
func TestQueuePosition(t *testing.T) {
	m, cleanup := newTestManager(t)
	defer cleanup()
	ctx := context.Background()

	a, b := uuid.New(), uuid.New()
	_ = m.Enqueue(ctx, a, 0, time.Now().UnixMilli())
	time.Sleep(2 * time.Millisecond)
	_ = m.Enqueue(ctx, b, 0, time.Now().UnixMilli())

	pa, _ := m.QueuePosition(ctx, a)
	pb, _ := m.QueuePosition(ctx, b)
	if pa != 0 || pb != 1 {
		t.Errorf("positions wrong: a=%d b=%d", pa, pb)
	}
}
