package worker

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/hoainguyen222/DongDo_CS_V2/internal/delivery/ws"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
	infraRedis "github.com/hoainguyen222/DongDo_CS_V2/internal/infra/redis"
)

type WSWorker struct {
	eventBus *infraRedis.EventBusService
	hub      *ws.Hub
	consumer string
}

func NewWSWorker(eventBus *infraRedis.EventBusService, hub *ws.Hub, consumerName string) *WSWorker {
	return &WSWorker{
		eventBus: eventBus,
		hub:      hub,
		consumer: consumerName,
	}
}

// Start runs the worker loop consuming from stream:ws with consumer group ws_group.
func (w *WSWorker) Start(ctx context.Context) {
	log.Println("🚀 Started WS Worker...")

	for {
		select {
		case <-ctx.Done():
			log.Println("🛑 WS Worker stopped")
			return
		default:
			// Read with 2-second block timeout
			messages, err := w.eventBus.ReadStreamGroup(ctx, infraRedis.StreamWS, infraRedis.GroupWS, w.consumer, 10, 2*time.Second)
			if err != nil {
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

				// Broadcast to session or admin inbox
				w.hub.BroadcastToSession(sessionID, event)

				// XACK after successful dispatch
				_ = w.eventBus.AckMessage(ctx, infraRedis.StreamWS, infraRedis.GroupWS, xmsg.ID)
			}
		}
	}
}
