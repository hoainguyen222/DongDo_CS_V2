// ari_service.go — High-level ARI orchestration for the DongDo WebRTC call flow.
//
// This service sits on top of ARIClient and owns the in-flight call
// state that flows through the Stasis application.  When a SIP channel
// enters the configured app we get a StasisStart event; we correlate it
// with the database record using the `DD_CALL_ID` / `DD_SESSION_ID`
// channel variables that the caller (or our originate helper) sets, and
// surface the lifecycle through the EventBus so admin dashboards stay in
// sync with Asterisk's reality.
//
// The service is intentionally minimal — it does not own call records
// (those live in PostgreSQL via VoiceCallRepository) and it does not own
// transport (the browser uses sip.js to talk WebRTC directly to Asterisk).
package asterisk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
)

// ARIService is the high-level façade that handlers / use-cases call to
// interact with the Stasis app.
type ARIService struct {
	client *ARIClient
	logger zerolog.Logger

	// sessionCallIndex: sessionID → call descriptor we built from StasisStart.
	mu        sync.RWMutex
	active    map[string]*stasisCall
	channels  map[string]string // channelID → sessionID (for StasisEnd lookup)

	enabled atomic.Bool

	// Optional hooks: when a StasisStart for a "guest" channel arrives
	// we can notify an external listener (the HTTP layer / use-case).
	// These are set by main.go at wire-up time.
	OnGuestRinging func(callID int64, sessionID, callerID, phone string)
	OnCallActive   func(callID int64, sessionID, agentExt string)
	OnCallEnded    func(callID int64, sessionID, cause string)
}

// stasisCall tracks the channels belonging to one DB call record while
// it lives inside our Stasis app.
type stasisCall struct {
	SessionID  string
	CallID     int64
	GuestChan  string
	AgentChan  string
	BridgeID   string
	StartedAt  time.Time
	AgentExt   string
}

// NewARIService constructs the service around an already-connected (or
// not-yet-connected) ARI client.
func NewARIService(client *ARIClient) *ARIService {
	s := &ARIService{
		client:  client,
		logger:  newLogger().With().Str("component", "ari_service").Logger(),
		active:   map[string]*stasisCall{},
		channels: map[string]string{},
	}
	// Register the StasisStart / StasisEnd handlers before Connect().
	client.On("StasisStart", s.onStasisStart)
	client.On("StasisEnd", s.onStasisEnd)
	client.On("ChannelDestroyed", s.onChannelDestroyed)
	client.On("ChannelHangupRequest", s.onHangupRequest)
	return s
}

// Start connects the underlying client (which will auto-reconnect).
func (s *ARIService) Start(ctx context.Context) error {
	s.enabled.Store(true)
	return s.client.Connect(ctx)
}

// Stop terminates the service.
func (s *ARIService) Stop() {
	s.enabled.Store(false)
	s.client.Stop()
}

// Enabled returns whether the service is wired up.
func (s *ARIService) Enabled() bool { return s.enabled.Load() }

// Connected returns whether the underlying WebSocket is currently up.
func (s *ARIService) Connected() bool { return s.client.IsConnected() }

// IsEnabled reports whether ARI was configured (so callers can branch on
// "no ARI" vs "ARI but disconnected").
func (s *ARIService) IsEnabled() bool { return s != nil && s.enabled.Load() }

// ============================================================================
// Stasis event handlers
// ============================================================================

// onStasisStart is called whenever a channel enters our Stasis app.
//
// Two scenarios:
//   1. The channel is the inbound guest leg.  We have DD_CALL_ID /
//      DD_SESSION_ID set in channel vars from the dialplan / originate.
//   2. The channel is the agent leg we originated via AcceptCall — its
//      `DD_LEG` variable is "agent".
func (s *ARIService) onStasisStart(ev ARIEvent) {
	var payload struct {
		Channel     Channel `json:"channel"`
		Replace     Channel `json:"replace"`
		AppArgs     string  `json:"args"`
		Channelvars map[string]string
	}
	// The channelvars live inside channel.channelvars on the wire.
	if err := json.Unmarshal(ev.Raw, &payload); err != nil {
		s.logger.Warn().Err(err).Str("event", ev.Type).Msg("ari: bad payload")
		return
	}
	ch := payload.Channel
	if ch.ID == "" {
		ch = payload.Replace
	}
	vars := ch.Channelvars
	if vars == nil {
		vars = map[string]string{}
	}

	leg := vars["DD_LEG"]
	callIDStr := vars["DD_CALL_ID"]
	sessionID := vars["DD_SESSION_ID"]
	callerID := ch.Caller["number"]
	if callerID == "" {
		callerID = ch.Dialplan["caller_id_num"]
	}

	s.logger.Info().
		Str("channel_id", ch.ID).
		Str("leg", leg).
		Str("caller", callerID).
		Str("session", sessionID).
		Str("call_id", callIDStr).
		Msg("ari: stasis start")

	if leg == "agent" {
		s.handleAgentStasis(ch, vars)
		return
	}
	// Default: treat as the guest leg.
	s.handleGuestStasis(ch, vars, callerID)
}

