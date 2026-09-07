package worker

import (
	"context"
	"encoding/json"
	"os"
	"sync"
	"time"

	"github.com/hoainguyen222/DongDo_CS_V2/internal/config"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
	infraRedis "github.com/hoainguyen222/DongDo_CS_V2/internal/infra/redis"
	"github.com/rs/zerolog"
)

// DBWorker consumes messages from Redis Stream and batch inserts to PostgreSQL.
// It implements configurable batch processing with flush intervals and safety caps.
type DBWorker struct {
	eventBus      *infraRedis.EventBusService
	messageRepo   domain.MessageRepository
	consumer      string
	cfg           config.WorkerDBConfig
	buffer        []*domain.Message
	msgIDs        []string
	mu            sync.Mutex
	logger        zerolog.Logger
}

// NewDBWorker creates a new DB worker with the provided configuration.
func NewDBWorker(
	eventBus *infraRedis.EventBusService,
	messageRepo domain.MessageRepository,
	consumerName string,
	cfg config.WorkerDBConfig,
) *DBWorker {
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	logger = logger.With().Str("component", "db_worker").Str("consumer", consumerName).Logger()

	// Apply defaults if not set
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 50
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = 2 * time.Second
	}
	if cfg.MaxBufferSize <= 0 {
		cfg.MaxBufferSize = 5000
	}
	if cfg.ReadCount <= 0 {
		cfg.ReadCount = 50
	}
	if cfg.BlockTimeout <= 0 {
		cfg.BlockTimeout = 1 * time.Second
	}
	if cfg.RetryDelay <= 0 {
		cfg.RetryDelay = 200 * time.Millisecond
	}

	return &DBWorker{
		eventBus:      eventBus,
		messageRepo:   messageRepo,
		consumer:      consumerName,
		cfg:           cfg,
		buffer:        make([]*domain.Message, 0, cfg.BatchSize),
		msgIDs:        make([]string, 0, cfg.BatchSize),
		logger:        logger,
	}
}

// Start runs the worker loop consuming from stream:db and batch inserting to PostgreSQL.
func (w *DBWorker) Start(ctx context.Context) {
	w.logger.Info().
		Int("batch_size", w.cfg.BatchSize).
		Dur("flush_interval", w.cfg.FlushInterval).
		Int("max_buffer", w.cfg.MaxBufferSize).
		Msg("Database Batch Worker started")

	defer w.logger.Info().Msg("Database Batch Worker stopped")

	ticker := time.NewTicker(w.cfg.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.Flush(context.Background())
			return

		case <-ticker.C:
			w.Flush(ctx)

		default:
			messages, err := w.eventBus.ReadStreamGroup(
				ctx,
				infraRedis.StreamDB,
				infraRedis.GroupDB,
				w.consumer,
				w.cfg.ReadCount,
				w.cfg.BlockTimeout,
			)
			if err != nil {
				w.logger.Error().Err(err).Msg("Error reading from DB stream")
				time.Sleep(w.cfg.RetryDelay)
				continue
			}

			if len(messages) == 0 {
				continue
			}

			w.mu.Lock()
			for _, xmsg := range messages {
				msgStr, _ := xmsg.Values["message"].(string)
				var msg domain.Message
				if err := json.Unmarshal([]byte(msgStr), &msg); err == nil {
					w.buffer = append(w.buffer, &msg)
					w.msgIDs = append(w.msgIDs, xmsg.ID)
				}
			}

			// Check if we need to flush due to batch size
			needFlush := len(w.buffer) >= w.cfg.BatchSize

			// Safety check: don't let buffer grow unbounded
			if len(w.buffer) >= w.cfg.MaxBufferSize {
				w.logger.Warn().
					Int("buffer_size", len(w.buffer)).
					Int("max_buffer", w.cfg.MaxBufferSize).
					Msg("Buffer limit reached, forcing flush")
				needFlush = true
			}
			w.mu.Unlock()

			if needFlush {
				w.Flush(ctx)
			}
		}
	}
}

// Flush executes batch write to PostgreSQL and acks all processed message IDs.
func (w *DBWorker) Flush(ctx context.Context) {
	w.mu.Lock()
	if len(w.buffer) == 0 {
		w.mu.Unlock()
		return
	}

	msgsToFlush := w.buffer
	idsToAck := w.msgIDs
	w.buffer = make([]*domain.Message, 0, w.cfg.BatchSize)
	w.msgIDs = make([]string, 0, w.cfg.BatchSize)
	w.mu.Unlock()

	if err := w.messageRepo.InsertBatch(ctx, msgsToFlush); err != nil {
		w.logger.Error().Err(err).Int("batch_size", len(msgsToFlush)).Msg("Failed to batch insert messages")
		return
	}

	if err := w.eventBus.AckMessage(ctx, infraRedis.StreamDB, infraRedis.GroupDB, idsToAck...); err != nil {
		w.logger.Error().Err(err).Msg("Failed to acknowledge messages")
	}
}
