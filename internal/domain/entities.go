package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)


// ============================================================
// User (CSKH Staff & Admin)
// ============================================================

type UserRole string

const (
	RoleOwner    UserRole = "owner"
	RoleAdmin    UserRole = "admin"
	RoleLeader   UserRole = "leader"
	RoleCSKH     UserRole = "cskh"
	RoleCustomer UserRole = "customer"
)

type User struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	Salt         string    `json:"-"`
	FullName     string    `json:"full_name"`
	Role         UserRole  `json:"role"`
	IsActive     bool      `json:"is_active"`
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

// ClientMsgIDString returns the string representation of ClientMsgID or empty string
func (m *Message) ClientMsgIDString() string {
	if m.ClientMsgID == nil {
		return ""
	}
	return m.ClientMsgID.String()
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
	LastSenderType string     `json:"last_sender_type,omitempty"`
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
	WSEventMessage        WSEventType = "message"
	WSEventTyping         WSEventType = "typing"
	WSEventStopTyping     WSEventType = "stop_typing"
	WSEventUnread         WSEventType = "unread"
	WSEventCaseUpdate     WSEventType = "case_update"
	WSEventLearningUpdate WSEventType = "learning_update"
	WSEventCallOffer      WSEventType = "call_offer"
	WSEventCallAnswer     WSEventType = "call_answer"
	WSEventCallICE        WSEventType = "call_ice"
	WSEventCallEnd        WSEventType = "call_end"
	WSEventCallRing       WSEventType = "call_ring"
	WSEventAIStatus       WSEventType = "ai_status"
)