// handleGuestStasis records the guest channel and notifies the listener.
func (s *ARIService) handleGuestStasis(ch Channel, vars map[string]string, callerID string) {
	sessionID := vars["DD_SESSION_ID"]
	if sessionID == "" {
		sessionID = ch.ID // fallback: use channel ID
	}
	callIDStr := vars["DD_CALL_ID"]
	var callID int64
	if callIDStr != "" {
		// ignore parse errors silently
		fmt.Sscanf(callIDStr, "%d", &callID)
	}

	s.mu.Lock()
	sc, ok := s.active[sessionID]
	if !ok {
		sc = &stasisCall{
			SessionID: sessionID,
			CallID:    callID,
			StartedAt: time.Now(),
		}
		s.active[sessionID] = sc
	}
	sc.GuestChan = ch.ID
	s.channels[ch.ID] = sessionID
	s.mu.Unlock()

	if s.OnGuestRinging != nil {
		s.OnGuestRinging(callID, sessionID, vars["DD_CALLER_NAME"], callerID)
	}
}

// handleAgentStasis is invoked when the agent leg we originated picks up.
// We answer the channel and bridge it together with the guest channel.
func (s *ARIService) handleAgentStasis(ch Channel, vars map[string]string) {
	sessionID := vars["DD_SESSION_ID"]
	agentExt := vars["DD_AGENT_EXT"]

	s.mu.Lock()
	sc, ok := s.active[sessionID]
	if !ok {
		// Should not happen — the originating AcceptCall always
		// populates active first.
		s.mu.Unlock()
		s.logger.Warn().Str("session", sessionID).Msg("ari: agent stasis for unknown session")
		return
	}
	sc.AgentChan = ch.ID
	sc.AgentExt = agentExt
	s.channels[ch.ID] = sessionID
	bridgeID := sc.BridgeID
	s.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.client.Answer(ctx, ch.ID); err != nil {
		s.logger.Warn().Err(err).Str("channel_id", ch.ID).Msg("ari: answer agent failed")
	}

	// Create a holding bridge on first agent join.
	if bridgeID == "" {
		br, err := s.client.CreateBridge(ctx, "bridge-"+sessionID)
		if err != nil {
			s.logger.Error().Err(err).Msg("ari: create bridge failed")
			return
		}
		bridgeID = br.ID
		s.mu.Lock()
		sc.BridgeID = bridgeID
		s.mu.Unlock()
	}

	// Add both channels to the bridge.
	if sc.GuestChan != "" {
		if err := s.client.AddChannelToBridge(ctx, bridgeID, sc.GuestChan); err != nil {
			s.logger.Warn().Err(err).Str("channel", sc.GuestChan).Msg("ari: add guest to bridge failed")
		}
	}
	if err := s.client.AddChannelToBridge(ctx, bridgeID, ch.ID); err != nil {
		s.logger.Warn().Err(err).Str("channel", ch.ID).Msg("ari: add agent to bridge failed")
	}

	if s.OnCallActive != nil {
		s.OnCallActive(sc.CallID, sessionID, agentExt)
	}
}

// onStasisEnd fires when a channel leaves our Stasis app — either because
// it hung up, was hung up, or was told to continue() out.
func (s *ARIService) onStasisEnd(ev ARIEvent) {
	var p struct {
		Channel Channel `json:"channel"`
		Cause   string  `json:"cause"`
	}
	if err := json.Unmarshal(ev.Raw, &p); err != nil {
		return
	}
	s.cleanupChannel(p.Channel.ID, "stasis_end:"+p.Cause)
}

