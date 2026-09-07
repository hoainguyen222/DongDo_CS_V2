package redis

import (
	"context"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

const (
	StreamCallRequests = "stream:call_requests"
	GroupCallRouter    = "call-router-group"
)

type CallStreamManager struct {
	client *redis.Client
}

func NewCallStreamManager(client *redis.Client) *CallStreamManager {
	return &CallStreamManager{client: client}
}

func (m *CallStreamManager) EnsureConsumerGroup(ctx context.Context) {
	err := m.client.XGroupCreateMkStream(ctx, StreamCallRequests, GroupCallRouter, "0").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		log.Warn().Err(err).Msg("Notice creating Redis stream consumer group")
	}
}

func (m *CallStreamManager) PublishCallRequest(ctx context.Context, callID, customerID string) error {
	err := m.client.XAdd(ctx, &redis.XAddArgs{
		Stream: StreamCallRequests,
		Values: map[string]interface{}{
			"call_id":     callID,
			"customer_id": customerID,
		},
	}).Err()
	if err != nil {
		log.Error().Err(err).Str("call_id", callID).Msg("Failed to publish call request to Redis Stream")
		return err
	}
	return nil
}

func (m *CallStreamManager) ConsumeCallRequests(ctx context.Context, consumerName string, handler func(callID, customerID string) error) {
	m.EnsureConsumerGroup(ctx)

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("Stopping Redis Stream Call Request Consumer worker")
			return
		default:
			streams, err := m.client.XReadGroup(ctx, &redis.XReadGroupArgs{
				Group:    GroupCallRouter,
				Consumer: consumerName,
				Streams:  []string{StreamCallRequests, ">"},
				Count:    1,
				Block:    0,
			}).Result()

			if err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Error().Err(err).Msg("Error reading from Redis Stream call_requests")
				continue
			}

			for _, stream := range streams {
				for _, msg := range stream.Messages {
					callID, _ := msg.Values["call_id"].(string)
					customerID, _ := msg.Values["customer_id"].(string)

					if callID != "" {
						err := handler(callID, customerID)
						if err == nil {
							m.client.XAck(ctx, StreamCallRequests, GroupCallRouter, msg.ID)
						}
					}
				}
			}
		}
	}
}
