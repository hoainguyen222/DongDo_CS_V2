package worker

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	infraRedis "github.com/hoainguyen222/DongDo_CS_V2/internal/infra/redis"
)

type RetryWorker struct {
	eventBus    *infraRedis.EventBusService
	consumer    string
	maxRetries  int
	minIdleTime time.Duration
	retryCounts map[string]int
	mu          sync.Mutex
}

func NewRetryWorker(
	eventBus *infraRedis.EventBusService,
	consumerName string,
	maxRetries int,
	minIdleSeconds int,
) *RetryWorker {
	if maxRetries <= 0 {
		maxRetries = 3
	}
	if minIdleSeconds <= 0 {
		minIdleSeconds = 60
	}
	return &RetryWorker{
		eventBus:    eventBus,
		consumer:    consumerName,
		maxRetries:  maxRetries,
		minIdleTime: time.Duration(minIdleSeconds) * time.Second,
		retryCounts: make(map[string]int),
	}
}

// Start runs the periodic claim and dead-letter queue check every 30 seconds.
func (w *RetryWorker) Start(ctx context.Context) {
	log.Println("🔄 Started Retry & Dead Letter Queue (DLQ) Worker...")

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	streams := []struct {
		stream string
		group  string
	}{
		{infraRedis.StreamWS, infraRedis.GroupWS},
		{infraRedis.StreamAI, infraRedis.GroupAI},
		{infraRedis.StreamDB, infraRedis.GroupDB},
	}

	for {
		select {
		case <-ctx.Done():
			log.Println("🛑 Retry Worker stopped")
			return

		case <-ticker.C:
			for _, s := range streams {
				w.claimAndHandle(ctx, s.stream, s.group)
			}
		}
	}
}

func (w *RetryWorker) claimAndHandle(ctx context.Context, stream, group string) {
	claimed, _, err := w.eventBus.AutoClaimPending(ctx, stream, group, w.consumer, w.minIdleTime, "0-0", 20)
	if err != nil || len(claimed) == 0 {
		return
	}

	for _, msg := range claimed {
		w.mu.Lock()
		w.retryCounts[msg.ID]++
		retries := w.retryCounts[msg.ID]
		w.mu.Unlock()

		if retries >= w.maxRetries {
			log.Printf("⚠️ Message %s in stream %s exceeded max retries (%d). Moving to DLQ.", msg.ID, stream, retries)
			_ = w.eventBus.MoveToDLQ(ctx, stream, msg.ID, fmt.Sprintf("Exceeded max retries (%d)", retries), msg.Values)
			_ = w.eventBus.AckMessage(ctx, stream, group, msg.ID)

			w.mu.Lock()
			delete(w.retryCounts, msg.ID)
			w.mu.Unlock()
		} else {
			log.Printf("🔄 AutoClaimed pending message %s in stream %s (Attempt %d/%d)", msg.ID, stream, retries, w.maxRetries)
		}
	}
}
