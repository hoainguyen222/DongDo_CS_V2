package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/hoainguyen222/DongDo_CS_V2/services/call-service/internal/domain"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

type AgentStateStore struct {
	client *redis.Client
}

func NewAgentStateStore(client *redis.Client) *AgentStateStore {
	return &AgentStateStore{client: client}
}

// Atomic Reservation Lua Script
// Checks if agents:available set has an agent.
// Picks the agent, sets agent:{id}:state to RESERVED with current_call = call_id,
// and removes agent from agents:available set atomically.
const reserveAgentLua = `
local call_id = ARGV[1]
local available_agent = redis.call('SPOP', 'agents:available')
if not available_agent then
    return nil
end

local state_key = 'agent:' .. available_agent .. ':state'
redis.call('HSET', state_key, 'status', 'RESERVED', 'current_call', call_id, 'updated_at', ARGV[2])
return available_agent
`

func (s *AgentStateStore) ReserveAvailableAgent(ctx context.Context, callID string) (string, error) {
	nowStr := time.Now().UTC().Format(time.RFC3339)
	res, err := s.client.Eval(ctx, reserveAgentLua, []string{}, callID, nowStr).Result()
	if err == redis.Nil || res == nil {
		return "", domain.ErrAgentNotAvailable
	}
	if err != nil {
		log.Error().Err(err).Str("call_id", callID).Msg("Lua script reserve agent failed")
		return "", err
	}

	agentID, ok := res.(string)
	if !ok {
		return "", fmt.Errorf("unexpected lua response: %v", res)
	}

	log.Info().Str("call_id", callID).Str("agent_id", agentID).Msg("Atomically reserved agent for call")
	return agentID, nil
}

func (s *AgentStateStore) SetAgentAvailable(ctx context.Context, agentID string) error {
	stateKey := fmt.Sprintf("agent:%s:state", agentID)
	pipe := s.client.TxPipeline()
	pipe.HSet(ctx, stateKey, "status", string(domain.AgentAvailable), "current_call", "", "updated_at", time.Now().UTC().Format(time.RFC3339))
	pipe.SAdd(ctx, "agents:available", agentID)
	_, err := pipe.Exec(ctx)
	if err != nil {
		log.Error().Err(err).Str("agent_id", agentID).Msg("Failed to set agent available")
		return err
	}
	return nil
}

func (s *AgentStateStore) SetAgentBusy(ctx context.Context, agentID, callID string) error {
	stateKey := fmt.Sprintf("agent:%s:state", agentID)
	pipe := s.client.TxPipeline()
	pipe.HSet(ctx, stateKey, "status", string(domain.AgentBusy), "current_call", callID, "updated_at", time.Now().UTC().Format(time.RFC3339))
	pipe.SRem(ctx, "agents:available", agentID)
	_, err := pipe.Exec(ctx)
	return err
}

func (s *AgentStateStore) ReleaseAgent(ctx context.Context, agentID string) error {
	return s.SetAgentAvailable(ctx, agentID)
}
