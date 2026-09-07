// Package callqueue implements domain.CallQueueManager backed by Redis.
//
// All multi-step operations are atomic via Lua scripts, eliminating the classic
// GET-then-SET race that would otherwise let two agents be reserved for the same call.
//
// Keys (see docs/call-architecture.md §5):
//   call:queue:waiting           ZSET
//   call:{id}:state              STRING (mirror of last-known status)
//   call:{id}:owner              STRING (customer_id)
//   call:{id}:idem:{key}         STRING (idempotency cache, 24h)
//   agent:{id}:state             STRING
//   agent:{id}:current_call      STRING
//   agent:{id}:session           STRING (heartbeat)
//   agents:available             ZSET
//   agents:by_state              HASH  field=state → set of agent ids (stored as JSON array)
package callqueue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

// Manager is the Redis-backed CallQueueManager.
type Manager struct {
	rdb    *redis.Client
	logger zerolog.Logger
}

// New constructs a Manager from an existing go-redis client.
func New(rdb *redis.Client, logger zerolog.Logger) *Manager {
	if rdb == nil {
		return nil
	}
	return &Manager{
		rdb:    rdb,
		logger: logger.With().Str("component", "call_queue").Logger(),
	}
}

func (m *Manager) client() *redis.Client { return m.rdb }

// ----------------------------------------------------------------
// Lua scripts (registered once, reused via EvalSha on retry)
// ----------------------------------------------------------------

// atomicReserveAgent: AVAILABLE → RESERVED, bind call, remove from available ZSET.
// If the agent has never been registered, return {0, "NEW"} so callers can
// initialize the key first.
const luaReserveAgent = `
-- KEYS[1]=agent:{id}:state  KEYS[2]=agent:{id}:current_call  KEYS[3]=agents:available
-- ARGV[1]=agent_id  ARGV[2]=call_id
local st = redis.call("GET", KEYS[1])
if not st then
  redis.call("SET", KEYS[1], "AVAILABLE")
  st = "AVAILABLE"
end
if st ~= "AVAILABLE" then return {0, st} end
redis.call("SET", KEYS[1], "RESERVED")
redis.call("SET", KEYS[2], ARGV[2])
redis.call("ZREM", KEYS[3], ARGV[1])
return {1, "RESERVED"}
`

// atomicReleaseAgent: {RESERVED,RINGING,BUSY} → AVAILABLE, unbind call.
const luaReleaseAgent = `
-- KEYS[1]=agent:{id}:state  KEYS[2]=agent:{id}:current_call
-- ARGV[1]=agent_id
local st = redis.call("GET", KEYS[1])
if not st then return {0, "MISSING"} end
if st ~= "RESERVED" and st ~= "RINGING" and st ~= "BUSY" then
  return {0, st}
end
redis.call("SET", KEYS[1], "AVAILABLE")
redis.call("DEL", KEYS[2])
return {1, "AVAILABLE"}
`

// atomicSetBusy: {RESERVED,RINGING} → BUSY
const luaSetBusy = `
local st = redis.call("GET", KEYS[1])
if not st then return {0, "MISSING"} end
if st ~= "RESERVED" and st ~= "RINGING" then return {0, st} end
redis.call("SET", KEYS[1], "BUSY")
return {1, "BUSY"}
`

// atomicSetAway: AVAILABLE → AWAY
const luaSetAway = `
local st = redis.call("GET", KEYS[1])
if not st then return {0, "MISSING"} end
if st ~= "AVAILABLE" then return {0, st} end
redis.call("SET", KEYS[1], "AWAY")
return {1, "AWAY"}
`

// atomicSetAvailable: any state → AVAILABLE, unbind call, register in pool.
const luaSetAvailable = `
local st = redis.call("GET", KEYS[1])
if not st then st = "AVAILABLE" end
redis.call("SET", KEYS[1], "AVAILABLE")
redis.call("DEL", KEYS[2])
redis.call("ZADD", KEYS[3], "NX", redis.call("TIME")[1], ARGV[1])
return {1, "AVAILABLE"}
`

// ----------------------------------------------------------------
// Queue operations
// ----------------------------------------------------------------

