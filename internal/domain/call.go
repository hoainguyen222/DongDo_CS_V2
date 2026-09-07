package domain

import (
	"time"

	"github.com/google/uuid"
)

// ============================================================
// Call v2 — authoritative state lives in Go, mirrored to Postgres.
// ============================================================

// CallStatus is the lifecycle state of a call. Validated by CallStateMachine.
type CallStatus string

const (
	CallStatusCreated      CallStatus = "CREATED"
	CallStatusWaiting      CallStatus = "WAITING"
	CallStatusWaitingAgent CallStatus = "WAITING_AGENT"
	CallStatusConnecting   CallStatus = "CONNECTING"
	CallStatusRinging      CallStatus = "RINGING"
	CallStatusInProgress   CallStatus = "IN_PROGRESS"
	CallStatusEnded        CallStatus = "ENDED"
	CallStatusRejected     CallStatus = "REJECTED"
	CallStatusCancelled    CallStatus = "CANCELLED"
	CallStatusMissed       CallStatus = "MISSED"
	CallStatusFailed       CallStatus = "FAILED"
	CallStatusTimeout      CallStatus = "TIMEOUT"
)

// IsTerminal reports whether the status is end-of-life and no further transitions are allowed.
func (s CallStatus) IsTerminal() bool {
	switch s {
	case CallStatusEnded, CallStatusRejected, CallStatusCancelled,
		CallStatusMissed, CallStatusFailed, CallStatusTimeout:
		return true
	}
	return false
}

// AgentState is the lifecycle state of a support agent.
type AgentState string

const (
	AgentOffline   AgentState = "OFFLINE"
	AgentAvailable AgentState = "AVAILABLE"
	AgentReserved  AgentState = "RESERVED"
	AgentRinging   AgentState = "RINGING"
	AgentBusy      AgentState = "BUSY"
	AgentAway      AgentState = "AWAY"
)

// IsActive reports whether the agent is currently occupied with a call.
func (a AgentState) IsActive() bool {
	switch a {
	case AgentReserved, AgentRinging, AgentBusy:
		return true
	}
	return false
}

// Call is the authoritative representation of one customer call.
type Call struct {
	ID              uuid.UUID  `json:"id"`
	CustomerID      string     `json:"customer_id"`
	AgentID         *string    `json:"agent_id,omitempty"`
	Status          CallStatus `json:"status"`
	Priority        int        `json:"priority"`
	RequestedAt     time.Time  `json:"requested_at"`
	AssignedAt      *time.Time `json:"assigned_at,omitempty"`
	StartedAt       *time.Time `json:"started_at,omitempty"`
	EndedAt         *time.Time `json:"ended_at,omitempty"`
	DurationSeconds int        `json:"duration_seconds"`
	LastEventAt     time.Time  `json:"last_event_at"`
	ARIBridgeID     string     `json:"ari_bridge_id,omitempty"`
	FailureReason   string     `json:"failure_reason,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// CallEventSource identifies where a call event came from.
type CallEventSource string

const (
	CallEventSourceAPI    CallEventSource = "API"
	CallEventSourceARI    CallEventSource = "ARI"
	CallEventSourceSystem CallEventSource = "SYSTEM"
)

// CallEvent is one entry in the append-only audit log.
type CallEvent struct {
	ID         int64           `json:"id"`
	CallID     uuid.UUID       `json:"call_id"`
	EventType  string          `json:"event_type"`
	Source     CallEventSource `json:"source"`
	AgentID    string          `json:"agent_id,omitempty"`
	Payload    map[string]any  `json:"payload"`
	OccurredAt time.Time       `json:"occurred_at"`
	CreatedAt  time.Time       `json:"created_at"`
}

// CallPartyRole identifies who took an action on a call.
type CallPartyRole string

const (
	CallPartyCustomer CallPartyRole = "customer"
	CallPartyAgent    CallPartyRole = "agent"
	CallPartySystem   CallPartyRole = "system"
)

// CallParticipant records an explicit action by a participant (used for idempotency).
type CallParticipant struct {
	ID            int64         `json:"id"`
	CallID        uuid.UUID     `json:"call_id"`
	ParticipantID string        `json:"participant_id"`
	Role          CallPartyRole `json:"role"`
	Action        string        `json:"action"`
	ActionAt      time.Time     `json:"action_at"`
}

// CallRecordingState tracks the recording lifecycle.
type CallRecordingState string

const (
	CallRecordingPending   CallRecordingState = "pending"
	CallRecordingRecording CallRecordingState = "recording"
	CallRecordingStopped   CallRecordingState = "stopped"
	CallRecordingFailed    CallRecordingState = "failed"
)

// CallRecording tracks metadata for one recording of a call.
type CallRecording struct {
	ID           int64              `json:"id"`
	CallID       uuid.UUID          `json:"call_id"`
	ARIRecording string             `json:"ari_recording,omitempty"`
	StorageURL   string             `json:"storage_url,omitempty"`
	StartedAt    *time.Time         `json:"started_at,omitempty"`
	EndedAt      *time.Time         `json:"ended_at,omitempty"`
	State        CallRecordingState `json:"state"`
}

// Agent is the runtime projection of a support agent (identity lives in users table).
type Agent struct {
	ID          string     `json:"id"`
	State       AgentState `json:"state"`
	CurrentCall *uuid.UUID `json:"current_call,omitempty"`
	LastSeenAt  time.Time  `json:"last_seen_at"`
}

// IdempotencyRecord caches the response of an idempotent POST so retries return the same body.
type IdempotencyRecord struct {
	Key          string         `json:"key"`
	CallID       uuid.UUID      `json:"call_id"`
	ResponseBody map[string]any `json:"response_body"`
	CreatedAt    time.Time      `json:"created_at"`
}

// ============================================================
// New WebSocket event types (Call v2)
// ============================================================

const (
	WSEventCallWaiting       WSEventType = "call_waiting"
	WSEventIncomingCall      WSEventType = "incoming_call"
	WSEventCallConnecting    WSEventType = "call_connecting"
	WSEventCallRingingV2     WSEventType = "call_ringing_v2"
	WSEventCallStarted       WSEventType = "call_started"
	WSEventCallEndedV2       WSEventType = "call_ended_v2"
	WSEventCallFailed        WSEventType = "call_failed"
	WSEventAgentStatusChange WSEventType = "agent_status_changed"
	WSEventQueuePosition     WSEventType = "queue_position_changed"
)

// ParticipantAction values used for CallParticipant.Action column.
const (
	CallActionAccept  = "accept"
	CallActionReject  = "reject"
	CallActionHangup  = "hangup"
	CallActionCancel  = "cancel"
	CallActionTimeout = "timeout"
)
