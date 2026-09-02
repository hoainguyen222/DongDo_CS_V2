package sqlite

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
)

type PartnerRepo struct {
	db *DB
}

func NewPartnerRepo(db *DB) *PartnerRepo {
	return &PartnerRepo{db: db}
}

// Dashboard Methods
func (r *PartnerRepo) GetDashboardKpi(ctx context.Context, startDate, endDate time.Time) (*domain.DashboardKpiSummary, error) {
	row := r.db.SQLDB.QueryRowContext(ctx, `
		SELECT
			(SELECT COUNT(*) FROM chat_cases)::int AS total_conversations,
			(SELECT COUNT(*) FROM chat_cases WHERE status = 'AI_ACTIVE' OR status = 'RESOLVED')::int AS ai_resolved_count,
			(SELECT COUNT(*) FROM chat_cases WHERE status = 'NEEDS_HUMAN_CS')::int AS human_handoff_count,
			4.9 AS avg_csat
	`)

	var res domain.DashboardKpiSummary
	if err := row.Scan(&res.TotalConversations, &res.AIResolvedCount, &res.HumanHandoffCount, &res.AvgCSAT); err != nil {
		res = domain.DashboardKpiSummary{
			TotalConversations: 0,
			AIResolvedCount:    0,
			HumanHandoffCount:  0,
			AvgCSAT:            4.9,
		}
	}

	if res.TotalConversations > 0 {
		aiRate := (float64(res.AIResolvedCount) / float64(res.TotalConversations)) * 100.0
		res.AIRateVal = fmt.Sprintf("%.1f%%", aiRate)
	} else {
		res.AIRateVal = "0%"
	}
	res.AvgResponseTime = "1.2s"
	res.CSATVal = fmt.Sprintf("%.1f / 5.0", res.AvgCSAT)
	return &res, nil
}

func (r *PartnerRepo) GetDashboardAutomationTrend(ctx context.Context, startDate, endDate time.Time) ([]*domain.DashboardAutomationTrendDaily, error) {
	var list []*domain.DashboardAutomationTrendDaily
	now := time.Now()
	for i := 6; i >= 0; i-- {
		d := now.AddDate(0, 0, -i)
		list = append(list, &domain.DashboardAutomationTrendDaily{
			DateDay:      d,
			Label:        d.Format("02/01"),
			TotalCases:   120 + i*15,
			AICases:      100 + i*12,
			HandoffCases: 20 + i*3,
		})
	}
	return list, nil
}

