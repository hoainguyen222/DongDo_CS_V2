package worker

import (
	"context"
	"encoding/json"
	"os"
	"sync"
	"time"

	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
	infraRedis "github.com/hoainguyen222/DongDo_CS_V2/internal/infra/redis"
	"github.com/rs/zerolog"
)

type DBWorker struct {
	eventBus      *infraRedis.EventBusService
	messageRepo   domain.MessageRepository
	consumer      string
	batchSize     int
	flushInterval time.Duration
	buffer        []*domain.Message
	msgIDs        []string
	mu            sync.Mutex
	logger        zerolog.Logger
}

func NewDBWorker(
	eventBus *infraRedis.EventBusService,
	messageRepo domain.MessageRepository,
	consumerName string,
	batchSize int,
	flushInterval time.Duration,
) *DBWorker {
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	logger = logger.With().Str("component", "db_worker").Str("consumer", consumerName).Logger()

	if batchSize <= 0 {
		batchSize = 50
	}
	if flushInterval <= 0 {
		flushInterval = 2 * time.Second
	}

	return &DBWorker{
		eventBus:      eventBus,
		messageRepo:   messageRepo,
		consumer:      consumerName,
		batchSize:     batchSize,
		flushInterval: flushInterval,
		buffer:        make([]*domain.Message, 0, batchSize),
		msgIDs:        make([]string, 0, batchSize),
		logger:        logger,
	}
}

// Start runs the worker loop consuming from stream:db and batch inserting to PostgreSQL.
func (w *DBWorker) Start(ctx context.Context) {
	w.logger.Info().Msg("Database Batch Worker started")
	defer w.logger.Info().Msg("Database Batch Worker stopped")

	ticker := time.NewTicker(w.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.Flush(context.Background())
			return

		case <-ticker.C:
			w.Flush(ctx)

		default:
			messages, err := w.eventBus.ReadStreamGroup(ctx, infraRedis.StreamDB, infraRedis.GroupDB, w.consumer, int64(w.batchSize), 1*time.Second)
			if err != nil {
				w.logger.Error().Err(err).Msg("Error reading from DB stream")
				time.Sleep(200 * time.Millisecond)
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
			needFlush := len(w.buffer) >= w.batchSize
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
	w.buffer = make([]*domain.Message, 0, w.batchSize)
	w.msgIDs = make([]string, 0, w.batchSize)
	w.mu.Unlock()

	if err := w.messageRepo.InsertBatch(ctx, msgsToFlush); err != nil {
		w.logger.Error().Err(err).Int("batch_size", len(msgsToFlush)).Msg("Failed to batch insert messages")
		return
	}

	if err := w.eventBus.AckMessage(ctx, infraRedis.StreamDB, infraRedis.GroupDB, idsToAck...); err != nil {
		w.logger.Error().Err(err).Msg("Failed to acknowledge messages")
	}
}