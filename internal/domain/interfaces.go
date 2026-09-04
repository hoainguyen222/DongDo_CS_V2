package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// ============================================================
// Repository Interfaces
// ============================================================

type UserRepository interface {
	Create(ctx context.Context, username, passwordHash, salt, fullName string, role UserRole) (*User, error)
	GetByUsername(ctx context.Context, username string) (*User, error)
	List(ctx context.Context) ([]*User, error)
	Delete(ctx context.Context, username string) error
	UpdatePassword(ctx context.Context, username, passwordHash, salt string) error
	UpdateUser(ctx context.Context, username, fullName string, role UserRole, isActive bool, passwordHash, salt string) (*User, error)
}

type SessionRepository interface {
	Create(ctx context.Context, token, username string, expiresAt time.Time) (*Session, error)
	Verify(ctx context.Context, token string) (*SessionUser, error)
	Delete(ctx context.Context, token string) error
	DeleteExpired(ctx context.Context) error
	DeleteByUsername(ctx context.Context, username string) error
}

type GuestRepository interface {
	Create(ctx context.Context, guestID uuid.UUID, displayName, phone string) (*Guest, error)
	GetByID(ctx context.Context, guestID uuid.UUID) (*Guest, error)
	List(ctx context.Context) ([]*CustomerProfile, error)
	Update(ctx context.Context, guestID uuid.UUID, displayName, phone string) error
	Delete(ctx context.Context, guestID uuid.UUID) error
}

type MessageRepository interface {
	Insert(ctx context.Context, msg *Message) (*Message, error)
	InsertBatch(ctx context.Context, msgs []*Message) error
	GetHistory(ctx context.Context, sessionID string) ([]*Message, error)
	GetUnlearned(ctx context.Context) ([]*Message, error)
	MarkLearned(ctx context.Context, ids []int64) error
	DeleteBySession(ctx context.Context, sessionID string) error
	DeleteAll(ctx context.Context) error
	ResetLearnedFlags(ctx context.Context) error
}

type CaseRepository interface {
	Upsert(ctx context.Context, sessionID string, guestID *uuid.UUID, customerName, customerPhone string, status CaseStatus, assignedCS, lastMessage string) (*ChatCase, error)
	List(ctx context.Context, statusFilter CaseStatus) ([]*ChatCase, error)
	Get(ctx context.Context, sessionID string) (*ChatCase, error)
	Assign(ctx context.Context, sessionID, csUsername string) error
	Resolve(ctx context.Context, sessionID, csUsername, resolutionNote string) error
	Delete(ctx context.Context, sessionID string) error
	DeleteAll(ctx context.Context) error
}

type LearningRepository interface {
	Add(ctx context.Context, sessionID, question, answer string, status LearnStatus, createdBy string) (*LearningItem, error)
	ListByStatus(ctx context.Context, status LearnStatus) ([]*LearningItem, error)
	Get(ctx context.Context, id int64) (*LearningItem, error)
	UpdateContent(ctx context.Context, id int64, question, answer string) error
	MarkStatus(ctx context.Context, id int64, status LearnStatus, approvedBy string) error
	DeleteBySession(ctx context.Context, sessionID string) error
	ClearAll(ctx context.Context) error
}

type SettingRepository interface {
	Get(ctx context.Context, key, fallback string) (string, error)
	Set(ctx context.Context, key, value string) error
}

type VoiceCallRepository interface {
	Create(ctx context.Context, call *VoiceCall) (*VoiceCall, error)
	UpdateStatus(ctx context.Context, id int64, status CallStatus) error
	End(ctx context.Context, id int64, durationSeconds int, recordingURL string) error
	MarkMissed(ctx context.Context, id int64) error
	SetTranscript(ctx context.Context, id int64, transcript string) error
	GetBySession(ctx context.Context, sessionID string) ([]*VoiceCall, error)
	ListAll(ctx context.Context) ([]*VoiceCall, error)
	GetByID(ctx context.Context, id int64) (*VoiceCall, error)
	Delete(ctx context.Context, id int64) error
}

type AnalyticsRepository interface {
	GetStats(ctx context.Context) (*AnalyticsStats, error)
}

// ============================================================
// Vector DB & Search Interfaces (Qdrant)
// ============================================================

type KnowledgeDocument struct {
	ID        string                 `json:"id"`
	Content   string                 `json:"content"`
	Score     float32                `json:"score"`
	Source    string                 `json:"source"`
	Metadata  map[string]interface{} `json:"metadata"`
}

type VectorStore interface {
	Search(ctx context.Context, queryVector []float32, limit int, scoreThreshold float32) ([]*KnowledgeDocument, error)
	Upsert(ctx context.Context, docs []*KnowledgeDocument, vectors [][]float32) error
	DeleteBySource(ctx context.Context, source string) (int, error)
	Count(ctx context.Context) (int64, error)
}

