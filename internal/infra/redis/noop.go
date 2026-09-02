package redis

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
)

// NoOpEventBus is a fallback event bus when Redis is not available, with direct Hub broadcasting.
type NoOpEventBus struct {
	hub domain.HubBroadcaster
}

func NewNoOpEventBus() domain.EventBus {
	return &NoOpEventBus{}
}

func (n *NoOpEventBus) SetHub(hub domain.HubBroadcaster) {
	n.hub = hub
}

func (n *NoOpEventBus) PublishWS(ctx context.Context, sessionID string, event domain.WSEventType, payload interface{}, senderID string) error {
	if n.hub != nil {
		n.hub.BroadcastToSession(sessionID, &domain.WSEvent{
			Type:      event,
			SessionID: sessionID,
			Payload:   payload,
			SenderID:  senderID,
			Timestamp: time.Now(),
		})
	}
	return nil
}

func (n *NoOpEventBus) PublishAIJob(ctx context.Context, sessionID string, query string, senderID string, clientMsgID *uuid.UUID) error {
	return nil
}

func (n *NoOpEventBus) PublishDBJob(ctx context.Context, msg *domain.Message) error {
	return nil
}

// NoOpStateManager is a fallback in-memory state manager when Redis is not available.
type NoOpStateManager struct{}

func NewNoOpStateManager() domain.StateManager {
	return &NoOpStateManager{}
}

func (n *NoOpStateManager) SetTyping(ctx context.Context, sessionID, userID string) error {
	return nil
}

func (n *NoOpStateManager) IsTyping(ctx context.Context, sessionID, userID string) (bool, error) {
	return false, nil
}

func (n *NoOpStateManager) IncrementUnread(ctx context.Context, sessionID, recipientID string) (int64, error) {
	return 0, nil
}

func (n *NoOpStateManager) GetUnread(ctx context.Context, sessionID, recipientID string) (int64, error) {
	return 0, nil
}

func (n *NoOpStateManager) ClearUnread(ctx context.Context, sessionID, recipientID string) error {
	return nil
}

func (n *NoOpStateManager) AcquireLock(ctx context.Context, key, owner string, ttl time.Duration) (bool, error) {
	return true, nil
}

func (n *NoOpStateManager) ReleaseLock(ctx context.Context, key, owner string) error {
	return nil
}

func (n *NoOpStateManager) SetAIExecution(ctx context.Context, sessionID string, active bool) error {
	return nil
}

func (n *NoOpStateManager) IsAIExecuting(ctx context.Context, sessionID string) (bool, error) {
	return false, nil
}
