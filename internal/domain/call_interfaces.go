package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ============================================================
// Call v2 — Repository & Infrastructure Interfaces
// ============================================================

// CallRepository is the persistence interface for the new call subsystem.
type CallRepository interface {
	// Insert creates a new call row in CREATED state.
	Insert(ctx context.Context, c *Call) error

	// Get returns a call by id, or (nil, nil) if not found.
	Get(ctx context.Context, id uuid.UUID) (*Call, error)

	// GetByIdempotencyKey returns a cached response if the key was used before.
	GetByIdempotencyKey(ctx context.Context, key string) (*IdempotencyRecord, error)

	// SaveIdempotency caches the response for an idempotent POST.
	SaveIdempotency(ctx context.Context, rec *IdempotencyRecord) error

	// ClaimIdempotency atomically reserves an idempotency key. Returns
	// claimed=true if THIS call reserved the slot (proceed with creation),
	// claimed=false if another request already reserved it (look up via
	// GetByIdempotencyKey and replay).
	ClaimIdempotency(ctx context.Context, key string) (claimed bool, err error)

	// UpdateState changes status atomically (CAS on current status).
	// Returns ErrInvalidTransition if the row is not in `from`.
	UpdateState(ctx context.Context, id uuid.UUID, from, to CallStatus, fields StateTransitionFields) error

	// AssignAgent sets agent_id, assigned_at and transitions WAITING→WAITING_AGENT atomically.
	// Returns ErrInvalidTransition if the call is not WAITING.
	AssignAgent(ctx context.Context, id uuid.UUID, agentID string) error

	// RecordParticipant records a participant action; returns ErrDuplicateAction on unique conflict.
	RecordParticipant(ctx context.Context, p *CallParticipant) error

	// AppendEvent adds an audit row. Idempotent on (call_id, event_type, occurred_at).
	AppendEvent(ctx context.Context, e *CallEvent) error

	// InsertRecording starts a recording row.
	InsertRecording(ctx context.Context, r *CallRecording) error

	// UpdateRecording updates the recording row when ARI finishes.
	UpdateRecording(ctx context.Context, callID uuid.UUID, ariName, storageURL string, endedAt time.Time, state CallRecordingState) error

	// ListByAgent returns recent calls for an agent (admin UI).
	ListByAgent(ctx context.Context, agentID string, limit, offset int) ([]*Call, error)

	// ListActiveByAgent returns non-terminal calls owned by an agent.
	// Used after a WebSocket reconnect so the admin layout can re-show
	// the ringing banner for a call announced before the page reload.
	ListActiveByAgent(ctx context.Context, agentID string) ([]*Call, error)

	// ListByCustomer returns recent calls for a customer.
	ListByCustomer(ctx context.Context, customerID string, limit, offset int) ([]*Call, error)

	// ListActive returns calls whose status is not terminal (used by reconciliation worker).
	ListActive(ctx context.Context, olderThan time.Duration, limit int) ([]*Call, error)

	// ListAll returns a paginated slice of all calls (Call v2). Used by the
	// admin call history view, which previously only saw the legacy
	// `voice_calls` table.
	ListAll(ctx context.Context, limit, offset int) ([]*Call, error)
}

// StateTransitionFields bundles optional column updates alongside a state change.
type StateTransitionFields struct {
	AgentID         *string
	AssignedAt      *time.Time
	StartedAt       *time.Time
	EndedAt         *time.Time
	DurationSeconds *int
	ARIBridgeID     *string
	FailureReason   *string
}

