package worker

import (
	"context"
	"encoding/json"
	"os"
	"time"

	"github.com/hoainguyen222/DongDo_CS_V2/internal/delivery/ws"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
	infraRedis "github.com/hoainguyen222/DongDo_CS_V2/internal/infra/redis"
	"github.com/rs/zerolog"
)

type WSWorker struct {
	eventBus *infraRedis.EventBusService
	hub      *ws.Hub
	consumer string
	logger   zerolog.Logger
}

func NewWSWorker(eventBus *infraRedis.EventBusService, hub *ws.Hub, consumerName string) *WSWorker {
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	logger = logger.With().Str("component", "ws_worker").Str("consumer", consumerName).Logger()

	return &WSWorker{
		eventBus: eventBus,
		hub:      hub,
		consumer: consumerName,
		logger:   logger,
	}
}

// Start runs the worker loop consuming from stream:ws with consumer group ws_group.
func (w *WSWorker) Start(ctx context.Context) {
	w.logger.Info().Msg("WS Worker started")
	defer w.logger.Info().Msg("WS Worker stopped")

	for {
		select {
		case <-ctx.Done():
			return
		default:
			messages, err := w.eventBus.ReadStreamGroup(ctx, infraRedis.StreamWS, infraRedis.GroupWS, w.consumer, 10, 2*time.Second)
			if err != nil {
				w.logger.Error().Err(err).Msg("Error reading from WS stream")
				time.Sleep(500 * time.Millisecond)
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