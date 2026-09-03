package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
	chatdb "github.com/hoainguyen222/DongDo_CS_V2/internal/repository/sqlcdb/chat"
	partnerdb "github.com/hoainguyen222/DongDo_CS_V2/internal/repository/sqlcdb/partner"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// PartnerRepo implements domain.PartnerRepository using sqlc-generated partner queries.
type PartnerRepo struct {
	db *DB
}

// NewPartnerRepo constructs a PartnerRepo using the shared DB handle.
func NewPartnerRepo(db *DB) *PartnerRepo {
	return &PartnerRepo{db: db}
}

// ============================================================
// Dashboard
// ============================================================

func (r *PartnerRepo) GetDashboardKpi(ctx context.Context, startDate, endDate time.Time) (*domain.DashboardKpiSummary, error) {
	row, err := r.db.Partner.GetDashboardKpiSummary(ctx, partnerdb.GetDashboardKpiSummaryParams{
		CreatedAt:   startDate,
		CreatedAt_2: endDate,
	})
	if err != nil {
		return nil, err
	}

	summary := &domain.DashboardKpiSummary{
		TotalConversations: int(row.TotalConversations),
		AIResolvedCount:    int(row.AiResolvedCount),
		HumanHandoffCount:  int(row.HumanHandoffCount),
		AvgCSAT:            row.AvgCsat,
	}

	if row.TotalConversations > 0 {
		aiRate := (float64(row.AiResolvedCount) / float64(row.TotalConversations)) * 100.0
		summary.AIRateVal = fmt.Sprintf("%.1f%%", aiRate)
	} else {
		summary.AIRateVal = "0%"
	}

	summary.AvgResponseTime = "1.2s"
	summary.CSATVal = fmt.Sprintf("%.1f / 5.0", row.AvgCsat)
	return summary, nil
}

func (r *PartnerRepo) GetDashboardAutomationTrend(ctx context.Context, startDate, endDate time.Time) ([]*domain.DashboardAutomationTrendDaily, error) {
	rows, err := r.db.Partner.GetDashboardAutomationTrendDaily(ctx, partnerdb.GetDashboardAutomationTrendDailyParams{
		CreatedAt:   startDate,
		CreatedAt_2: endDate,
	})
	if err != nil {
		return nil, err
	}

	list := make([]*domain.DashboardAutomationTrendDaily, 0, len(rows))
	for _, row := range rows {
		list = append(list, &domain.DashboardAutomationTrendDaily{
			DateDay:      row.DateDay,
			Label:        row.DateDay.Format("02/01"),
			TotalCases:   int(row.TotalCases),
			AICases:      int(row.AiCases),
			HandoffCases: int(row.HandoffCases),
		})
	}
	return list, nil
}