// onChannelDestroyed covers channels that never entered Stasis (e.g. the
// agent leg fails to register).  We still want to clean our tracking.
func (s *ARIService) onChannelDestroyed(ev ARIEvent) {
	var p struct {
		Channel Channel `json:"channel"`
		Cause   string  `json:"cause"`
	}
	if err := json.Unmarshal(ev.Raw, &p); err != nil {
		return
	}
	s.cleanupChannel(p.Channel.ID, "destroyed:"+p.Cause)
}

// onHangupRequest catches channels whose owner requested hangup.
func (s *ARIService) onHangupRequest(ev ARIEvent) {
	var p struct {
		Channel Channel `json:"channel"`
		Cause   string  `json:"cause"`
	}
	if err := json.Unmarshal(ev.Raw, &p); err != nil {
		return
	}
	s.cleanupChannel(p.Channel.ID, "hangup:"+p.Cause)
}

// cleanupChannel drops tracking for the channel and fires OnCallEnded
// when the last channel of a session leaves.
func (s *ARIService) cleanupChannel(channelID, cause string) {
	s.mu.Lock()
	sessionID, ok := s.channels[channelID]
	if !ok {
		s.mu.Unlock()
		return
	}
	delete(s.channels, channelID)
	sc, ok := s.active[sessionID]
	if !ok {
		s.mu.Unlock()
		return
	}
	if sc.GuestChan == channelID {
		sc.GuestChan = ""
	}
	if sc.AgentChan == channelID {
		sc.AgentChan = ""
	}
	if sc.GuestChan == "" && sc.AgentChan == "" {
		delete(s.active, sessionID)
		bridgeID := sc.BridgeID
		s.mu.Unlock()
		if bridgeID != "" {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			_ = s.client.DestroyBridge(ctx, bridgeID)
			cancel()
		}
		if s.OnCallEnded != nil {
			s.OnCallEnded(sc.CallID, sessionID, cause)
		}
		return
	}
	s.mu.Unlock()
}

// ============================================================================
// Outbound call-control API (called from HTTP handlers / use-case)
// ============================================================================

// OriginateGuestCall brings a guest channel into our Stasis app, dialing
// the supplied SIP endpoint.  Used when the guest wants to call a PSTN
// number through Asterisk (the SIP endpoint is the trunk).
func (s *ARIService) OriginateGuestCall(ctx context.Context, p OriginateGuestParams) (*Channel, error) {
	if !s.client.IsConnected() {
		return nil, errors.New("ari: not connected")
	}
	ch, err := s.client.Originate(ctx, OriginateParams{
		Endpoint: p.Endpoint,
		AppArgs:  fmt.Sprintf("%d,%s", p.CallID, p.SessionID),
		CallerID: p.CallerID,
		Timeout:  p.TimeoutSec,
		ChannelVars: map[string]string{
			"DD_CALL_ID":     fmt.Sprintf("%d", p.CallID),
			"DD_SESSION_ID":  p.SessionID,
			"DD_LEG":         "guest",
			"DD_CALLER_NAME": p.CallerName,
		},
	})
	if err != nil {
		return nil, err
	}
	// Answer immediately so dialplan variables propagate + the agent
	// can hear the ringtone on the other side.
	if err := s.client.Answer(ctx, ch.ID); err != nil {
		s.logger.Warn().Err(err).Msg("ari: failed to auto-answer guest channel")
	}
	return ch, nil
}

// OriginateGuestParams configures a new guest-side originate.
type OriginateGuestParams struct {
	CallID     int64
	SessionID  string
	Endpoint   string // e.g. "PJSIP/trunk-pstn/84987654321"
	CallerID   string
	CallerName string
	TimeoutSec int
}

