package worker

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/hoainguyen222/DongDo_CS_V2/internal/config"
	infraRedis "github.com/hoainguyen222/DongDo_CS_V2/internal/infra/redis"
	"github.com/rs/zerolog"
)

// RetryWorker periodically claims pending messages from Redis Stream consumer groups
// and moves them to the Dead Letter Queue after exceeding max retries.
type RetryWorker struct {
	eventBus    *infraRedis.EventBusService
	consumer    string
	cfg         config.WorkerRetryConfig
	retryCounts map[string]int
	mu          sync.Mutex
	logger      zerolog.Logger
}

// NewRetryWorker creates a new retry worker with the provided configuration.
func NewRetryWorker(
	eventBus *infraRedis.EventBusService,
	consumerName string,
	cfg config.WorkerRetryConfig,
) *RetryWorker {
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	logger = logger.With().Str("component", "retry_worker").Str("consumer", consumerName).Logger()

	// Apply defaults if not set
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 3
	}
	if cfg.ClaimAfter <= 0 {
		cfg.ClaimAfter = 60 * time.Second
	}
	if cfg.CheckInterval <= 0 {
		cfg.CheckInterval = 30 * time.Second
	}
	if cfg.ClaimBatchSize <= 0 {
		cfg.ClaimBatchSize = 20
	}

	return &RetryWorker{
		eventBus:    eventBus,
		consumer:    consumerName,
		cfg:         cfg,
		retryCounts: make(map[string]int),
		logger:      logger,
	}
}

// Start runs the periodic claim and dead-letter queue check.
func (w *RetryWorker) Start(ctx context.Context) {
	w.logger.Info().
		Int("max_retries", w.cfg.MaxRetries).
		Dur("claim_after", w.cfg.ClaimAfter).
		Dur("check_interval", w.cfg.CheckInterval).
		Int64("claim_batch_size", w.cfg.ClaimBatchSize).
		Msg("Retry & DLQ Worker started")

	defer w.logger.Info().Msg("Retry & DLQ Worker stopped")

	ticker := time.NewTicker(w.cfg.CheckInterval)
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
	claimedMsgs, _, claimErr := w.eventBus.AutoClaimPending(
		ctx,
		stream,
		group,
		w.consumer,
		w.cfg.ClaimAfter,
		"0-0",
		w.cfg.ClaimBatchSize,
	)
	if claimErr != nil || len(claimedMsgs) == 0 {
		return 0, 0, claimErr
	}

	for _, msg := range claimedMsgs {
		w.mu.Lock()
		w.retryCounts[msg.ID]++
		retries := w.retryCounts[msg.ID]
		w.mu.Unlock()

		if retries >= w.cfg.MaxRetries {
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
