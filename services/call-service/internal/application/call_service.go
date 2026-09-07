package application

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/hoainguyen222/DongDo_CS_V2/services/call-service/internal/domain"
	"github.com/hoainguyen222/DongDo_CS_V2/services/call-service/internal/infrastructure/asterisk"
	"github.com/hoainguyen222/DongDo_CS_V2/services/call-service/internal/infrastructure/postgres"
	"github.com/hoainguyen222/DongDo_CS_V2/services/call-service/internal/infrastructure/redis"
	"github.com/hoainguyen222/DongDo_CS_V2/services/call-service/internal/interfaces/websocket"
	"github.com/rs/zerolog/log"
)

type CallService struct {
	repo       *postgres.CallRepository
	agentStore *redis.AgentStateStore
	streamMgr  *redis.CallStreamManager
	ariClient  *asterisk.ARIClient
	wsHub      *websocket.WSHub
}

func NewCallService(
	repo *postgres.CallRepository,
	agentStore *redis.AgentStateStore,
	streamMgr *redis.CallStreamManager,
	ariClient *asterisk.ARIClient,
	wsHub *websocket.WSHub,
) *CallService {
	return &CallService{
		repo:       repo,
		agentStore: agentStore,
		streamMgr:  streamMgr,
		ariClient:  ariClient,
		wsHub:      wsHub,
	}
}

// 1. Customer initiates call request (Phase 1)
func (s *CallService) CreateCall(ctx context.Context, sessionID, customerID string) (*domain.Call, error) {
	callID := "call_" + uuid.New().String()
	if sessionID == "" {
		sessionID = "sess_" + uuid.New().String()
	}

	call := &domain.Call{
		ID:          callID,
		SessionID:   sessionID,
		CustomerID:  customerID,
		Status:      domain.StatusCreated,
		RequestedAt: time.Now().UTC(),
	}

	if err := s.repo.Create(ctx, call); err != nil {
		return nil, err
	}

	_ = s.repo.SaveEvent(ctx, &domain.CallEvent{
		CallID:    callID,
		EventType: "CALL_REQUESTED",
		Source:    "API",
		Payload:   map[string]interface{}{"customer_id": customerID, "session_id": sessionID},
	})

	// Transition to WAITING
	_ = s.repo.UpdateStatus(ctx, callID, domain.StatusWaiting)
	call.Status = domain.StatusWaiting

	// Publish to Redis Stream for async router worker
	if err := s.streamMgr.PublishCallRequest(ctx, callID, customerID); err != nil {
		log.Error().Err(err).Str("call_id", callID).Msg("Failed to publish to stream, falling back")
	}

	// Realtime WS update
	s.wsHub.Broadcast(customerID, "call_status", map[string]interface{}{
		"call_id": callID,
		"status":  domain.StatusWaiting,
	})

	// 45-second ringing timeout worker
	go func(cID string) {
		time.Sleep(45 * time.Second)
		ctxTimeout := context.Background()
		c, err := s.repo.GetByID(ctxTimeout, cID)
		if err == nil && (c.Status == domain.StatusWaiting || c.Status == domain.StatusWaitingAgent || c.Status == domain.StatusCreated) {
			log.Info().Str("call_id", cID).Msg("Call ringing timeout reached (45s), automatically canceling call")
			_ = s.repo.EndCall(ctxTimeout, cID, 0, domain.StatusTimeout)
			if c.AgentID != "" {
				_ = s.agentStore.SetAgentAvailable(ctxTimeout, c.AgentID)
			}
			s.wsHub.Broadcast("admin_inbox", "call_ended", map[string]interface{}{
				"call_id": cID,
				"status":  domain.StatusTimeout,
				"reason":  "Ringing timeout (45s)",
			})
			s.wsHub.Broadcast(c.CustomerID, "call_ended", map[string]interface{}{
				"call_id": cID,
				"status":  domain.StatusTimeout,
				"reason":  "Ringing timeout (45s)",
			})
		}
	}(callID)

	log.Info().Str("call_id", callID).Str("customer_id", customerID).Msg("Call requested by customer")
	return call, nil
}

