package redis

import (
	"context"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
	"github.com/rs/zerolog"
)

// NoOpEventBus is a fallback event bus when Redis is not available, with direct Hub broadcasting.
type NoOpEventBus struct {
	hub    domain.HubBroadcaster
	logger zerolog.Logger
}

func NewNoOpEventBus() domain.EventBus {
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	logger = logger.With().Str("component", "noop_event_bus").Logger()

	logger.Warn().Msg("NoOpEventBus initialized - Redis streaming disabled")

	return &NoOpEventBus{logger: logger}
}

func (n *NoOpEventBus) SetHub(hub domain.HubBroadcaster) {
	n.hub = hub
	if hub == nil {
		n.logger.Warn().Msg("Hub broadcaster is nil for NoOpEventBus")
	}
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
	} else {
		n.logger.Warn().Str("session_id", sessionID).Msg("NoOpEventBus: event dropped (no hub)")
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
type NoOpStateManager struct {
	logger zerolog.Logger
}

func NewNoOpStateManager() domain.StateManager {
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	logger = logger.With().Str("component", "noop_state_manager").Logger()

	logger.Warn().Msg("NoOpStateManager initialized - Redis state disabled")

	return &NoOpStateManager{logger: logger}
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