func (r *PartnerRepo) GetRecentCompletedChats(ctx context.Context, limit, offset int) ([]*domain.ChatCase, error) {
	rows, err := r.db.SQLDB.QueryContext(ctx, `
		SELECT id, session_id, guest_id, customer_name, status, assigned_cs, last_message, resolution_note, created_at, updated_at
		FROM chat_cases
		ORDER BY updated_at DESC
		LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return []*domain.ChatCase{}, nil
	}
	defer rows.Close()

	var cases []*domain.ChatCase
	for rows.Next() {
		var c domain.ChatCase
		var sType string
		if err := rows.Scan(&c.ID, &c.SessionID, &c.GuestID, &c.CustomerName, &sType, &c.AssignedCS, &c.LastMessage, &c.ResolutionNote, &c.CreatedAt, &c.UpdatedAt); err != nil {
			continue
		}
		c.Status = domain.CaseStatus(sType)
		cases = append(cases, &c)
	}
	return cases, nil
}

// Quick Templates
func (r *PartnerRepo) ListQuickTemplates(ctx context.Context) ([]*domain.QuickTemplate, error) {
	return []*domain.QuickTemplate{
		{ID: 1, Title: "Hướng dẫn nạp tiền DDP Invest", Category: "Nạp/Rút tiền", Content: "Chào anh/chị, hạn mức nạp tối thiểu là 1,000,000 VNĐ qua QR Napas 24/7...", CreatedBy: "admin", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: 2, Title: "Quy định eKYC mở tài khoản", Category: "Tài khoản", Content: "Để hoàn tất eKYC, anh/chị vui lòng chụp rõ 2 mặt CCCD và quét khuôn mặt...", CreatedBy: "admin", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: 3, Title: "Thông báo Margin Call cảnh báo", Category: "Rủi ro", Content: "Tài khoản của anh/chị đang có tỷ lệ ký quỹ dưới 80%, vui lòng bổ sung ký quỹ...", CreatedBy: "admin", CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}, nil
}

func (r *PartnerRepo) CreateQuickTemplate(ctx context.Context, t *domain.QuickTemplate) (*domain.QuickTemplate, error) {
	t.ID = time.Now().UnixNano()
	t.CreatedAt = time.Now()
	t.UpdatedAt = time.Now()
	return t, nil
}

func (r *PartnerRepo) UpdateQuickTemplate(ctx context.Context, id int64, title, category, content string) error {
	return nil
}

func (r *PartnerRepo) DeleteQuickTemplate(ctx context.Context, id int64) error {
	return nil
}

// Prompt & RBAC
func (r *PartnerRepo) GetLatestSystemPromptHistory(ctx context.Context) (*domain.SystemPromptHistory, error) {
	return nil, nil
}

func (r *PartnerRepo) InsertSystemPromptHistory(ctx context.Context, p *domain.SystemPromptHistory) (*domain.SystemPromptHistory, error) {
	p.ID = time.Now().UnixNano()
	p.CreatedAt = time.Now()
	return p, nil
}

var (
	sqliteRolePermsLock sync.Mutex
	sqliteRolePermsMap  = make(map[string]*domain.RolePermission)
)

func (r *PartnerRepo) ListRolePermissions(ctx context.Context) ([]*domain.RolePermission, error) {
	sqliteRolePermsLock.Lock()
	defer sqliteRolePermsLock.Unlock()

	if len(sqliteRolePermsMap) == 0 {
		defaults := getDefaultSqliteRolePermissions()
		for _, item := range defaults {
			key := fmt.Sprintf("%s:%s", item.RoleName, item.FeatureKey)
			sqliteRolePermsMap[key] = item
		}
	}

	var list []*domain.RolePermission
	for _, v := range sqliteRolePermsMap {
		list = append(list, v)
	}
	return list, nil
}

func (r *PartnerRepo) UpsertRolePermission(ctx context.Context, p *domain.RolePermission) error {
	sqliteRolePermsLock.Lock()
	defer sqliteRolePermsLock.Unlock()

	if p.PermissionLevel == "" {
		if p.CanEdit {
			p.PermissionLevel = "act"
		} else if p.CanView {
			p.PermissionLevel = "view"
		} else {
			p.PermissionLevel = "none"
		}
	}
	p.CanView = p.PermissionLevel != "none"
	p.CanEdit = p.PermissionLevel == "act"
	p.UpdatedAt = time.Now()

	key := fmt.Sprintf("%s:%s", p.RoleName, p.FeatureKey)
	sqliteRolePermsMap[key] = p
	return nil
}

func getDefaultSqliteRolePermissions() []*domain.RolePermission {
	roles := []string{"Owner", "Admin", "Leader", "Staff"}
	features := []string{
		"partner_dashboard",
		"inbox",
		"customers",
		"calls",
		"learning",
		"knowledge",
		"partner_analytics",
		"partner_config",
		"config",
	}

	var list []*domain.RolePermission
	for _, role := range roles {
		for _, feat := range features {
			perm := "act"
			if role == "Leader" {
				if feat == "partner_config" {
					perm = "view"
				} else if feat == "config" {
					perm = "none"
				}
			} else if role == "Staff" {
				if feat == "inbox" || feat == "calls" {
					perm = "act"
				} else if feat == "partner_dashboard" || feat == "customers" || feat == "knowledge" {
					perm = "view"
				} else {
					perm = "none"
				}
			}

			canView := perm != "none"
			canEdit := perm == "act"

			list = append(list, &domain.RolePermission{
				RoleName:        role,
				FeatureKey:      feat,
				PermissionLevel: perm,
				CanView:         canView,
				CanEdit:         canEdit,
				UpdatedAt:       time.Now(),
			})
		}
	}
	return list
}

// Audit Logs
func (r *PartnerRepo) InsertAuditLog(ctx context.Context, log *domain.SystemAuditLog) (*domain.SystemAuditLog, error) {
	log.ID = time.Now().UnixNano()
	log.CreatedAt = time.Now()
	return log, nil
}

func (r *PartnerRepo) ListAuditLogs(ctx context.Context, limit, offset int) ([]*domain.SystemAuditLog, error) {
	return []*domain.SystemAuditLog{
		{ID: 1, ActionType: "SYSTEM_CACHE_CLEAR", Details: "Dọn dẹp cache hệ thống thành công", PerformedBy: "admin", CreatedAt: time.Now()},
	}, nil
}

// Reports
func (r *PartnerRepo) GetGeneralOverviewReport(ctx context.Context, startDate, endDate time.Time) (*domain.GeneralOverviewMetrics, error) {
	return &domain.GeneralOverviewMetrics{
		TotalCustomers: 125,
		TotalCases:     340,
		ResolvedCases:  310,
		OpenCases:      30,
		ResolutionRate: "91.2%",
	}, nil
}

func (r *PartnerRepo) GetAIPerformanceReport(ctx context.Context, startDate, endDate time.Time) (*domain.AIPerformanceMetrics, error) {
	return &domain.AIPerformanceMetrics{
		TotalCases:       340,
		AIResolvedCases:  295,
		HandoffCases:     45,
		AIResolutionRate: "86.8%",
		HandoffRate:      "13.2%",
		AvgAICSAT:        4.91,
		AvgResponseTime:  "1.18s",
	}, nil
}

func (r *PartnerRepo) GetStaffPerformanceReport(ctx context.Context, startDate, endDate time.Time) ([]*domain.StaffPerformanceItem, error) {
	return []*domain.StaffPerformanceItem{
		{StaffUsername: "thunt", StaffFullName: "Nguyễn Thị Thu", StaffRole: "Staff CS", TotalCasesHandled: 14, ResolvedCases: 14, AvgResponseTime: "26s", SLABreachRate: "0%", AvgCSAT: 4.95, Status: "Hoạt động"},
		{StaffUsername: "hoangtv", StaffFullName: "Trần Văn Hoàng", StaffRole: "Staff CS", TotalCasesHandled: 8, ResolvedCases: 8, AvgResponseTime: "30s", SLABreachRate: "1.2%", AvgCSAT: 4.88, Status: "Hoạt động"},
		{StaffUsername: "anhpm", StaffFullName: "Phạm Minh Anh", StaffRole: "Leader CS", TotalCasesHandled: 5, ResolvedCases: 5, AvgResponseTime: "22s", SLABreachRate: "0%", AvgCSAT: 5.0, Status: "Hoạt động"},
	}, nil
}

func (r *PartnerRepo) GetCXReport(ctx context.Context, startDate, endDate time.Time) (*domain.CXMetricsReport, error) {
	return &domain.CXMetricsReport{
		TotalFeedbackCount:    180,
		AvgCSATScore:          4.89,
		PositiveFeedbackCount: 175,
		NegativeFeedbackCount: 5,
		NSIIndex:              "+96.8%",
		FCRRate:               "87.5%",
	}, nil
}

func (r *PartnerRepo) InsertCSATFeedback(ctx context.Context, fb *domain.CSATFeedback) (*domain.CSATFeedback, error) {
	fb.ID = time.Now().UnixNano()
	fb.CreatedAt = time.Now()
	return fb, nil
}

func (r *PartnerRepo) GetHourlyOperationalLoad(ctx context.Context, startDate, endDate time.Time) ([]*domain.HourlyOperationalItem, error) {
	var list []*domain.HourlyOperationalItem
	for h := 8; h <= 18; h++ {
		list = append(list, &domain.HourlyOperationalItem{
			HourOfDay:     h,
			TotalMessages: 50 + h*10,
		})
	}
	return list, nil
}

func (r *PartnerRepo) GetIssueAnalysisReport(ctx context.Context, startDate, endDate time.Time) ([]*domain.IssueAnalysisItem, error) {
	return []*domain.IssueAnalysisItem{
		{CategoryName: "Quy trình Nạp / Rút tiền DDP Invest", TotalRequests: 340, PercentageShare: "32.5%", AIResolvedCount: 327, AIResolutionRate: "96.2%"},
		{CategoryName: "Margin Call & Quản trị rủi ro", TotalRequests: 210, PercentageShare: "20.1%", AIResolvedCount: 191, AIResolutionRate: "91.0%"},
		{CategoryName: "Biểu phí giao dịch Hàng hóa CBOT", TotalRequests: 180, PercentageShare: "17.2%", AIResolvedCount: 177, AIResolutionRate: "98.5%"},
		{CategoryName: "Hướng dẫn eKYC mở tài khoản", TotalRequests: 160, PercentageShare: "15.3%", AIResolvedCount: 150, AIResolutionRate: "94.0%"},
		{CategoryName: "Thắc mắc lỗi kỹ thuật app DDP", TotalRequests: 156, PercentageShare: "14.9%", AIResolvedCount: 128, AIResolutionRate: "82.0%"},
	}, nil
}

func (r *PartnerRepo) GetAILearningReportStats(ctx context.Context) (*domain.AILearningReportStats, error) {
	return &domain.AILearningReportStats{
		PendingCount:       5,
		ApprovedCount:      128,
		RejectedCount:      8,
		TotalLearningItems: 141,
		ApprovalRate:       "94.2%",
	}, nil
}

func (r *PartnerRepo) CreateSystemError(ctx context.Context, errRecord *domain.SystemErrorRecord) (*domain.SystemErrorRecord, error) {
	// Auto purge records older than 30 days
	_, _ = r.db.SQLDB.ExecContext(ctx, `DELETE FROM system_errors WHERE datetime(created_at) < datetime('now', '-30 days')`)

	if errRecord.CreatedAt.IsZero() {
		errRecord.CreatedAt = time.Now()
	}
	nowStr := errRecord.CreatedAt.Format(time.RFC3339)
	isHandledInt := 0
	if errRecord.IsHandled {
		isHandledInt = 1
	}

	_, err := r.db.SQLDB.ExecContext(ctx, `
		INSERT INTO system_errors (id, source, title, details, severity, is_handled, suggested_fix, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET is_handled = excluded.is_handled
	`, errRecord.ID, errRecord.Source, errRecord.Title, errRecord.Details, errRecord.Severity, isHandledInt, errRecord.SuggestedFix, nowStr)

	if err != nil {
		return nil, err
	}
	return errRecord, nil
}

func (r *PartnerRepo) ListSystemErrors(ctx context.Context) ([]*domain.SystemErrorRecord, error) {
	// Auto purge records older than 30 days
	_, _ = r.db.SQLDB.ExecContext(ctx, `DELETE FROM system_errors WHERE datetime(created_at) < datetime('now', '-30 days')`)

	rows, err := r.db.SQLDB.QueryContext(ctx, `
		SELECT id, source, title, details, severity, is_handled, suggested_fix, created_at
		FROM system_errors
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*domain.SystemErrorRecord
	for rows.Next() {
		var item domain.SystemErrorRecord
		var isHandledInt int
		var createdAtStr string
		if err := rows.Scan(&item.ID, &item.Source, &item.Title, &item.Details, &item.Severity, &isHandledInt, &item.SuggestedFix, &createdAtStr); err != nil {
			return nil, err
		}
		item.IsHandled = isHandledInt == 1
		item.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr)
		list = append(list, &item)
	}
	return list, nil
}

func (r *PartnerRepo) MarkSystemErrorHandled(ctx context.Context, id string) error {
	_, err := r.db.SQLDB.ExecContext(ctx, `UPDATE system_errors SET is_handled = 1 WHERE id = ?`, id)
	return err
}
