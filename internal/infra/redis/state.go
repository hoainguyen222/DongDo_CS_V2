package redis

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type StateService struct {
	client *Client
}

func NewStateManager(client *Client) *StateService {
	return &StateService{client: client}
}

// ============================================================
// 1. Typing Indicator State (TTL: 3 seconds)
// ============================================================

func (s *StateService) SetTyping(ctx context.Context, sessionID, userID string) error {
	key := fmt.Sprintf("typing:%s:%s", sessionID, userID)
	return s.client.rdb.Set(ctx, key, "1", 3*time.Second).Err()
}

func (s *StateService) IsTyping(ctx context.Context, sessionID, userID string) (bool, error) {
	key := fmt.Sprintf("typing:%s:%s", sessionID, userID)
	res, err := s.client.rdb.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return res > 0, nil
}

// ============================================================
// 2. Unread Message Counter State
// ============================================================

func (s *StateService) IncrementUnread(ctx context.Context, sessionID, recipientID string) (int64, error) {
	key := fmt.Sprintf("unread:%s:%s", sessionID, recipientID)
	return s.client.rdb.Incr(ctx, key).Result()
}

func (s *StateService) GetUnread(ctx context.Context, sessionID, recipientID string) (int64, error) {
	key := fmt.Sprintf("unread:%s:%s", sessionID, recipientID)
	val, err := s.client.rdb.Get(ctx, key).Int64()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return 0, nil
		}
		return 0, err
	}
	return val, nil
}

func (s *StateService) ClearUnread(ctx context.Context, sessionID, recipientID string) error {
	key := fmt.Sprintf("unread:%s:%s", sessionID, recipientID)
	return s.client.rdb.Del(ctx, key).Err()
}

// ============================================================
// 3. Distributed Lock (chống Race Condition, ví dụ: 2 CSKH cùng take 1 case)
// ============================================================

func (s *StateService) AcquireLock(ctx context.Context, key, owner string, ttl time.Duration) (bool, error) {
	lockKey := fmt.Sprintf("lock:%s", key)
	return s.client.rdb.SetNX(ctx, lockKey, owner, ttl).Result()
}

func (s *StateService) ReleaseLock(ctx context.Context, key, owner string) error {
	lockKey := fmt.Sprintf("lock:%s", key)
	// Atomic release using Lua script to ensure only the owner can release the lock
	script := `
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("del", KEYS[1])
		else
			return 0
		end
	`
	return s.client.rdb.Eval(ctx, script, []string{lockKey}, owner).Err()
}

// ============================================================
// 4. AI Execution State (TTL: 30 seconds to prevent overlapping AI calls)
// ============================================================

func (s *StateService) SetAIExecution(ctx context.Context, sessionID string, active bool) error {
	key := fmt.Sprintf("ai_exec:%s", sessionID)
	if active {
		return s.client.rdb.Set(ctx, key, "1", 30*time.Second).Err()
	}
	return s.client.rdb.Del(ctx, key).Err()
}

func (s *StateService) IsAIExecuting(ctx context.Context, sessionID string) (bool, error) {
	key := fmt.Sprintf("ai_exec:%s", sessionID)
	res, err := s.client.rdb.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return res > 0, nil
}