type Embedder interface {
	EmbedText(ctx context.Context, text string) ([]float32, error)
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
}

// ============================================================
// LLM & RAG Interface
// ============================================================

type LLMClient interface {
	GenerateResponse(ctx context.Context, systemPrompt string, history []*Message, query string, contextBlock string) (reply string, isFallback bool, err error)
	StreamResponse(ctx context.Context, systemPrompt string, history []*Message, query string, contextBlock string, outChan chan<- string) (isFallback bool, err error)
}

// ============================================================
// Event Bus & State (Redis)
// ============================================================

type HubBroadcaster interface {
	BroadcastToSession(sessionID string, event *WSEvent)
	BroadcastToSessionExcept(sessionID string, event *WSEvent, excludeUserID string)
}

type EventBus interface {
	SetHub(hub HubBroadcaster)
	PublishWS(ctx context.Context, sessionID string, event WSEventType, payload interface{}, senderID string) error
	PublishAIJob(ctx context.Context, sessionID string, query string, senderID string, clientMsgID *uuid.UUID) error
	PublishDBJob(ctx context.Context, msg *Message) error
}

type StateManager interface {
	SetTyping(ctx context.Context, sessionID, userID string) error
	IsTyping(ctx context.Context, sessionID, userID string) (bool, error)
	IncrementUnread(ctx context.Context, sessionID, recipientID string) (int64, error)
	GetUnread(ctx context.Context, sessionID, recipientID string) (int64, error)
	ClearUnread(ctx context.Context, sessionID, recipientID string) error
	AcquireLock(ctx context.Context, key, owner string, ttl time.Duration) (bool, error)
	ReleaseLock(ctx context.Context, key, owner string) error
	SetAIExecution(ctx context.Context, sessionID string, active bool) error
	IsAIExecuting(ctx context.Context, sessionID string) (bool, error)
}

// ============================================================
// Partner System Repository Interface
// ============================================================

type PartnerRepository interface {
	// Dashboard
	GetDashboardKpi(ctx context.Context, startDate, endDate time.Time) (*DashboardKpiSummary, error)
	GetDashboardAutomationTrend(ctx context.Context, startDate, endDate time.Time) ([]*DashboardAutomationTrendDaily, error)
	GetRecentCompletedChats(ctx context.Context, limit, offset int) ([]*ChatCase, error)
	
	// Quick Templates
	ListQuickTemplates(ctx context.Context) ([]*QuickTemplate, error)
	CreateQuickTemplate(ctx context.Context, t *QuickTemplate) (*QuickTemplate, error)
	UpdateQuickTemplate(ctx context.Context, id int64, title, category, content string) error
	DeleteQuickTemplate(ctx context.Context, id int64) error

	// Prompt History & RBAC
	GetLatestSystemPromptHistory(ctx context.Context) (*SystemPromptHistory, error)
	InsertSystemPromptHistory(ctx context.Context, p *SystemPromptHistory) (*SystemPromptHistory, error)
	ListRolePermissions(ctx context.Context) ([]*RolePermission, error)
	UpsertRolePermission(ctx context.Context, p *RolePermission) error

	// Audit Logs
	InsertAuditLog(ctx context.Context, log *SystemAuditLog) (*SystemAuditLog, error)
	ListAuditLogs(ctx context.Context, limit, offset int) ([]*SystemAuditLog, error)

	// Reports
	GetGeneralOverviewReport(ctx context.Context, startDate, endDate time.Time) (*GeneralOverviewMetrics, error)
	GetAIPerformanceReport(ctx context.Context, startDate, endDate time.Time) (*AIPerformanceMetrics, error)
	GetStaffPerformanceReport(ctx context.Context, startDate, endDate time.Time) ([]*StaffPerformanceItem, error)
	GetCXReport(ctx context.Context, startDate, endDate time.Time) (*CXMetricsReport, error)
	InsertCSATFeedback(ctx context.Context, fb *CSATFeedback) (*CSATFeedback, error)
	GetHourlyOperationalLoad(ctx context.Context, startDate, endDate time.Time) ([]*HourlyOperationalItem, error)
	GetIssueAnalysisReport(ctx context.Context, startDate, endDate time.Time) ([]*IssueAnalysisItem, error)
	GetAILearningReportStats(ctx context.Context) (*AILearningReportStats, error)

	// System Errors (Auto-cleanup > 30 days)
	CreateSystemError(ctx context.Context, errRecord *SystemErrorRecord) (*SystemErrorRecord, error)
	ListSystemErrors(ctx context.Context) ([]*SystemErrorRecord, error)
	MarkSystemErrorHandled(ctx context.Context, id string) error
}