// AcceptCall answers the incoming guest channel and originates a new
// agent-side channel, then bridges them.
func (s *ARIService) AcceptCall(ctx context.Context, sessionID, agentExt string) error {
	if !s.client.IsConnected() {
		return errors.New("ari: not connected")
	}
	s.mu.Lock()
	sc, ok := s.active[sessionID]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("ari: no active call for session %q", sessionID)
	}
	if sc.AgentChan != "" {
		s.mu.Unlock()
		return errors.New("ari: call already accepted")
	}
	guestChan := sc.GuestChan
	s.mu.Unlock()

	// 1. Pre-create the bridge so the agent leg has somewhere to land.
	br, err := s.client.CreateBridge(ctx, "bridge-"+sessionID)
	if err != nil {
		return fmt.Errorf("ari: create bridge: %w", err)
	}
	s.mu.Lock()
	sc.BridgeID = br.ID
	s.mu.Unlock()

	// 2. Add the existing guest channel.
	if guestChan != "" {
		if err := s.client.AddChannelToBridge(ctx, br.ID, guestChan); err != nil {
			s.logger.Warn().Err(err).Msg("ari: add guest to bridge failed")
		}
	}

	// 3. Originate the agent leg.  We set DD_LEG=agent + the session id
	//    so handleAgentStasis can correlate and bridge.
	ch, err := s.client.Originate(ctx, OriginateParams{
		Endpoint: "PJSIP/" + agentExt,
		AppArgs:  sessionID,
		CallerID: fmt.Sprintf("\"Agent %s\" <%s>", agentExt, agentExt),
		Timeout:  60,
		ChannelVars: map[string]string{
			"DD_LEG":        "agent",
			"DD_SESSION_ID": sessionID,
			"DD_AGENT_EXT":  agentExt,
		},
	})
	if err != nil {
		return fmt.Errorf("ari: originate agent: %w", err)
	}

	// 4. Pre-answer the agent channel so it's already up when it lands
	//    in Stasis (avoid double-answer race in handleAgentStasis).
	if err := s.client.Answer(ctx, ch.ID); err != nil {
		s.logger.Warn().Err(err).Msg("ari: pre-answer agent failed")
	}

	return nil
}

// HangupCall terminates both legs (if any) of a session.
func (s *ARIService) HangupCall(ctx context.Context, sessionID string) error {
	s.mu.RLock()
	sc, ok := s.active[sessionID]
	s.mu.RUnlock()
	if !ok {
		return nil // already gone
	}
	if sc.GuestChan != "" {
		_ = s.client.Hangup(ctx, sc.GuestChan)
	}
	if sc.AgentChan != "" {
		_ = s.client.Hangup(ctx, sc.AgentChan)
	}
	return nil
}

// ============================================================
// AsteriskGateway interface implementation
// ============================================================
//
// ARIService satisfies the domain.AsteriskGateway interface so the
// HTTP / use-case layers can talk to Asterisk without importing the
// infra package directly. See internal/domain/call_contracts.go.

// Wrap our service so we satisfy the domain.AsteriskGateway interface
// through composition (not by editing ARIService directly — the ARI
// methods take sessionID, not callID).
type ariGatewayAdapter struct {
	svc            *ARIService
	resolveSession func(ctx context.Context, callID int64) (string, error)
}

// NewARIGatewayAdapter wraps an ARIService into a domain.AsteriskGateway.
// resolveSession is used to translate (callID) → (sessionID) since the
// ARI service is keyed by sessionID internally.
func NewARIGatewayAdapter(svc *ARIService, resolveSession func(ctx context.Context, callID int64) (string, error)) *ariGatewayAdapter {
	return &ariGatewayAdapter{svc: svc, resolveSession: resolveSession}
}

func (a *ariGatewayAdapter) Enabled() bool    { return a.svc != nil && a.svc.IsEnabled() }
func (a *ariGatewayAdapter) Connected() bool { return a.svc != nil && a.svc.Connected() }

func (a *ariGatewayAdapter) OriginateGuestCall(ctx context.Context, callID int64, sessionID, endpoint string) error {
	if a.svc == nil {
		return errors.New("ari gateway: not configured")
	}
	_, err := a.svc.OriginateGuestCall(ctx, OriginateGuestParams{
		CallID:    callID,
		SessionID: sessionID,
		Endpoint:  endpoint,
		CallerID:  "DongDo CS Guest",
	})
	return err
}

func (a *ariGatewayAdapter) OriginateAgentCall(ctx context.Context, callID int64, sessionID, agentExt string) error {
	if a.svc == nil {
		return errors.New("ari gateway: not configured")
	}
	return a.svc.AcceptCall(ctx, sessionID, agentExt)
}

func (a *ariGatewayAdapter) HangupCall(ctx context.Context, callID int64) error {
	if a.svc == nil || a.resolveSession == nil {
		return errors.New("ari gateway: not configured")
	}
	sessionID, err := a.resolveSession(ctx, callID)
	if err != nil {
		return err
	}
	return a.svc.HangupCall(ctx, sessionID)
}

func (a *ariGatewayAdapter) StartRecording(ctx context.Context, callID int64, filename string) error {
	// ARI does not expose a recording helper from this service. Real
	// recording is handled by Asterisk MixMonitor, configured in the
	// dialplan. Returning nil keeps the gateway contract intact.
	return nil
}
