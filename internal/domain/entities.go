package domain

import (
	"time"

	"github.com/google/uuid"
)

// ============================================================
// User (CSKH Staff & Admin)
// ============================================================

type UserRole string

const (
	RoleAdmin    UserRole = "admin"
	RoleCSKH     UserRole = "cskh"
	RoleCustomer UserRole = "customer"
)

type User struct {
	ID           int64    `json:"id"`
	Username     string   `json:"username"`
	PasswordHash string   `json:"-"`
	Salt         string   `json:"-"`
	FullName     string   `json:"full_name"`
	Role         UserRole `json:"role"`
	IsActive     bool     `json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
}

// ============================================================
// Session (Auth Token)
// ============================================================

type Session struct {
	ID        int64     `json:"id"`
	Token     string    `json:"token"`
	Username  string    `json:"username"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

type SessionUser struct {
	Username string   `json:"username"`
	FullName string   `json:"full_name"`
	Role     UserRole `json:"role"`
	Token    string   `json:"token,omitempty"`
}

// ============================================================
// Guest (Customer pre-chat)
// ============================================================

type Guest struct {
	ID          int64     `json:"id"`
	GuestID     uuid.UUID `json:"guest_id"`
	DisplayName string    `json:"display_name"`
	Phone       string    `json:"phone,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type CustomerProfile struct {
	ID            int64     `json:"id"`
	GuestID       string    `json:"guest_id"`
	DisplayName   string    `json:"display_name"`
	Phone         string    `json:"phone"`
	LastSessionID string    `json:"last_session_id,omitempty"`
	LastMessage   string    `json:"last_message,omitempty"`
	LastStatus    string    `json:"last_status,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// ============================================================
// Message
// ============================================================

type SenderType string

const (
	SenderGuest   SenderType = "guest"
	SenderAI      SenderType = "ai"
	SenderHumanCS SenderType = "human_cs"
	SenderSystem  SenderType = "system"
)

type Message struct {
	ID          int64      `json:"id"`
	SessionID   string     `json:"session_id"`
	SenderType  SenderType `json:"sender_type"`
	SenderID    string     `json:"sender_id"`
	Content     string     `json:"content"`
	ClientMsgID *uuid.UUID `json:"client_msg_id,omitempty"`
	IsLearned   bool       `json:"is_learned"`
	CreatedAt   time.Time  `json:"created_at"`
}

// ============================================================
// Chat Case (Live CS Inbox)
// ============================================================

type CaseStatus string

const (
	StatusAIActive      CaseStatus = "AI_ACTIVE"
	StatusNeedsHumanCS  CaseStatus = "NEEDS_HUMAN_CS"
	StatusHumanCSActive CaseStatus = "HUMAN_CS_ACTIVE"
	StatusResolved      CaseStatus = "RESOLVED"
)

type ChatCase struct {
	ID             int64      `json:"id"`
	SessionID      string     `json:"session_id"`
	GuestID        *uuid.UUID `json:"guest_id,omitempty"`
	CustomerName   string     `json:"customer_name"`
	CustomerPhone  string     `json:"customer_phone"`
	Status         CaseStatus `json:"status"`
	AssignedCS     string     `json:"assigned_cs"`
	LastMessage    string     `json:"last_message"`
	ResolutionNote string     `json:"resolution_note"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// ============================================================
// Learning Queue
// ============================================================

type LearnStatus string

const (
	LearnPending  LearnStatus = "PENDING"
	LearnApproved LearnStatus = "APPROVED"
	LearnRejected LearnStatus = "REJECTED"
)

type LearningItem struct {
	ID         int64       `json:"id"`
	SessionID  string      `json:"session_id"`
	Question   string      `json:"question"`
	Answer     string      `json:"answer"`
	Status     LearnStatus `json:"status"`
	CreatedBy  string      `json:"created_by"`
	ApprovedBy string      `json:"approved_by"`
	CreatedAt  time.Time   `json:"created_at"`
	ApprovedAt *time.Time  `json:"approved_at,omitempty"`
}

// ============================================================
// Voice Call
// ============================================================

type CallStatus string

const (
	CallRinging  CallStatus = "RINGING"
	CallActive   CallStatus = "ACTIVE"
	CallEnded    CallStatus = "ENDED"
	CallMissed   CallStatus = "MISSED"
	CallRejected CallStatus = "REJECTED"
)

type CallerType string

const (
	CallerGuest   CallerType = "guest"
	CallerCSKH    CallerType = "cskh"
	CallerHumanCS CallerType = "cskh"
)

type VoiceCall struct {
	ID              int64      `json:"id"`
	SessionID       string     `json:"session_id"`
	CallerType      CallerType `json:"caller_type"`
	CallerID        string     `json:"caller_id"`
	CalleeType      CallerType `json:"callee_type"`
	CalleeID        string     `json:"callee_id"`
	Status          CallStatus `json:"status"`
	DurationSeconds int        `json:"duration_seconds"`
	RecordingURL    string     `json:"recording_url"`
	Transcript      string     `json:"transcript"`
	CreatedAt       time.Time  `json:"created_at"`
	EndedAt         *time.Time `json:"ended_at,omitempty"`
}

// ============================================================
// Analytics
// ============================================================

type AnalyticsStats struct {
	TotalCases        int     `json:"total_cases"`
	TotalSessions     int     `json:"total_sessions"`
	AIActiveCases     int     `json:"ai_active_cases"`
	NeedsHumanCases   int     `json:"needs_human_cases"`
	ActiveHumanCases  int     `json:"active_human_cases"`
	ResolvedCases     int     `json:"resolved_cases"`
	AIServiceRate     float64 `json:"ai_self_service_rate"`
	TotalLearnedQA    int     `json:"total_learned_qa"`
	PendingLearnCount int     `json:"pending_learn_count"`
}

// ============================================================
// WebSocket Event Types
// ============================================================

type WSEventType string

const (
	WSEventMessage    WSEventType = "message"
	WSEventTyping     WSEventType = "typing"
	WSEventStopTyping WSEventType = "stop_typing"
	WSEventUnread     WSEventType = "unread"
	WSEventCaseUpdate WSEventType = "case_update"
	WSEventCallOffer  WSEventType = "call_offer"
	WSEventCallAnswer WSEventType = "call_answer"
	WSEventCallICE    WSEventType = "call_ice"
	WSEventCallEnd    WSEventType = "call_end"
	WSEventCallRing   WSEventType = "call_ring"
	WSEventAIStatus   WSEventType = "ai_status"
)

type WSEvent struct {
	Type      WSEventType    `json:"type"`
	SessionID string         `json:"session_id"`
	Payload   interface{}    `json:"payload"`
	SenderID  string         `json:"sender_id,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
}

// ============================================================
// Redis Stream Messages
// ============================================================

type StreamMessage struct {
	ID        string      `json:"id"`
	SessionID string      `json:"session_id"`
	Event     WSEventType `json:"event"`
	Payload   string      `json:"payload"` // JSON encoded
	SenderID  string      `json:"sender_id"`
	Timestamp int64       `json:"timestamp"`
}

// ============================================================
// QA Pair (for learning extraction)
// ============================================================

type QAPair struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
}
