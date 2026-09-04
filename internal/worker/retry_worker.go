package worker

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	infraRedis "github.com/hoainguyen222/DongDo_CS_V2/internal/infra/redis"
	"github.com/rs/zerolog"
)

type RetryWorker struct {
	eventBus    *infraRedis.EventBusService
	consumer    string
	maxRetries  int
	minIdleTime time.Duration
	retryCounts map[string]int
	mu          sync.Mutex
	logger      zerolog.Logger
}

func NewRetryWorker(
	eventBus *infraRedis.EventBusService,
	consumerName string,
	maxRetries int,
	minIdleSeconds int,
) *RetryWorker {
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	logger = logger.With().Str("component", "retry_worker").Str("consumer", consumerName).Logger()

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
		logger:      logger,
	}
}

// Start runs the periodic claim and dead-letter queue check every 30 seconds.
func (w *RetryWorker) Start(ctx context.Context) {
	w.logger.Info().Msg("Retry & DLQ Worker started")
	defer w.logger.Info().Msg("Retry & DLQ Worker stopped")

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
			return

		case <-ticker.C:
			for _, s := range streams {
				_, _, err := w.claimAndHandle(ctx, s.stream, s.group)
				if err != nil {
					w.logger.Error().Err(err).Str("stream", s.stream).Msg("Retry cycle error")
				}
			}
		}
	}
}

func (w *RetryWorker) claimAndHandle(ctx context.Context, stream, group string) (claimed int, dlq int, err error) {
	claimedMsgs, _, claimErr := w.eventBus.AutoClaimPending(ctx, stream, group, w.consumer, w.minIdleTime, "0-0", 20)
	if claimErr != nil || len(claimedMsgs) == 0 {
		return 0, 0, claimErr
	}

	for _, msg := range claimedMsgs {
		w.mu.Lock()
		w.retryCounts[msg.ID]++
		retries := w.retryCounts[msg.ID]
		w.mu.Unlock()

		if retries >= w.maxRetries {
			dlq++

			w.logger.Warn().
				Str("message_id", msg.ID).
				Str("stream", stream).
				Int("retries", retries).
				Msg("Message exceeded max retries - moving to DLQ")

			if err := w.eventBus.MoveToDLQ(ctx, stream, msg.ID, fmt.Sprintf("Exceeded max retries (%d)", retries), msg.Values); err != nil {
				w.logger.Error().Err(err).Str("message_id", msg.ID).Msg("CRITICAL: Failed to move message to DLQ")
			}

			if err := w.eventBus.AckMessage(ctx, stream, group, msg.ID); err != nil {
				w.logger.Error().Err(err).Msg("Failed to acknowledge after DLQ")
			}

			w.mu.Lock()
			delete(w.retryCounts, msg.ID)
			w.mu.Unlock()
		}
	}

	return len(claimedMsgs), dlq, nil
}