func (r *PartnerRepo) GetRecentCompletedChats(ctx context.Context, limit, offset int) ([]*domain.ChatCase, error) {
	rows, err := r.db.Chat.GetRecentCompletedChats(ctx, chatdb.GetRecentCompletedChatsParams{
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, err
	}

	list := make([]*domain.ChatCase, 0, len(rows))
	for _, row := range rows {
		c := &domain.ChatCase{
			ID:             row.ID,
			SessionID:      row.SessionID,
			CustomerName:   row.CustomerName,
			CustomerPhone:  row.CustomerPhone,
			Status:         row.Status,
			CreatedAt:      row.CreatedAt,
			UpdatedAt:      row.UpdatedAt,
		}
		if row.GuestID.Valid {
			id := uuid.UUID(row.GuestID.Bytes)
			c.GuestID = &id
		}
		if row.AssignedCs.Valid {
			c.AssignedCS = row.AssignedCs.String
		}
		if row.LastMessage.Valid {
			c.LastMessage = row.LastMessage.String
		}
		if row.ResolutionNote.Valid {
			c.ResolutionNote = row.ResolutionNote.String
		}
		list = append(list, c)
	}
	return list, nil
}

// ============================================================
// Quick Templates
// ============================================================

func (r *PartnerRepo) ListQuickTemplates(ctx context.Context) ([]*domain.QuickTemplate, error) {
	rows, err := r.db.Partner.ListQuickTemplates(ctx)
	if err != nil {
		return nil, err
	}

	list := make([]*domain.QuickTemplate, 0, len(rows))
	for _, row := range rows {
		list = append(list, &domain.QuickTemplate{
			ID:        row.ID,
			Title:     row.Title,
			Category:  row.Category,
			Content:   row.Content,
			CreatedBy: textToString(row.CreatedBy),
			CreatedAt: row.CreatedAt,
			UpdatedAt: row.UpdatedAt,
		})
	}
	return list, nil
}

func (r *PartnerRepo) CreateQuickTemplate(ctx context.Context, t *domain.QuickTemplate) (*domain.QuickTemplate, error) {
	row, err := r.db.Partner.CreateQuickTemplate(ctx, partnerdb.CreateQuickTemplateParams{
		Title:     t.Title,
		Category:  t.Category,
		Content:   t.Content,
		CreatedBy: textFromString(t.CreatedBy),
	})
	if err != nil {
		return nil, err
	}
	return &domain.QuickTemplate{
		ID:        row.ID,
		Title:     row.Title,
		Category:  row.Category,
		Content:   row.Content,
		CreatedBy: textToString(row.CreatedBy),
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}, nil
}

func (r *PartnerRepo) UpdateQuickTemplate(ctx context.Context, id int64, title, category, content string) error {
	return r.db.Partner.UpdateQuickTemplate(ctx, partnerdb.UpdateQuickTemplateParams{
		Title:    title,
		Category: category,
		Content:  content,
		ID:       id,
	})
}

func (r *PartnerRepo) DeleteQuickTemplate(ctx context.Context, id int64) error {
	return r.db.Partner.DeleteQuickTemplate(ctx, id)
}

// ============================================================
// Prompt History
// ============================================================

func (r *PartnerRepo) GetLatestSystemPromptHistory(ctx context.Context) (*domain.SystemPromptHistory, error) {
	row, err := r.db.Partner.GetLatestSystemPrompt(ctx)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	var temp float64
	if v, convErr := row.Temperature.Float64Value(); convErr == nil {
		temp = v.Float64
	}
	return &domain.SystemPromptHistory{
		ID:           row.ID,
		SystemPrompt: row.SystemPrompt,
		LLMModel:     row.LlmModel,
		Temperature:  temp,
		CreatedBy:    textToString(row.CreatedBy),
		CreatedAt:    row.CreatedAt,
	}, nil
}

func (r *PartnerRepo) InsertSystemPromptHistory(ctx context.Context, p *domain.SystemPromptHistory) (*domain.SystemPromptHistory, error) {
	row, err := r.db.Partner.InsertSystemPromptHistory(ctx, partnerdb.InsertSystemPromptHistoryParams{
		SystemPrompt: p.SystemPrompt,
		LlmModel:    p.LLMModel,
		Temperature: numericFromFloat64(p.Temperature),
		CreatedBy:   textFromString(p.CreatedBy),
	})
	if err != nil {
		return nil, err
	}
	var temp float64
	if v, convErr := row.Temperature.Float64Value(); convErr == nil {
		temp = v.Float64
	}
	return &domain.SystemPromptHistory{
		ID:           row.ID,
		SystemPrompt: row.SystemPrompt,
		LLMModel:     row.LlmModel,
		Temperature:  temp,
		CreatedBy:    textToString(row.CreatedBy),
		CreatedAt:    row.CreatedAt,
	}, nil
}

// ============================================================
// RBAC — Role Permissions
// ============================================================

func (r *PartnerRepo) ListRolePermissions(ctx context.Context) ([]*domain.RolePermission, error) {
	rows, err := r.db.Partner.ListRolePermissions(ctx)
	if err != nil {
		return nil, err
	}

	if len(rows) == 0 {
		if seedErr := r.seedDefaultRolePermissions(ctx); seedErr != nil {
			return nil, seedErr
		}
		rows, err = r.db.Partner.ListRolePermissions(ctx)
		if err != nil {
			return nil, err
		}
	}

	list := make([]*domain.RolePermission, 0, len(rows))
	for _, row := range rows {
		list = append(list, &domain.RolePermission{
			ID:              row.ID,
			RoleName:        row.RoleName,
			FeatureKey:      row.FeatureKey,
			PermissionLevel: row.PermissionLevel,
			CanView:         row.CanView,
			CanEdit:         row.CanEdit,
			UpdatedAt:       row.UpdatedAt,
		})
	}
	return list, nil
}

func (r *PartnerRepo) UpsertRolePermission(ctx context.Context, p *domain.RolePermission) error {
	permLevel := p.PermissionLevel
	if permLevel == "" {
		if p.CanEdit {
			permLevel = "act"
		} else if p.CanView {
			permLevel = "view"
		} else {
			permLevel = "none"
		}
	}
	canView := permLevel != "none"
	canEdit := permLevel == "act"

	return r.db.Partner.UpsertRolePermission(ctx, partnerdb.UpsertRolePermissionParams{
		RoleName:        p.RoleName,
		FeatureKey:      p.FeatureKey,
		PermissionLevel: permLevel,
		CanView:         canView,
		CanEdit:         canEdit,
	})
}

func (r *PartnerRepo) seedDefaultRolePermissions(ctx context.Context) error {
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

			if err := r.db.Partner.UpsertRolePermission(ctx, partnerdb.UpsertRolePermissionParams{
				RoleName:        role,
				FeatureKey:      feat,
				PermissionLevel: perm,
				CanView:         canView,
				CanEdit:         canEdit,
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

// ============================================================
// Audit Logs
// ============================================================

func (r *PartnerRepo) InsertAuditLog(ctx context.Context, log *domain.SystemAuditLog) (*domain.SystemAuditLog, error) {
	row, err := r.db.Partner.InsertAuditLog(ctx, partnerdb.InsertAuditLogParams{
		ActionType:  log.ActionType,
		Details:     textFromString(log.Details),
		PerformedBy: textFromString(log.PerformedBy),
	})
	if err != nil {
		return nil, err
	}
	return &domain.SystemAuditLog{
		ID:          row.ID,
		ActionType:  row.ActionType,
		Details:     textToString(row.Details),
		PerformedBy: textToString(row.PerformedBy),
		CreatedAt:   row.CreatedAt,
	}, nil
}

func (r *PartnerRepo) ListAuditLogs(ctx context.Context, limit, offset int) ([]*domain.SystemAuditLog, error) {
	rows, err := r.db.Partner.ListAuditLogs(ctx, partnerdb.ListAuditLogsParams{
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, err
	}

	list := make([]*domain.SystemAuditLog, 0, len(rows))
	for _, row := range rows {
		list = append(list, &domain.SystemAuditLog{
			ID:          row.ID,
			ActionType:  row.ActionType,
			Details:     textToString(row.Details),
			PerformedBy: textToString(row.PerformedBy),
			CreatedAt:   row.CreatedAt,
		})
	}
	return list, nil
}

// ============================================================
// Reports — 7 sub-reports
// ============================================================

func (r *PartnerRepo) GetGeneralOverviewReport(ctx context.Context, startDate, endDate time.Time) (*domain.GeneralOverviewMetrics, error) {
	row, err := r.db.Partner.GetGeneralOverviewMetrics(ctx, partnerdb.GetGeneralOverviewMetricsParams{
		CreatedAt:   startDate,
		CreatedAt_2: endDate,
	})
	if err != nil {
		return nil, err
	}

	m := &domain.GeneralOverviewMetrics{
		TotalCustomers: int(row.TotalCustomers),
		TotalCases:     int(row.TotalCases),
		ResolvedCases:  int(row.ResolvedCases),
		OpenCases:      int(row.OpenCases),
	}

	if row.TotalCases > 0 {
		m.ResolutionRate = fmt.Sprintf("%.1f%%", (float64(row.ResolvedCases)/float64(row.TotalCases))*100.0)
	} else {
		m.ResolutionRate = "0%"
	}
	return m, nil
}

func (r *PartnerRepo) GetAIPerformanceReport(ctx context.Context, startDate, endDate time.Time) (*domain.AIPerformanceMetrics, error) {
	row, err := r.db.Partner.GetAIPerformanceMetrics(ctx, partnerdb.GetAIPerformanceMetricsParams{
		CreatedAt:   startDate,
		CreatedAt_2: endDate,
	})
	if err != nil {
		return nil, err
	}

	m := &domain.AIPerformanceMetrics{
		TotalCases:       int(row.TotalCases),
		AIResolvedCases:  int(row.AiResolvedCases),
		HandoffCases:     int(row.HandoffCases),
		AvgAICSAT:        row.AvgAiCsat,
		AvgResponseTime:  "1.18s",
	}

	if row.TotalCases > 0 {
		m.AIResolutionRate = fmt.Sprintf("%.1f%%", (float64(row.AiResolvedCases)/float64(row.TotalCases))*100.0)
		m.HandoffRate = fmt.Sprintf("%.1f%%", (float64(row.HandoffCases)/float64(row.TotalCases))*100.0)
	} else {
		m.AIResolutionRate = "0%"
		m.HandoffRate = "0%"
	}
	return m, nil
}

func (r *PartnerRepo) GetStaffPerformanceReport(ctx context.Context, startDate, endDate time.Time) ([]*domain.StaffPerformanceItem, error) {
	rows, err := r.db.Partner.GetStaffPerformanceReport(ctx, partnerdb.GetStaffPerformanceReportParams{
		CreatedAt:   startDate,
		CreatedAt_2: endDate,
	})
	if err != nil {
		return nil, err
	}

	list := make([]*domain.StaffPerformanceItem, 0, len(rows))
	for _, row := range rows {
		list = append(list, &domain.StaffPerformanceItem{
			StaffUsername:     row.StaffUsername,
			StaffFullName:    textToString(row.StaffFullName),
			StaffRole:        row.StaffRole,
			TotalCasesHandled: int(row.TotalCasesHandled),
			ResolvedCases:     int(row.ResolvedCases),
			AvgResponseTime:   "26s",
			SLABreachRate:     "0%",
			AvgCSAT:           row.AvgCsat,
			Status:            "Hoạt động",
		})
	}
	return list, nil
}

func (r *PartnerRepo) GetCXReport(ctx context.Context, startDate, endDate time.Time) (*domain.CXMetricsReport, error) {
	row, err := r.db.Partner.GetCXMetricsReport(ctx, partnerdb.GetCXMetricsReportParams{
		CreatedAt:   startDate,
		CreatedAt_2: endDate,
	})
	if err != nil {
		return nil, err
	}

	m := &domain.CXMetricsReport{
		TotalFeedbackCount:    int(row.TotalFeedbackCount),
		AvgCSATScore:          row.AvgCsatScore,
		PositiveFeedbackCount: int(row.PositiveFeedbackCount),
		NegativeFeedbackCount: int(row.NegativeFeedbackCount),
	}

	if row.TotalFeedbackCount > 0 {
		m.NSIIndex = fmt.Sprintf("%+.1f%%", (float64(row.PositiveFeedbackCount-row.NegativeFeedbackCount)/float64(row.TotalFeedbackCount))*100.0)
		m.FCRRate = fmt.Sprintf("%.1f%%", (float64(row.PositiveFeedbackCount)/float64(row.TotalFeedbackCount))*100.0)
	} else {
		m.NSIIndex = "0%"
		m.FCRRate = "0%"
	}
	return m, nil
}

func (r *PartnerRepo) InsertCSATFeedback(ctx context.Context, fb *domain.CSATFeedback) (*domain.CSATFeedback, error) {
	row, err := r.db.Partner.InsertCSATFeedback(ctx, partnerdb.InsertCSATFeedbackParams{
		SessionID:     fb.SessionID,
		Rating:        int32(fb.Rating),
		FeedbackText:  textFromString(fb.FeedbackText),
		TargetType:    fb.TargetType,
		StaffUsername: textFromString(fb.StaffUsername),
	})
	if err != nil {
		return nil, err
	}
	return &domain.CSATFeedback{
		ID:            row.ID,
		SessionID:     row.SessionID,
		Rating:        int(row.Rating),
		FeedbackText:  textToString(row.FeedbackText),
		TargetType:    row.TargetType,
		StaffUsername: textToString(row.StaffUsername),
		CreatedAt:    row.CreatedAt,
	}, nil
}

func (r *PartnerRepo) GetHourlyOperationalLoad(ctx context.Context, startDate, endDate time.Time) ([]*domain.HourlyOperationalItem, error) {
	rows, err := r.db.Partner.GetHourlyOperationalLoad(ctx, partnerdb.GetHourlyOperationalLoadParams{
		CreatedAt:   startDate,
		CreatedAt_2: endDate,
	})
	if err != nil {
		return nil, err
	}

	list := make([]*domain.HourlyOperationalItem, 0, len(rows))
	for _, row := range rows {
		list = append(list, &domain.HourlyOperationalItem{
			HourOfDay:     int(row.HourOfDay),
			TotalMessages: int(row.TotalMessages),
		})
	}
	return list, nil
}

func (r *PartnerRepo) GetIssueAnalysisReport(ctx context.Context, startDate, endDate time.Time) ([]*domain.IssueAnalysisItem, error) {
	rows, err := r.db.Partner.GetIssueAnalysisReport(ctx, partnerdb.GetIssueAnalysisReportParams{
		CreatedAt:   startDate,
		CreatedAt_2: endDate,
	})
	if err != nil {
		return nil, err
	}

	var grandTotal int
	for _, row := range rows {
		grandTotal += int(row.TotalRequests)
	}

	list := make([]*domain.IssueAnalysisItem, 0, len(rows))
	for _, row := range rows {
		item := &domain.IssueAnalysisItem{
			CategoryName:    row.CategoryName,
			TotalRequests:   int(row.TotalRequests),
			AIResolvedCount: int(row.AiResolvedCount),
		}
		if row.TotalRequests > 0 {
			item.AIResolutionRate = fmt.Sprintf("%.1f%%", (float64(row.AiResolvedCount)/float64(row.TotalRequests))*100.0)
		} else {
			item.AIResolutionRate = "0%"
		}
		if grandTotal > 0 {
			item.PercentageShare = fmt.Sprintf("%.1f%%", (float64(row.TotalRequests)/float64(grandTotal))*100.0)
		} else {
			item.PercentageShare = "0%"
		}
		list = append(list, item)
	}
	return list, nil
}

func (r *PartnerRepo) GetAILearningReportStats(ctx context.Context) (*domain.AILearningReportStats, error) {
	row, err := r.db.Partner.GetAILearningReportStats(ctx)
	if err != nil {
		return nil, err
	}

	m := &domain.AILearningReportStats{
		PendingCount:       int(row.PendingCount),
		ApprovedCount:      int(row.ApprovedCount),
		RejectedCount:      int(row.RejectedCount),
		TotalLearningItems: int(row.TotalLearningItems),
	}

	if row.TotalLearningItems > 0 {
		m.ApprovalRate = fmt.Sprintf("%.1f%%", (float64(row.ApprovedCount)/float64(row.TotalLearningItems))*100.0)
	} else {
		m.ApprovalRate = "0%"
	}
	return m, nil
}

// ============================================================
// System Errors
// ============================================================

func (r *PartnerRepo) CreateSystemError(ctx context.Context, errRecord *domain.SystemErrorRecord) (*domain.SystemErrorRecord, error) {
	if err := r.db.Partner.PurgeOldSystemErrors(ctx); err != nil {
		return nil, err
	}

	createdAt := errRecord.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}

	if err := r.db.Partner.UpsertSystemError(ctx, partnerdb.UpsertSystemErrorParams{
		ID:           errRecord.ID,
		Source:       errRecord.Source,
		Title:        errRecord.Title,
		Details:      textFromString(errRecord.Details),
		Severity:     errRecord.Severity,
		IsHandled:    errRecord.IsHandled,
		SuggestedFix: textFromString(errRecord.SuggestedFix),
		CreatedAt:    createdAt,
	}); err != nil {
		return nil, err
	}

	errRecord.CreatedAt = createdAt
	return errRecord, nil
}

func (r *PartnerRepo) ListSystemErrors(ctx context.Context) ([]*domain.SystemErrorRecord, error) {
	if err := r.db.Partner.PurgeOldSystemErrors(ctx); err != nil {
		return nil, err
	}

	rows, err := r.db.Partner.ListSystemErrors(ctx)
	if err != nil {
		return nil, err
	}

	list := make([]*domain.SystemErrorRecord, 0, len(rows))
	for _, row := range rows {
		list = append(list, &domain.SystemErrorRecord{
			ID:           row.ID,
			Source:       row.Source,
			Title:        row.Title,
			Details:      textToString(row.Details),
			Severity:     row.Severity,
			IsHandled:    row.IsHandled,
			SuggestedFix: textToString(row.SuggestedFix),
			CreatedAt:    row.CreatedAt,
		})
	}
	return list, nil
}

func (r *PartnerRepo) MarkSystemErrorHandled(ctx context.Context, id string) error {
	return r.db.Partner.MarkSystemErrorHandled(ctx, id)
}

// textToString converts a pgtype.Text to a Go string, returning "" when null.
func textToString(t pgtype.Text) string {
	if !t.Valid {
		return ""
	}
	return t.String
}

// textFromString wraps a Go string into a pgtype.Text for sqlc parameters.
// An empty string is treated as SQL NULL.
func textFromString(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: s != ""}
}

// numericFromFloat64 converts a Go float64 to a pgtype.Numeric by parsing
// the string representation via the underlying Numeric scanner.
func numericFromFloat64(v float64) pgtype.Numeric {
	n := &pgtype.Numeric{}
	_ = n.Scan(fmt.Sprintf("%.4f", v))
	if n.Valid {
		return *n
	}
	return pgtype.Numeric{Valid: false}
}


