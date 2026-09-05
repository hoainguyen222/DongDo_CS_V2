package redis

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
	"github.com/redis/go-redis/v9"
)

// ============================================================
// QueueManager — Redis-backed implementation
// ============================================================
//
// Owns the realtime queue, agent state and idempotency keys. The hot
// path uses atomic Lua scripts so concurrent callers cannot race on
// "reserve the next agent".
type QueueManager struct {
	client *Client
}

// NewQueueManager builds a Redis-backed QueueManager.
func NewQueueManager(client *Client) *QueueManager {
	return &QueueManager{client: client}
}

func (q *QueueManager) rdb() *redis.Client { return q.client.RDB() }

// EnqueueCall appends callID to the FIFO queue and stores the queue
// position on the call:{id}:state key as JSON-ish "POS:n". Returns
// the 1-based position.
func (q *QueueManager) EnqueueCall(ctx context.Context, callID int64) (int, error) {
	if err := q.rdb().RPush(ctx, keyCallQueue(), callID).Err(); err != nil {
		return 0, fmt.Errorf("redis: rpush queue: %w", err)
	}
	pos, err := q.rdb().LPos(ctx, keyCallQueue(), strconv.FormatInt(callID, 10), redis.LPosArgs{}).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return 0, nil
		}
		return 0, fmt.Errorf("redis: lpos: %w", err)
	}
	return int(pos) + 1, nil
}

// DequeueCall pops the head of the queue.
func (q *QueueManager) DequeueCall(ctx context.Context) (int64, error) {
	v, err := q.rdb().LPop(ctx, keyCallQueue()).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return 0, domain.ErrQueueEmpty
		}
		return 0, fmt.Errorf("redis: lpop: %w", err)
	}
	id, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("redis: parse call id: %w", err)
	}
	return id, nil
}

// QueuePosition returns the 1-based position. Returns 0 if not found.
func (q *QueueManager) QueuePosition(ctx context.Context, callID int64) (int, error) {
	pos, err := q.rdb().LPos(ctx, keyCallQueue(), strconv.FormatInt(callID, 10), redis.LPosArgs{}).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return 0, nil
		}
		return 0, fmt.Errorf("redis: lpos: %w", err)
	}
	return int(pos) + 1, nil
}

// reserveScript atomically:
//   1. Sets agent:{ext}:state = RESERVED with TTL.
//   2. Stores call:{id}:agent = ext.
//   3. Pops the queue head; pops only if it equals `expectedHead`.
// Returns 1 on success, 0 on mismatch.
var reserveScript = redis.NewScript(`
local agentKey    = KEYS[1]
local callAgentKey = KEYS[2]
local queueKey    = KEYS[3]
local ttl         = tonumber(ARGV[1])
local agentExt    = ARGV[2]
local callIDStr   = ARGV[3]
local expectedHead = ARGV[4]

local cur = redis.call('GET', agentKey)
if cur ~= false and cur ~= 'AVAILABLE' then
  return 0
end

-- Pop only if the head matches the expected call ID
local head = redis.call('LINDEX', queueKey, 0)
if head ~= expectedHead then
  return 0
end
redis.call('LPOP', queueKey)

redis.call('SET', agentKey, 'RESERVED', 'EX', ttl)
redis.call('SET', callAgentKey, agentExt)
return 1
`)

// AtomicReserveAgent — see domain.QueueManager for semantics.
func (q *QueueManager) AtomicReserveAgent(ctx context.Context, callID int64, agentExt string, ttl time.Duration) (bool, error) {
	head, err := q.rdb().LIndex(ctx, keyCallQueue(), 0).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return false, fmt.Errorf("redis: lindex: %w", err)
	}
	if head == "" {
		head = "0"
	}
	res, err := reserveScript.Run(ctx, q.rdb(),
		[]string{
			keyAgentState(agentExt),
			keyCallAgent(callID),
			keyCallQueue(),
		},
		int(ttl.Seconds()),
		agentExt,
		strconv.FormatInt(callID, 10),
		head,
	).Int()
	if err != nil {
		return false, fmt.Errorf("redis: reserve script: %w", err)
	}
	return res == 1, nil
}

// releaseScript: RESERVED → AVAILABLE; idempotent. Preserves BUSY and
// OFFLINE (so we never accidentally clear those).
var releaseScript = redis.NewScript(`
local agentKey = KEYS[1]
local cur = redis.call('GET', agentKey)
if cur == false then
  redis.call('SET', agentKey, 'AVAILABLE')
  return 1
end
if cur == 'RESERVED' then
  redis.call('SET', agentKey, 'AVAILABLE')
  return 1
end
return 0
`)

// ReleaseReservation moves an agent from RESERVED → AVAILABLE. If the
// agent is currently in BUSY/RINGING we leave it alone (those states
// are owned by the voice usecase, not the queue manager).
func (q *QueueManager) ReleaseReservation(ctx context.Context, agentExt string) error {
	_, err := releaseScript.Run(ctx, q.rdb(), []string{keyAgentState(agentExt)}).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return fmt.Errorf("redis: release script: %w", err)
	}
	return nil
}

// SetAgentState writes the agent status with optional TTL.
func (q *QueueManager) SetAgentState(ctx context.Context, agentExt string, status domain.AgentStatus, ttl time.Duration) error {
	if ttl > 0 {
		return q.rdb().Set(ctx, keyAgentState(agentExt), string(status), ttl).Err()
	}
	return q.rdb().Set(ctx, keyAgentState(agentExt), string(status), 0).Err()
}

// GetAgentState returns "" if no entry exists.
func (q *QueueManager) GetAgentState(ctx context.Context, agentExt string) (domain.AgentStatus, error) {
	v, err := q.rdb().Get(ctx, keyAgentState(agentExt)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", nil
		}
		return "", err
	}
	return domain.AgentStatus(v), nil
}

// SetAgentCurrentCall records the call the agent is currently serving.
// Pass 0 to clear.
func (q *QueueManager) SetAgentCurrentCall(ctx context.Context, agentExt string, callID int64) error {
	if callID == 0 {
		return q.rdb().Del(ctx, keyAgentCurrentCall(agentExt)).Err()
	}
	return q.rdb().Set(ctx, keyAgentCurrentCall(agentExt), strconv.FormatInt(callID, 10), 0).Err()
}

// ReserveIdempotency uses SET NX to atomically claim a key. If the key
// already exists, the cached payload is returned as `existing` and
// `hit=true`.
func (q *QueueManager) ReserveIdempotency(ctx context.Context, key, payload string, ttl time.Duration) (string, bool, error) {
	ok, err := q.rdb().SetNX(ctx, key, payload, ttl).Result()
	if err != nil {
		return "", false, fmt.Errorf("redis: setnx: %w", err)
	}
	if ok {
		return "", false, nil
	}
	// Hit: fetch existing payload.
	existing, err := q.rdb().Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", false, nil
		}
		return "", false, err
	}
	return existing, true, nil
}

// ============================================================
// Key helpers
// ============================================================

func keyCallQueue() string          { return "queue:call" }
func keyAgentState(ext string) string {
	return "agent:" + ext + ":state"
}
func keyAgentCurrentCall(ext string) string {
	return "agent:" + ext + ":current_call"
}
func keyCallAgent(callID int64) string {
	return "call:" + strconv.FormatInt(callID, 10) + ":agent"
}
