package worker

import (
	"context"
	"encoding/json"
	"os"
	"time"

	"github.com/hoainguyen222/DongDo_CS_V2/internal/config"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/delivery/ws"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
	infraRedis "github.com/hoainguyen222/DongDo_CS_V2/internal/infra/redis"
	"github.com/rs/zerolog"
)

// WSWorker consumes messages from Redis Stream and broadcasts to WebSocket clients.
// It processes WebSocket events with configurable read counts and timeouts.
type WSWorker struct {
	eventBus *infraRedis.EventBusService
	hub      *ws.Hub
	consumer string
	cfg      config.WorkerWSConfig
	logger   zerolog.Logger
}

// NewWSWorker creates a new WS worker with the provided configuration.
func NewWSWorker(
	eventBus *infraRedis.EventBusService,
	hub *ws.Hub,
	consumerName string,
	cfg config.WorkerWSConfig,
) *WSWorker {
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	logger = logger.With().Str("component", "ws_worker").Str("consumer", consumerName).Logger()

	// Apply defaults if not set
	if cfg.ReadCount <= 0 {
		cfg.ReadCount = 10
	}
	if cfg.BlockTimeout <= 0 {
		cfg.BlockTimeout = 2 * time.Second
	}
	if cfg.RetryDelay <= 0 {
		cfg.RetryDelay = 500 * time.Millisecond
	}

	return &WSWorker{
		eventBus: eventBus,
		hub:      hub,
		consumer: consumerName,
		cfg:      cfg,
		logger:   logger,
	}
}

// Start runs the worker loop consuming from stream:ws with consumer group ws_group.
func (w *WSWorker) Start(ctx context.Context) {
	w.logger.Info().
		Int64("read_count", w.cfg.ReadCount).
		Dur("block_timeout", w.cfg.BlockTimeout).
		Msg("WS Worker started")

	defer w.logger.Info().Msg("WS Worker stopped")

	for {
		select {
		case <-ctx.Done():
			return
		default:
			messages, err := w.eventBus.ReadStreamGroup(
				ctx,
				infraRedis.StreamWS,
				infraRedis.GroupWS,
				w.consumer,
				w.cfg.ReadCount,
				w.cfg.BlockTimeout,
			)
			if err != nil {
				w.logger.Error().Err(err).Msg("Error reading from WS stream")
				time.Sleep(w.cfg.RetryDelay)
				continue
			}

			for _, xmsg := range messages {
				sessionID, _ := xmsg.Values["session_id"].(string)
				eventType, _ := xmsg.Values["event"].(string)
				payloadStr, _ := xmsg.Values["payload"].(string)
				senderID, _ := xmsg.Values["sender_id"].(string)

				var payload interface{}
				_ = json.Unmarshal([]byte(payloadStr), &payload)

				event := &domain.WSEvent{
					Type:      domain.WSEventType(eventType),
					SessionID: sessionID,
					Payload:   payload,
					SenderID:  senderID,
					Timestamp: time.Now(),
				}

				w.hub.BroadcastToSession(sessionID, event)

				if err := w.eventBus.AckMessage(ctx, infraRedis.StreamWS, infraRedis.GroupWS, xmsg.ID); err != nil {
					w.logger.Error().Err(err).Msg("Failed to acknowledge WS message")
				}
			}
		}
	}
}
