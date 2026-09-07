package domain

import "time"

type Call struct {
	ID              string     `json:"id"`
	SessionID       string     `json:"session_id"`
	CustomerID      string     `json:"customer_id"`
	AgentID         string     `json:"agent_id,omitempty"`
	Status          CallStatus `json:"status"`
	RequestedAt     time.Time  `json:"requested_at"`
	AssignedAt      *time.Time `json:"assigned_at,omitempty"`
	StartedAt       *time.Time `json:"started_at,omitempty"`
	EndedAt         *time.Time `json:"ended_at,omitempty"`
	DurationSeconds int        `json:"duration_seconds"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`

	// Session & Recording metadata
	Session   *CallSession   `json:"session,omitempty"`
	Recording *CallRecording `json:"recording,omitempty"`
}

type CallSession struct {
	CallID            string    `json:"call_id"`
	CustomerChannelID string    `json:"customer_channel_id,omitempty"`
	AgentChannelID    string    `json:"agent_channel_id,omitempty"`
	BridgeID          string    `json:"bridge_id,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type CallEvent struct {
	ID        int64                  `json:"id"`
	CallID    string                 `json:"call_id"`
	EventType string                 `json:"event_type"`
	Source    string                 `json:"source"` // 'API', 'SYSTEM', 'ARI'
	Payload   map[string]interface{} `json:"payload"`
	CreatedAt time.Time              `json:"created_at"`
}

type CallRecording struct {
	ID            int64     `json:"id"`
	CallID        string    `json:"call_id"`
	RecordingURL  string    `json:"recording_url"`
	FileSizeBytes int64     `json:"file_size_bytes,omitempty"`
	Transcript    string    `json:"transcript,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

type AgentState struct {
	AgentID     string      `json:"agent_id"`
	Status      AgentStatus `json:"status"`
	CurrentCall string      `json:"current_call,omitempty"`
	LastSeenAt  time.Time   `json:"last_seen_at"`
}