// CallQueueManager owns the realtime queue + agent state in Redis.
// All operations are atomic (Lua). Failures return wrapped errors.
//
// Dedup note (IsAnnounced/MarkAnnounced): callers use these to ensure each
// call is announced to an agent exactly once during its entire lifetime.
// Without dedup, when ring timeout fires, the routing worker re-pops the
// call from the queue and re-publishes `incoming_call` to admins every
// RingTimeout seconds, causing UI spam and missed-call timer restarts.
type CallQueueManager interface {
	// Enqueue adds a call to the waiting queue with priority (lower score = sooner).
	Enqueue(ctx context.Context, callID uuid.UUID, priority int, scoreMS int64) error

	// PopNext atomically pops the call with lowest score and returns it.
	// Returns (uuid.Nil, false, nil) when queue is empty.
	PopNext(ctx context.Context) (uuid.UUID, bool, error)

	// QueuePosition returns the 0-based position of a call in the queue (0 = next).
	QueuePosition(ctx context.Context, callID uuid.UUID) (int64, error)

	// QueueSize returns the current waiting count.
	QueueSize(ctx context.Context) (int64, error)

	// RemoveFromQueue removes a call from the waiting queue (cancel scenarios).
	RemoveFromQueue(ctx context.Context, callID uuid.UUID) error

	// ReserveAgent atomically transitions agent AVAILABLE→RESERVED and binds it to a call.
	// Returns (false, currentState, error) when not available.
	ReserveAgent(ctx context.Context, agentID string, callID uuid.UUID) (bool, AgentState, error)

	// ReleaseAgent transitions RESERVED/RINGING/BUSY → AVAILABLE and unbinds.
	ReleaseAgent(ctx context.Context, agentID string) (AgentState, error)

	// SetAgentBusy transitions RESERVED/RINGING → BUSY.
	SetAgentBusy(ctx context.Context, agentID string, callID uuid.UUID) error

	// SetAgentAway/Available transitions between AVAILABLE ↔ AWAY.
	SetAgentAway(ctx context.Context, agentID string) error
	SetAgentAvailable(ctx context.Context, agentID string) error

	// SetAgentOffline marks an agent offline (WS disconnect).
	SetAgentOffline(ctx context.Context, agentID string) error

	// HeartbeatAgent refreshes last_seen for an agent (used to detect offline).
	HeartbeatAgent(ctx context.Context, agentID string) error

	// GetAgent returns the current agent state.
	GetAgent(ctx context.Context, agentID string) (*Agent, error)

	// ListAvailable returns agents in AVAILABLE state (admin view).
	ListAvailable(ctx context.Context) ([]string, error)

	// ListActiveAgents returns agents in {RESERVED,RINGING,BUSY} (for reconciliation).
	ListActiveAgents(ctx context.Context) ([]string, error)

	// MarkAnnounced records that a call has been published to an admin/agent
	// (i.e. `incoming_call` event fired). Subsequent calls return false from
	// IsAnnounced so the routing worker can skip re-announce. The TTL should
	// be at least the max ring-timeout you want to suppress, e.g.
	// ringTimeout + some slack.
	MarkAnnounced(ctx context.Context, callID uuid.UUID, ttl time.Duration) error

	// IsAnnounced returns true if the call was already announced and the
	// TTL has not expired. If true, the routing worker MUST skip the
	// publish step (`incoming_call`).
	IsAnnounced(ctx context.Context, callID uuid.UUID) (bool, error)

	// ClearAnnounced is called when the call leaves the WAITING_AGENT phase
	// (accept/reject/cancel/end). After this, IsAnnounced returns false so
	// the routing flow starts fresh next time.
	ClearAnnounced(ctx context.Context, callID uuid.UUID) error
}

// AsteriskGateway is the seam to the Asterisk ARI layer.
// Implementations live under internal/infrastructure/asterisk.
type AsteriskGateway interface {
	// OriginateCustomer dials the customer's SIP endpoint and connects it to a Stasis app.
	OriginateCustomer(ctx context.Context, callID uuid.UUID, customerEndpoint string) (channelID string, err error)

	// OriginateAgent dials the agent's SIP endpoint and connects it to the Stasis app.
	OriginateAgent(ctx context.Context, callID uuid.UUID, agentID string) (channelID string, err error)

	// BridgeChannels adds both channels into a bridge.
	BridgeChannels(ctx context.Context, bridgeID string, channelIDs ...string) error

	// CreateBridge creates a holding bridge.
	CreateBridge(ctx context.Context, bridgeType string) (bridgeID string, err error)

	// Hangup terminates a channel (or all channels of a call).
	Hangup(ctx context.Context, callID uuid.UUID) error

	// StartRecording begins MixMonitor on both channels.
	StartRecording(ctx context.Context, callID uuid.UUID, bridgeID string) error

	// StopRecording ends MixMonitor and finalizes the recording.
	StopRecording(ctx context.Context, callID uuid.UUID) error

	// HealthCheck pings ARI. If false, callers must fail-fast the call (→ FAILED).
	HealthCheck(ctx context.Context) error
}

// ARIEvent is the parsed representation of an ARI WebSocket message.
type ARIEvent struct {
	Type        string         `json:"type"` // e.g. "StasisStart", "ChannelStateChange"
	ChannelID   string         `json:"channel_id,omitempty"`
	Channel     *ARIChannel    `json:"channel,omitempty"`
	BridgeID    string         `json:"bridge_id,omitempty"`
	Bridge      *ARIBridge     `json:"bridge,omitempty"`
	Recording   *ARIRecording  `json:"recording,omitempty"`
	Application string         `json:"application,omitempty"`
	Timestamp   time.Time      `json:"timestamp"`
	Args        []string       `json:"args,omitempty"`
	Variables   map[string]any `json:"variables,omitempty"`
}

// ARIChannel is the subset of channel fields we care about.
type ARIChannel struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	State      string `json:"state"`
	CallerNum  string `json:"caller.number,omitempty"`
	CallerName string `json:"caller.name,omitempty"`
	Dialplan   struct {
		Context  string `json:"context,omitempty"`
		Exten    string `json:"exten,omitempty"`
		Priority int    `json:"priority,omitempty"`
		AppName  string `json:"app_name,omitempty"`
		AppData  string `json:"app_data,omitempty"`
	} `json:"dialplan,omitempty"`
	ChannelVars map[string]string `json:"channelvars,omitempty"`
}

// ARIRecording is the subset of recording fields we care about.
type ARIRecording struct {
	Name     string `json:"name"`
	Format   string `json:"format"`
	State    string `json:"state"`
	Duration string `json:"duration,omitempty"`
}

// ARIBridge is the subset of bridge fields we care about.
type ARIBridge struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Channels []string `json:"channels"`
	State    string   `json:"state,omitempty"`
}

// ARIEventConsumer is the long-running consumer of ARI WebSocket events.
type ARIEventConsumer interface {
	// Start begins consuming; blocks until ctx is cancelled or a fatal error occurs.
	Start(ctx context.Context) error
}