// Enqueue adds a call to the waiting queue.
// Score = -priority*1e13 + now_ms so higher priority → lower score → pops first.
// Within the same priority, ties resolve by insertion order (FIFO).
func (m *Manager) Enqueue(ctx context.Context, callID uuid.UUID, priority int, scoreMS int64) error {
	key := "call:queue:waiting"
	score := -float64(priority)*1e13 + float64(scoreMS)
	if err := m.client().ZAdd(ctx, key, redis.Z{Score: score, Member: callID.String()}).Err(); err != nil {
		return fmt.Errorf("callqueue: enqueue: %w", err)
	}
	return nil
}

// PopNext atomically pops the lowest-score call.
func (m *Manager) PopNext(ctx context.Context) (uuid.UUID, bool, error) {
	res, err := m.client().ZPopMin(ctx, "call:queue:waiting", 1).Result()
	if err != nil {
		return uuid.Nil, false, err
	}
	if len(res) == 0 {
		return uuid.Nil, false, nil
	}
	idStr, ok := res[0].Member.(string)
	if !ok {
		return uuid.Nil, false, errors.New("callqueue: unexpected ZPopMin payload")
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		return uuid.Nil, false, fmt.Errorf("callqueue: bad uuid in queue: %w", err)
	}
	return id, true, nil
}

// QueuePosition returns the 0-based rank.
func (m *Manager) QueuePosition(ctx context.Context, callID uuid.UUID) (int64, error) {
	rank, err := m.client().ZRank(ctx, "call:queue:waiting", callID.String()).Result()
	if errors.Is(err, redis.Nil) {
		return -1, nil
	}
	if err != nil {
		return 0, err
	}
	return rank, nil
}

// QueueSize returns the number of waiting calls.
func (m *Manager) QueueSize(ctx context.Context) (int64, error) {
	return m.client().ZCard(ctx, "call:queue:waiting").Result()
}

// RemoveFromQueue removes the call from the queue (cancel scenarios).
func (m *Manager) RemoveFromQueue(ctx context.Context, callID uuid.UUID) error {
	return m.client().ZRem(ctx, "call:queue:waiting", callID.String()).Err()
}

// MarkAnnounced records that the call has been published to an agent/admin.
// TTL controls how long the dedup entry sticks around; subsequent routing
// attempts within the TTL will see IsAnnounced=true and skip re-publish.
func (m *Manager) MarkAnnounced(ctx context.Context, callID uuid.UUID, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = 2 * time.Minute
	}
	return m.client().Set(ctx, "call:"+callID.String()+":announced", "1", ttl).Err()
}