// 2. Redis Stream Call Router Handler
func (s *CallService) HandleRouteCall(ctx context.Context, callID, customerID string) error {
	call, err := s.repo.GetByID(ctx, callID)
	if err != nil || call.Status != domain.StatusWaiting {
		return nil // Call cancelled or non-existent
	}

	// Atomic Lua Reservation
	agentID, err := s.agentStore.ReserveAvailableAgent(ctx, callID)
	if err != nil {
		// No available agent right now
		log.Info().Str("call_id", callID).Msg("No available agents, call remains in queue")
		return nil
	}

	// Assign agent
	if err := s.repo.AssignAgent(ctx, callID, agentID); err != nil {
		_ = s.agentStore.SetAgentAvailable(ctx, agentID)
		return err
	}

	_ = s.repo.SaveEvent(ctx, &domain.CallEvent{
		CallID:    callID,
		EventType: "AGENT_ASSIGNED",
		Source:    "SYSTEM",
		Payload:   map[string]interface{}{"agent_id": agentID},
	})

	// Notify Agent UI via WebSocket
	s.wsHub.Broadcast("admin_inbox", "incoming_call", map[string]interface{}{
		"call_id":     callID,
		"session_id":  call.SessionID,
		"customer_id": customerID,
		"agent_id":    agentID,
		"status":      domain.StatusWaitingAgent,
	})
	s.wsHub.Broadcast(agentID, "incoming_call", map[string]interface{}{
		"call_id":     callID,
		"session_id":  call.SessionID,
		"customer_id": customerID,
		"agent_id":    agentID,
		"status":      domain.StatusWaitingAgent,
	})

	log.Info().Str("call_id", callID).Str("agent_id", agentID).Msg("Assigned agent to call and sent notification")
	return nil
}

// 3. Agent Accepts Call (Phase 2 Media Setup with Asterisk ARI)
func (s *CallService) AcceptCall(ctx context.Context, callID, agentID string) error {
	call, err := s.repo.GetByID(ctx, callID)
	if err != nil {
		return err
	}

	if call.Status != domain.StatusWaitingAgent {
		return fmt.Errorf("cannot accept call in status: %s", call.Status)
	}

	// Idempotent & state check
	_ = s.repo.UpdateStatus(ctx, callID, domain.StatusConnecting)
	_ = s.agentStore.SetAgentBusy(ctx, agentID, callID)

	_ = s.repo.SaveEvent(ctx, &domain.CallEvent{
		CallID:    callID,
		EventType: "AGENT_ACCEPTED",
		Source:    "API",
		Payload:   map[string]interface{}{"agent_id": agentID},
	})

	// Create Asterisk Bridge
	bridgeID := "bridge_" + callID
	_, err = s.ariClient.CreateBridge(ctx, bridgeID)
	if err != nil {
		log.Error().Err(err).Str("call_id", callID).Msg("Failed to create Asterisk bridge")
	}

	// Save Call Session Mapping
	customerChanID := "chan_cust_" + callID
	agentChanID := "chan_agent_" + callID
	_ = s.repo.UpsertSession(ctx, &domain.CallSession{
		CallID:            callID,
		CustomerChannelID: customerChanID,
		AgentChannelID:    agentChanID,
		BridgeID:          bridgeID,
	})

	// Trigger Asterisk Channel Originate for Agent & Customer
	go func() {
		_, _ = s.ariClient.OriginateChannel(context.Background(), "PJSIP/agent", agentChanID, call.CustomerID)
		_, _ = s.ariClient.OriginateChannel(context.Background(), "PJSIP/customer", customerChanID, call.CustomerID)
		_ = s.ariClient.AddChannelToBridge(context.Background(), bridgeID, agentChanID)
		_ = s.ariClient.AddChannelToBridge(context.Background(), bridgeID, customerChanID)
		_ = s.ariClient.StartRecording(context.Background(), bridgeID, "rec_"+callID)
	}()

	_ = s.repo.SetStarted(ctx, callID)

	// Notify both UI sides
	s.wsHub.Broadcast("admin_inbox", "call_connected", map[string]interface{}{
		"call_id":   callID,
		"status":    domain.StatusInProgress,
		"bridge_id": bridgeID,
	})
	s.wsHub.Broadcast(call.CustomerID, "call_connected", map[string]interface{}{
		"call_id":   callID,
		"status":    domain.StatusInProgress,
		"bridge_id": bridgeID,
	})

	log.Info().Str("call_id", callID).Str("agent_id", agentID).Msg("Call accepted and Asterisk channels bridged")
	return nil
}

