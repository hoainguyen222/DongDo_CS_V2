package redis

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
)

// ============================================================
// InMemoryQueueManager — unit tests
// ============================================================

func newMem() *InMemoryQueueManager { return NewInMemoryQueueManager() }

func TestInMemory_EnqueueDequeue(t *testing.T) {
	q := newMem()
	ctx := context.Background()

	pos, err := q.EnqueueCall(ctx, 1)
	if err != nil || pos != 1 {
		t.Fatalf("enqueue 1: pos=%d err=%v", pos, err)
	}
	pos, err = q.EnqueueCall(ctx, 2)
	if err != nil || pos != 2 {
		t.Fatalf("enqueue 2: pos=%d err=%v", pos, err)
	}
	id, err := q.DequeueCall(ctx)
	if err != nil || id != 1 {
		t.Fatalf("dequeue: id=%d err=%v", id, err)
	}
	id, _ = q.DequeueCall(ctx)
	if id != 2 {
		t.Fatalf("dequeue 2: id=%d", id)
	}
	if _, err := q.DequeueCall(ctx); !errors.Is(err, domain.ErrQueueEmpty) {
		t.Fatalf("expected ErrQueueEmpty, got %v", err)
	}
}

func TestInMemory_AtomicReserveAgent_OnlyOneWins(t *testing.T) {
	q := newMem()
	ctx := context.Background()
	_, _ = q.EnqueueCall(ctx, 100)

	// 5 agents race for the same call.
	winners := 0
	for ext := 0; ext < 5; ext++ {
		extID := "100" + string(rune('0'+ext))
		ok, err := q.AtomicReserveAgent(ctx, 100, extID, time.Minute)
		if err != nil {
			t.Fatalf("AtomicReserveAgent %s: %v", extID, err)
		}
		if ok {
			winners++
		}
	}
	if winners != 1 {
		t.Fatalf("expected exactly 1 winner, got %d", winners)
	}
	st, _ := q.GetAgentState(ctx, "1000")
	if st != domain.AgentReserved {
		t.Fatalf("winning agent should be RESERVED, got %s", st)
	}
}

func TestInMemory_ReserveFailsWhenHeadMismatch(t *testing.T) {
	q := newMem()
	ctx := context.Background()
	_, _ = q.EnqueueCall(ctx, 1)
	_, _ = q.EnqueueCall(ctx, 2)

	// Try to reserve callID=99 — head is 1, should fail.
	ok, err := q.AtomicReserveAgent(ctx, 99, "1001", time.Minute)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if ok {
		t.Fatalf("expected reserve to fail when head mismatches")
	}
}

func TestInMemory_ReleaseReservation(t *testing.T) {
	q := newMem()
	ctx := context.Background()
	_, _ = q.EnqueueCall(ctx, 1)
	_, _ = q.AtomicReserveAgent(ctx, 1, "1001", time.Minute)
	if err := q.ReleaseReservation(ctx, "1001"); err != nil {
		t.Fatalf("release: %v", err)
	}
	st, _ := q.GetAgentState(ctx, "1001")
	if st != domain.AgentAvailable {
		t.Fatalf("expected AVAILABLE after release, got %s", st)
	}
}

func TestInMemory_IdempotencyReserve(t *testing.T) {
	q := newMem()
	ctx := context.Background()
	_, hit, err := q.ReserveIdempotency(ctx, "k1", "payload-a", time.Minute)
	if err != nil || hit {
		t.Fatalf("first reserve: hit=%v err=%v", hit, err)
	}
	existing, hit, err := q.ReserveIdempotency(ctx, "k1", "payload-b", time.Minute)
	if err != nil || !hit || existing != "payload-a" {
		t.Fatalf("second reserve: hit=%v existing=%q err=%v", hit, existing, err)
	}
}