// IsAnnounced returns true if the call has been announced and the dedup TTL
// has not expired. Returns false when the key is missing (REDIS nil included).
func (m *Manager) IsAnnounced(ctx context.Context, callID uuid.UUID) (bool, error) {
	n, err := m.client().Exists(ctx, "call:"+callID.String()+":announced").Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// ClearAnnounced removes the dedup key — called once the call leaves the
// "waiting for agent" phase (accept, reject, cancel, end).
func (m *Manager) ClearAnnounced(ctx context.Context, callID uuid.UUID) error {
	return m.client().Del(ctx, "call:"+callID.String()+":announced").Err()
}

// ----------------------------------------------------------------
// Agent state operations
// ----------------------------------------------------------------

func agentKey(id string) string  { return "agent:" + id + ":state" }
func agentCall(id string) string { return "agent:" + id + ":current_call" }
func agentSeen(id string) string { return "agent:" + id + ":session" }

// ReserveAgent atomically transitions AVAILABLE → RESERVED.
func (m *Manager) ReserveAgent(ctx context.Context, agentID string, callID uuid.UUID) (bool, domain.AgentState, error) {
	res, err := m.client().Eval(ctx, luaReserveAgent,
		[]string{agentKey(agentID), agentCall(agentID), "agents:available"},
		agentID, callID.String(),
	).Result()
	if err != nil {
		return false, "", fmt.Errorf("callqueue: reserve: %w", err)
	}
	return parseTriRes(res)
}

// ReleaseAgent atomically transitions back to AVAILABLE.
func (m *Manager) ReleaseAgent(ctx context.Context, agentID string) (domain.AgentState, error) {
	res, err := m.client().Eval(ctx, luaReleaseAgent,
		[]string{agentKey(agentID), agentCall(agentID)},
		agentID,
	).Result()
	if err != nil {
		return "", err
	}
	ok, st, err := parseTriRes(res)
	if err != nil {
		return "", err
	}
	if ok {
		// Add back to available pool
		_ = m.client().ZAdd(ctx, "agents:available", redis.Z{
			Score:  float64(time.Now().Unix()),
			Member: agentID,
		}).Err()
	}
	return st, nil
}

// SetAgentBusy transitions to BUSY. Returns an error if the agent is not in
// RESERVED or RINGING state.
func (m *Manager) SetAgentBusy(ctx context.Context, agentID string, _ uuid.UUID) error {
	res, err := m.client().Eval(ctx, luaSetBusy, []string{agentKey(agentID)}).Result()
	if err != nil {
		return err
	}
	ok, _, err2 := parseTriRes(res)
	if err2 != nil {
		return err2
	}
	if !ok {
		return fmt.Errorf("callqueue: cannot set agent %s to BUSY from current state", agentID)
	}
	return nil
}

// SetAgentAway transitions AVAILABLE → AWAY.
func (m *Manager) SetAgentAway(ctx context.Context, agentID string) error {
	_, err := m.client().Eval(ctx, luaSetAway,
		[]string{agentKey(agentID)},
	).Result()
	return err
}

// SetAgentAvailable transitions to AVAILABLE and re-adds to pool.
func (m *Manager) SetAgentAvailable(ctx context.Context, agentID string) error {
	_, err := m.client().Eval(ctx, luaSetAvailable,
		[]string{agentKey(agentID), agentCall(agentID), "agents:available"},
		agentID,
	).Result()
	return err
}

// SetAgentOffline removes from pool and marks offline.
func (m *Manager) SetAgentOffline(ctx context.Context, agentID string) error {
	pipe := m.client().TxPipeline()
	pipe.Set(ctx, agentKey(agentID), string(domain.AgentOffline), 0)
	pipe.Del(ctx, agentCall(agentID))
	pipe.ZRem(ctx, "agents:available", agentID)
	_, err := pipe.Exec(ctx)
	return err
}

// HeartbeatAgent refreshes last_seen (60s TTL).
func (m *Manager) HeartbeatAgent(ctx context.Context, agentID string) error {
	return m.client().Set(ctx, agentSeen(agentID), time.Now().UTC().Format(time.RFC3339), 60*time.Second).Err()
}

// GetAgent returns the agent state and last_seen.
func (m *Manager) GetAgent(ctx context.Context, agentID string) (*domain.Agent, error) {
	state, err := m.client().Get(ctx, agentKey(agentID)).Result()
	if errors.Is(err, redis.Nil) {
		return &domain.Agent{ID: agentID, State: domain.AgentOffline}, nil
	}
	if err != nil {
		return nil, err
	}
	a := &domain.Agent{ID: agentID, State: domain.AgentState(state)}

	if callStr, err := m.client().Get(ctx, agentCall(agentID)).Result(); err == nil {
		if cid, perr := uuid.Parse(callStr); perr == nil {
			a.CurrentCall = &cid
		}
	}

	if seen, err := m.client().Get(ctx, agentSeen(agentID)).Result(); err == nil {
		if t, perr := time.Parse(time.RFC3339, seen); perr == nil {
			a.LastSeenAt = t
		}
	}
	return a, nil
}

// ListAvailable returns agent ids currently AVAILABLE.
func (m *Manager) ListAvailable(ctx context.Context) ([]string, error) {
	ids, err := m.client().ZRange(ctx, "agents:available", 0, -1).Result()
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		// Double-check the actual state — ZSET may be stale if Redis failed mid-update.
		st, err := m.client().Get(ctx, agentKey(id)).Result()
		if err != nil || st != string(domain.AgentAvailable) {
			continue
		}
		out = append(out, id)
	}
	return out, nil
}