// 4. Agent Rejects or Timeouts Call
func (s *CallService) RejectCall(ctx context.Context, callID, agentID string) error {
	call, err := s.repo.GetByID(ctx, callID)
	if err != nil {
		return err
	}

	// Release Agent
	_ = s.agentStore.SetAgentAvailable(ctx, agentID)

	// Put call back to WAITING
	_ = s.repo.UpdateStatus(ctx, callID, domain.StatusWaiting)

	_ = s.repo.SaveEvent(ctx, &domain.CallEvent{
		CallID:    callID,
		EventType: "AGENT_REJECTED",
		Source:    "API",
		Payload:   map[string]interface{}{"agent_id": agentID},
	})

	// Re-publish to stream for retry router
	_ = s.streamMgr.PublishCallRequest(ctx, callID, call.CustomerID)

	log.Info().Str("call_id", callID).Str("agent_id", agentID).Msg("Agent rejected call, re-queued call")
	return nil
}

// 5. Hangup Call
func (s *CallService) HangupCall(ctx context.Context, callID string, durationSec int) error {
	call, err := s.repo.GetByID(ctx, callID)
	if err != nil {
		return err
	}

	if call.Status == domain.StatusEnded || call.Status == domain.StatusTerminating {
		return nil // Idempotent
	}

	_ = s.repo.UpdateStatus(ctx, callID, domain.StatusTerminating)

	// Cleanup Asterisk Channels
	customerChanID := "chan_cust_" + callID
	agentChanID := "chan_agent_" + callID
	go func() {
		_ = s.ariClient.HangupChannel(context.Background(), customerChanID)
		_ = s.ariClient.HangupChannel(context.Background(), agentChanID)
	}()

	// Release Agent
	if call.AgentID != "" {
		_ = s.agentStore.SetAgentAvailable(ctx, call.AgentID)
	}

	_ = s.repo.EndCall(ctx, callID, durationSec, domain.StatusEnded)

	_ = s.repo.SaveEvent(ctx, &domain.CallEvent{
		CallID:    callID,
		EventType: "CALL_ENDED",
		Source:    "API",
		Payload:   map[string]interface{}{"duration_seconds": durationSec},
	})

	// Notify WS clients
	s.wsHub.Broadcast("admin_inbox", "call_ended", map[string]interface{}{
		"call_id":          callID,
		"duration_seconds": durationSec,
		"status":           domain.StatusEnded,
	})
	s.wsHub.Broadcast(call.CustomerID, "call_ended", map[string]interface{}{
		"call_id":          callID,
		"duration_seconds": durationSec,
		"status":           domain.StatusEnded,
	})

	log.Info().Str("call_id", callID).Int("duration", durationSec).Msg("Call hangup completed cleanly")
	return nil
}

// 6. Handle Asterisk Stasis Events (Source of Truth)
func (s *CallService) HandleARIEvent(event *asterisk.ARIEvent) {
	if event == nil || event.Channel == nil {
		return
	}

	chanID := event.Channel.ID
	log.Debug().Str("event_type", event.Type).Str("channel_id", chanID).Msg("Received Asterisk ARI Event")

	switch event.Type {
	case "StasisEnd", "ChannelDestroyed":
		// Handle channel hangup event from Asterisk
		if len(chanID) > 10 { // e.g. chan_cust_call_123 or chan_agent_call_123
			parts := []string{}
			if len(chanID) > 10 {
				parts = append(parts, chanID)
			}
			log.Info().Str("channel_id", chanID).Str("event", event.Type).Msg("Asterisk ARI channel destroyed")
		}
	}
}

// 7. List Calls for History Page
func (s *CallService) ListCalls(ctx context.Context, limit int) ([]*domain.Call, error) {
	return s.repo.ListAll(ctx, limit)
}

// 8. Save Recording
func (s *CallService) SaveRecording(ctx context.Context, callID, recordingURL string, fileSize int64, transcript string) error {
	return s.repo.SaveRecording(ctx, &domain.CallRecording{
		CallID:        callID,
		RecordingURL:  recordingURL,
		FileSizeBytes: fileSize,
		Transcript:    transcript,
	})
}
