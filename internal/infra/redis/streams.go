package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
	"github.com/redis/go-redis/v9"
)

const (
	StreamWS = "stream:ws"
	StreamAI = "stream:ai"
	StreamDB = "stream:db"
	StreamDLQ = "stream:dlq"

	GroupWS = "ws_group"
	GroupAI = "ai_group"
	GroupDB = "db_group"

	// Retention: trim stream to 5000 max items (approximate)
	StreamMaxLen = 5000
)

type EventBusService struct {
	client *Client
	hub    domain.HubBroadcaster
}

func NewEventBus(client *Client) *EventBusService {
	eb := &EventBusService{client: client}
	eb.initGroups(context.Background())
	return eb
}

func (eb *EventBusService) SetHub(hub domain.HubBroadcaster) {
	eb.hub = hub
}

func (eb *EventBusService) initGroups(ctx context.Context) {
	streams := []struct {
		stream string
		group  string
	}{
		{StreamWS, GroupWS},
		{StreamAI, GroupAI},
		{StreamDB, GroupDB},
	}

	for _, s := range streams {
		err := eb.client.rdb.XGroupCreateMkStream(ctx, s.stream, s.group, "0").Err()
		if err != nil && !isBusyGroupErr(err) {
			log.Printf("⚠️ Warning creating consumer group %s on %s: %v", s.group, s.stream, err)
		}
	}
}

func isBusyGroupErr(err error) bool {
	return err != nil && (err.Error() == "BUSYGROUP Consumer Group name already exists" ||
		err.Error() == "BUSYGROUP Group name already exists")
}

// PublishWS publishes an event to the WebSocket dispatch stream and local hub.
func (eb *EventBusService) PublishWS(ctx context.Context, sessionID string, event domain.WSEventType, payload interface{}, senderID string) error {
	// 1. Instant local broadcast with 0 latency
	if eb.hub != nil {
		eb.hub.BroadcastToSession(sessionID, &domain.WSEvent{
			Type:      event,
			SessionID: sessionID,
			Payload:   payload,
			SenderID:  senderID,
			Timestamp: time.Now(),
		})
	}

	// 2. Stream to Redis
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal WS payload: %w", err)
	}

	args := &redis.XAddArgs{
		Stream: StreamWS,
		MaxLen: StreamMaxLen,
		Approx: true,
		Values: map[string]interface{}{
			"session_id": sessionID,
			"event":      string(event),
			"payload":    string(payloadBytes),
			"sender_id":  senderID,
			"timestamp":  time.Now().UnixMilli(),
		},
	}

	if eb.client != nil && eb.client.rdb != nil {
		return eb.client.rdb.XAdd(ctx, args).Err()
	}
	return nil
}

// PublishAIJob publishes a question to the AI processing stream.
func (eb *EventBusService) PublishAIJob(ctx context.Context, sessionID string, query string, senderID string, clientMsgID *uuid.UUID) error {
	var cMsgIDStr string
	if clientMsgID != nil {
		cMsgIDStr = clientMsgID.String()
	}

	args := &redis.XAddArgs{
		Stream: StreamAI,
		MaxLen: StreamMaxLen,
		Approx: true,
		Values: map[string]interface{}{
			"session_id":    sessionID,
			"query":         query,
			"sender_id":     senderID,
			"client_msg_id": cMsgIDStr,
			"timestamp":     time.Now().UnixMilli(),
		},
	}

	return eb.client.rdb.XAdd(ctx, args).Err()
}

// PublishDBJob publishes a message to the database batch write stream.
func (eb *EventBusService) PublishDBJob(ctx context.Context, msg *domain.Message) error {
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal DB message: %w", err)
	}

	args := &redis.XAddArgs{
		Stream: StreamDB,
		MaxLen: StreamMaxLen,
		Approx: true,
		Values: map[string]interface{}{
			"session_id": msg.SessionID,
			"message":    string(msgBytes),
			"timestamp":  time.Now().UnixMilli(),
		},
	}

	return eb.client.rdb.XAdd(ctx, args).Err()
}

// ReadStreamGroup reads messages from a stream using consumer group.
func (eb *EventBusService) ReadStreamGroup(ctx context.Context, stream, group, consumer string, count int64, block time.Duration) ([]redis.XMessage, error) {
	res, err := eb.client.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    group,
		Consumer: consumer,
		Streams:  []string{stream, ">"},
		Count:    count,
		Block:    block,
	}).Result()

	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, err
	}

	if len(res) == 0 {
		return nil, nil
	}

	return res[0].Messages, nil
}

// AckMessage acknowledges a processed stream message.
func (eb *EventBusService) AckMessage(ctx context.Context, stream, group string, messageIDs ...string) error {
	return eb.client.rdb.XAck(ctx, stream, group, messageIDs...).Err()
}

// AutoClaimPending claims pending messages that exceeded minIdle time.
func (eb *EventBusService) AutoClaimPending(ctx context.Context, stream, group, consumer string, minIdle time.Duration, start string, count int64) ([]redis.XMessage, string, error) {
	res, nextStart, err := eb.client.rdb.XAutoClaim(ctx, &redis.XAutoClaimArgs{
		Stream:   stream,
		Group:    group,
		Consumer: consumer,
		MinIdle:  minIdle,
		Start:    start,
		Count:    count,
	}).Result()

	return res, nextStart, err
}

// MoveToDLQ moves a failed message to the Dead Letter Queue.
func (eb *EventBusService) MoveToDLQ(ctx context.Context, originalStream, messageID, reason string, values map[string]interface{}) error {
	dlqValues := make(map[string]interface{})
	for k, v := range values {
		dlqValues[k] = v
	}
	dlqValues["_orig_stream"] = originalStream
	dlqValues["_orig_msg_id"] = messageID
	dlqValues["_fail_reason"] = reason
	dlqValues["_failed_at"] = time.Now().UnixMilli()

	args := &redis.XAddArgs{
		Stream: StreamDLQ,
		MaxLen: StreamMaxLen,
		Approx: true,
		Values: dlqValues,
	}

	return eb.client.rdb.XAdd(ctx, args).Err()
}