type WSEvent struct {
	Type      WSEventType `json:"type"`
	SessionID string      `json:"session_id"`
	Payload   interface{} `json:"payload"`
	SenderID  string      `json:"sender_id,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
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

// ============================================================
// Partner Features Entities & Report DTOs
// ============================================================

type QuickTemplate struct {
	ID        int64     `json:"id"`
	Title     string    `json:"title"`
	Category  string    `json:"category"`
	Content   string    `json:"content"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type SystemPromptHistory struct {
	ID           int64     `json:"id"`
	SystemPrompt string    `json:"system_prompt"`
	LLMModel     string    `json:"llm_model"`
	Temperature  float64   `json:"temperature"`
	CreatedBy    string    `json:"created_by"`
	CreatedAt    time.Time `json:"created_at"`
}

type RolePermission struct {
	ID              int64     `json:"id"`
	RoleName        string    `json:"role_name"`
	FeatureKey      string    `json:"feature_key"`
	PermissionLevel string    `json:"permission_level"` // 'act', 'view', 'none'
	CanView         bool      `json:"can_view"`
	CanEdit         bool      `json:"can_edit"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type CSATFeedback struct {
	ID            int64     `json:"id"`
	SessionID     string    `json:"session_id"`
	Rating        int       `json:"rating"`
	FeedbackText  string    `json:"feedback_text"`
	TargetType    string    `json:"target_type"` // 'ai' or 'cskh'
	StaffUsername string    `json:"staff_username"`
	CreatedAt     time.Time `json:"created_at"`
}

type IssueCategory struct {
	ID           int64     `json:"id"`
	SessionID    string    `json:"session_id"`
	CategoryName string    `json:"category_name"`
	AIResolved   bool      `json:"ai_resolved"`
	CreatedAt    time.Time `json:"created_at"`
}

type SystemAuditLog struct {
	ID          int64     `json:"id"`
	ActionType  string    `json:"action_type"`
	Details     string    `json:"details"`
	PerformedBy string    `json:"performed_by"`
	CreatedAt   time.Time `json:"created_at"`
}

// Report DTOs
type DashboardKpiSummary struct {
	TotalConversations int     `json:"total_conversations"`
	AIResolvedCount    int     `json:"ai_resolved_count"`
	HumanHandoffCount  int     `json:"human_handoff_count"`
	AIRateVal          string  `json:"ai_rate_val"`
	AvgResponseTime    string  `json:"avg_response_time"`
	AvgCSAT            float64 `json:"avg_csat"`
	CSATVal            string  `json:"csat_val"`
}

type DashboardAutomationTrendDaily struct {
	DateDay      time.Time `json:"date_day"`
	Label        string    `json:"label"`
	TotalCases   int       `json:"total_cases"`
	AICases      int       `json:"ai_cases"`
	HandoffCases int       `json:"handoff_cases"`
}

type GeneralOverviewMetrics struct {
	TotalCustomers int    `json:"total_customers"`
	TotalCases     int    `json:"total_cases"`
	ResolvedCases  int    `json:"resolved_cases"`
	OpenCases      int    `json:"open_cases"`
	ResolutionRate string `json:"resolution_rate"`
}

type AIPerformanceMetrics struct {
	TotalCases       int     `json:"total_cases"`
	AIResolvedCases  int     `json:"ai_resolved_cases"`
	HandoffCases     int     `json:"handoff_cases"`
	AIResolutionRate string  `json:"ai_resolution_rate"`
	HandoffRate      string  `json:"handoff_rate"`
	AvgAICSAT        float64 `json:"avg_ai_csat"`
	AvgResponseTime  string  `json:"avg_response_time"`
}

type StaffPerformanceItem struct {
	StaffUsername     string  `json:"staff_username"`
	StaffFullName     string  `json:"staff_full_name"`
	StaffRole         string  `json:"staff_role"`
	TotalCasesHandled int     `json:"total_cases_handled"`
	ResolvedCases     int     `json:"resolved_cases"`
	AvgResponseTime   string  `json:"avg_response_time"`
	SLABreachRate     string  `json:"sla_breach_rate"`
	AvgCSAT           float64 `json:"avg_csat"`
	Status            string  `json:"status"`
}

type CXMetricsReport struct {
	TotalFeedbackCount    int     `json:"total_feedback_count"`
	AvgCSATScore          float64 `json:"avg_csat_score"`
	PositiveFeedbackCount int     `json:"positive_feedback_count"`
	NegativeFeedbackCount int     `json:"negative_feedback_count"`
	NSIIndex              string  `json:"nsi_index"`
	FCRRate               string  `json:"fcr_rate"`
}

type HourlyOperationalItem struct {
	HourOfDay     int `json:"hour_of_day"`
	TotalMessages int `json:"total_messages"`
}

type IssueAnalysisItem struct {
	CategoryName     string `json:"category_name"`
	TotalRequests    int    `json:"total_requests"`
	PercentageShare  string `json:"percentage_share"`
	AIResolvedCount  int    `json:"ai_resolved_count"`
	AIResolutionRate string `json:"ai_resolution_rate"`
}

type AILearningReportStats struct {
	PendingCount       int    `json:"pending_count"`
	ApprovedCount      int    `json:"approved_count"`
	RejectedCount      int    `json:"rejected_count"`
	TotalLearningItems int    `json:"total_learning_items"`
	ApprovalRate       string `json:"approval_rate"`
}

type SystemErrorRecord struct {
	ID           string    `json:"id"`
	Source       string    `json:"source"`
	Title        string    `json:"title"`
	Details      string    `json:"details"`
	Severity     string    `json:"severity"`
	IsHandled    bool      `json:"is_handled"`
	SuggestedFix string    `json:"suggested_fix"`
	CreatedAt    time.Time `json:"created_at"`
}

// ============================================================
// Chat Tag System
// ============================================================

type ChatTag struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Color       string    `json:"color"`
	CreatedBy   string    `json:"created_by"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CaseTag struct {
	ID         int64     `json:"id"`
	SessionID  string    `json:"session_id"`
	TagID      int64     `json:"tag_id"`
	AssignedBy string    `json:"assigned_by"`
	CreatedAt  time.Time `json:"created_at"`
	// Joined fields
	TagName  string `json:"tag_name,omitempty"`
	TagColor string `json:"tag_color,omitempty"`
}

type CaseTagHistory struct {
	ID          int64     `json:"id"`
	SessionID   string    `json:"session_id"`
	TagID       int64     `json:"tag_id"`
	TagName     string    `json:"tag_name"`
	TagColor    string    `json:"tag_color"`
	Action      string    `json:"action"` // "attach" | "detach"
	PerformedBy string    `json:"performed_by"`
	CreatedAt   time.Time `json:"created_at"`
}

// ============================================================
// Alert Config & Events
// ============================================================

type AlertConfig struct {
	ID             int64     `json:"id"`
	IsEnabled      bool      `json:"is_enabled"`
	TimeoutSeconds int       `json:"timeout_seconds"`
	AlertContent   string    `json:"alert_content"`
	UpdatedBy      string    `json:"updated_by"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type AlertEvent struct {
	ID             int64      `json:"id"`
	SessionID      string     `json:"session_id"`
	TimeoutSeconds int        `json:"timeout_seconds"`
	TriggeredAt    time.Time  `json:"triggered_at"`
	ResolvedAt     *time.Time `json:"resolved_at,omitempty"`
	IsResolved     bool       `json:"is_resolved"`
}

// ============================================================
// ChatTagRepository interface
// ============================================================

type ChatTagRepository interface {
	// Tag CRUD
	ListTags(ctx context.Context) ([]*ChatTag, error)
	CreateTag(ctx context.Context, tag *ChatTag) (*ChatTag, error)
	UpdateTag(ctx context.Context, id int64, name, description, color string) error
	DeleteTag(ctx context.Context, id int64) error

	// Case Tag operations
	GetCaseTags(ctx context.Context, sessionID string) ([]*CaseTag, error)
	AttachTag(ctx context.Context, sessionID string, tagID int64, assignedBy string) error
	DetachTag(ctx context.Context, sessionID string, tagID int64, performedBy string) error

	// Alert Config
	GetAlertConfig(ctx context.Context) (*AlertConfig, error)
	UpsertAlertConfig(ctx context.Context, cfg *AlertConfig) error

	// Alert Events
	CreateAlertEvent(ctx context.Context, sessionID string, timeoutSeconds int) (*AlertEvent, error)
	ResolveAlertEvent(ctx context.Context, sessionID string) error
	ListUnresolvedAlertEvents(ctx context.Context) ([]*AlertEvent, error)
}

