package redis

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

type StateService struct {
	client *Client
	logger zerolog.Logger
}

func NewStateManager(client *Client) *StateService {
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	logger = logger.With().Str("component", "state_manager").Logger()

	return &StateService{client: client, logger: logger}
}

// ============================================================
// 1. Typing Indicator State (TTL: 3 seconds)
// ============================================================

func (s *StateService) SetTyping(ctx context.Context, sessionID, userID string) error {
	key := fmt.Sprintf("typing:%s:%s", sessionID, userID)

	if err := s.client.rdb.Set(ctx, key, "1", 3*time.Second).Err(); err != nil {
		s.logger.Error().Err(err).Msg("Failed to set typing indicator")
		return err
	}

	return nil
}

func (s *StateService) IsTyping(ctx context.Context, sessionID, userID string) (bool, error) {
	key := fmt.Sprintf("typing:%s:%s", sessionID, userID)

	res, err := s.client.rdb.Exists(ctx, key).Result()
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to check typing indicator")
		return false, err
	}

	return res > 0, nil
}

// ============================================================
// 2. Unread Message Counter State
// ============================================================

func (s *StateService) IncrementUnread(ctx context.Context, sessionID, recipientID string) (int64, error) {
	key := fmt.Sprintf("unread:%s:%s", sessionID, recipientID)

	count, err := s.client.rdb.Incr(ctx, key).Result()
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to increment unread counter")
		return 0, err
	}

	return count, nil
}

func (s *StateService) GetUnread(ctx context.Context, sessionID, recipientID string) (int64, error) {
	key := fmt.Sprintf("unread:%s:%s", sessionID, recipientID)

	val, err := s.client.rdb.Get(ctx, key).Int64()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return 0, nil
		}
		s.logger.Error().Err(err).Msg("Failed to get unread count")
		return 0, err
	}

	return val, nil
}

func (s *StateService) ClearUnread(ctx context.Context, sessionID, recipientID string) error {
	key := fmt.Sprintf("unread:%s:%s", sessionID, recipientID)

	if err := s.client.rdb.Del(ctx, key).Err(); err != nil {
		s.logger.Error().Err(err).Msg("Failed to clear unread counter")
		return err
	}

	return nil
}

// ============================================================
// 3. Distributed Lock (chống Race Condition, ví dụ: 2 CSKH cùng take 1 case)
// ============================================================

func (s *StateService) AcquireLock(ctx context.Context, key, owner string, ttl time.Duration) (bool, error) {
	lockKey := fmt.Sprintf("lock:%s", key)

	acquired, err := s.client.rdb.SetNX(ctx, lockKey, owner, ttl).Result()
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to acquire distributed lock")
		return false, err
	}

	return acquired, nil
}

func (s *StateService) ReleaseLock(ctx context.Context, key, owner string) error {
	lockKey := fmt.Sprintf("lock:%s", key)

	script := `
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("del", KEYS[1])
		else
			return 0
		end
	`

	if err := s.client.rdb.Eval(ctx, script, []string{lockKey}, owner).Err(); err != nil {
		s.logger.Error().Err(err).Msg("Failed to release distributed lock")
		return err
	}

	return nil
}

// ============================================================
// 4. AI Execution State (TTL: 30 seconds to prevent overlapping AI calls)
// ============================================================

func (s *StateService) SetAIExecution(ctx context.Context, sessionID string, active bool) error {
	key := fmt.Sprintf("ai_exec:%s", sessionID)

	var err error
	if active {
		err = s.client.rdb.Set(ctx, key, "1", 30*time.Second).Err()
	} else {
		err = s.client.rdb.Del(ctx, key).Err()
	}

	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to set AI execution state")
		return err
	}

	return nil
}

func (s *StateService) IsAIExecuting(ctx context.Context, sessionID string) (bool, error) {
	key := fmt.Sprintf("ai_exec:%s", sessionID)

	res, err := s.client.rdb.Exists(ctx, key).Result()
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to check AI execution state")
		return false, err
	}

	return res > 0, nil
}