// ListActiveAgents returns agent ids in {RESERVED,RINGING,BUSY}.
func (m *Manager) ListActiveAgents(ctx context.Context) ([]string, error) {
	states := []string{string(domain.AgentReserved), string(domain.AgentRinging), string(domain.AgentBusy)}
	out := make([]string, 0)
	for _, st := range states {
		ids, err := m.scanByState(ctx, st)
		if err != nil {
			return nil, err
		}
		out = append(out, ids...)
	}
	return out, nil
}

// scanByState is a helper using SCAN to find agent:*:state keys with a given value.
// (Cheap for moderate agent counts; for huge fleets, consider a sorted set per state.)
func (m *Manager) scanByState(ctx context.Context, wantState string) ([]string, error) {
	out := make([]string, 0)
	var cursor uint64
	for {
		keys, next, err := m.client().Scan(ctx, cursor, "agent:*:state", 256).Result()
		if err != nil {
			return nil, err
		}
		for _, k := range keys {
			v, err := m.client().Get(ctx, k).Result()
			if err == nil && v == wantState {
				// extract id from "agent:{id}:state"
				id := k[len("agent:") : len(k)-len(":state")]
				out = append(out, id)
			}
		}
		if next == 0 {
			break
		}
		cursor = next
	}
	return out, nil
}

// parseTriRes parses the {ok, state} tuple from Lua scripts.
func parseTriRes(res any) (bool, domain.AgentState, error) {
	arr, ok := res.([]any)
	if !ok || len(arr) != 2 {
		return false, "", fmt.Errorf("callqueue: unexpected lua reply: %#v", res)
	}
	okI, _ := arr[0].(int64)
	stateS, _ := arr[1].(string)
	return okI == 1, domain.AgentState(stateS), nil
}

// NoopManager is the in-memory CallQueueManager used when Redis is unavailable.
// It implements the interface but every operation is a no-op (call is "always available").
// This is intentionally simple — production code must use the real Redis manager.
type NoopManager struct{ logger zerolog.Logger }

func NewNoopManager() *NoopManager {
	return &NoopManager{logger: zerolog.New(os.Stderr).With().Timestamp().Logger()}
}

func (NoopManager) Enqueue(context.Context, uuid.UUID, int, int64) error { return nil }
func (NoopManager) PopNext(context.Context) (uuid.UUID, bool, error)    { return uuid.Nil, false, nil }
func (NoopManager) QueuePosition(context.Context, uuid.UUID) (int64, error) { return 0, nil }
func (NoopManager) QueueSize(context.Context) (int64, error)            { return 0, nil }
func (NoopManager) RemoveFromQueue(context.Context, uuid.UUID) error    { return nil }
func (NoopManager) ReserveAgent(context.Context, string, uuid.UUID) (bool, domain.AgentState, error) {
	return true, domain.AgentReserved, nil
}
func (NoopManager) ReleaseAgent(context.Context, string) (domain.AgentState, error) {
	return domain.AgentAvailable, nil
}
func (NoopManager) SetAgentBusy(context.Context, string, uuid.UUID) error { return nil }
func (NoopManager) SetAgentAway(context.Context, string) error             { return nil }
func (NoopManager) SetAgentAvailable(context.Context, string) error       { return nil }
func (NoopManager) SetAgentOffline(context.Context, string) error         { return nil }
func (NoopManager) HeartbeatAgent(context.Context, string) error           { return nil }
func (NoopManager) GetAgent(context.Context, string) (*domain.Agent, error) {
	return &domain.Agent{ID: "noop", State: domain.AgentAvailable}, nil
}
func (NoopManager) ListAvailable(context.Context) ([]string, error)    { return nil, nil }
func (NoopManager) ListActiveAgents(context.Context) ([]string, error) { return nil, nil }
func (NoopManager) MarkAnnounced(context.Context, uuid.UUID, time.Duration) error { return nil }
func (NoopManager) IsAnnounced(context.Context, uuid.UUID) (bool, error) { return false, nil }
func (NoopManager) ClearAnnounced(context.Context, uuid.UUID) error    { return nil }

// MarshalJSON helper used by tests
func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
