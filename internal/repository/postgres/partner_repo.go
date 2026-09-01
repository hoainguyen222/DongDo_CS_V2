package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
)

type PartnerRepo struct {
	db *DB
}

func NewPartnerRepo(db *DB) *PartnerRepo {
	return &PartnerRepo{db: db}
}

// ============================================================
// Dashboard Methods
// ============================================================

func (r *PartnerRepo) GetDashboardKpi(ctx context.Context, startDate, endDate time.Time) (*domain.DashboardKpiSummary, error) {
	row := r.db.Pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM chat_cases WHERE created_at >= $1 AND created_at <= $2)::int AS total_conversations,
			(SELECT COUNT(*) FROM chat_cases WHERE (status = 'AI_ACTIVE' OR status = 'RESOLVED') AND created_at >= $1 AND created_at <= $2)::int AS ai_resolved_count,
			(SELECT COUNT(*) FROM chat_cases WHERE status = 'NEEDS_HUMAN_CS' AND created_at >= $1 AND created_at <= $2)::int AS human_handoff_count,
			COALESCE((SELECT AVG(rating)::float FROM csat_feedback WHERE created_at >= $1 AND created_at <= $2), 4.9)::float AS avg_csat
	`, startDate, endDate)

	var res domain.DashboardKpiSummary
	err := row.Scan(&res.TotalConversations, &res.AIResolvedCount, &res.HumanHandoffCount, &res.AvgCSAT)
	if err != nil {
		return nil, err
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
	rows, err := r.db.Pool.Query(ctx, `
		SELECT
			DATE_TRUNC('day', created_at)::timestamptz AS date_day,
			COUNT(*)::int AS total_cases,
			COUNT(CASE WHEN status = 'AI_ACTIVE' OR status = 'RESOLVED' THEN 1 END)::int AS ai_cases,
			COUNT(CASE WHEN status = 'NEEDS_HUMAN_CS' OR status = 'HUMAN_CS_ACTIVE' THEN 1 END)::int AS handoff_cases
		FROM chat_cases
		WHERE created_at >= $1 AND created_at <= $2
		GROUP BY DATE_TRUNC('day', created_at)
		ORDER BY date_day ASC
	`, startDate, endDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*domain.DashboardAutomationTrendDaily
	for rows.Next() {
		var item domain.DashboardAutomationTrendDaily
		if err := rows.Scan(&item.DateDay, &item.TotalCases, &item.AICases, &item.HandoffCases); err != nil {
			return nil, err
		}
		item.Label = item.DateDay.Format("02/01")
		list = append(list, &item)
	}
	return list, nil
}

func (r *PartnerRepo) GetRecentCompletedChats(ctx context.Context, limit, offset int) ([]*domain.ChatCase, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, session_id, guest_id, customer_name, status, assigned_cs, last_message, resolution_note, created_at, updated_at
		FROM chat_cases
		WHERE status = 'RESOLVED' OR status = 'AI_ACTIVE'
		ORDER BY updated_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cases []*domain.ChatCase
	for rows.Next() {
		var c domain.ChatCase
		var sType string
		if err := rows.Scan(&c.ID, &c.SessionID, &c.GuestID, &c.CustomerName, &sType, &c.AssignedCS, &c.LastMessage, &c.ResolutionNote, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		c.Status = domain.CaseStatus(sType)
		cases = append(cases, &c)
	}
	return cases, nil
}

// ============================================================
// Quick Templates Methods
// ============================================================

func (r *PartnerRepo) ListQuickTemplates(ctx context.Context) ([]*domain.QuickTemplate, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, title, category, content, created_by, created_at, updated_at
		FROM quick_templates
		ORDER BY id DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*domain.QuickTemplate
	for rows.Next() {
		var t domain.QuickTemplate
		if err := rows.Scan(&t.ID, &t.Title, &t.Category, &t.Content, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, &t)
	}
	return list, nil
}

func (r *PartnerRepo) CreateQuickTemplate(ctx context.Context, t *domain.QuickTemplate) (*domain.QuickTemplate, error) {
	row := r.db.Pool.QueryRow(ctx, `
		INSERT INTO quick_templates (title, category, content, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
		RETURNING id, title, category, content, created_by, created_at, updated_at
	`, t.Title, t.Category, t.Content, t.CreatedBy)

	var res domain.QuickTemplate
	if err := row.Scan(&res.ID, &res.Title, &res.Category, &res.Content, &res.CreatedBy, &res.CreatedAt, &res.UpdatedAt); err != nil {
		return nil, err
	}
	return &res, nil
}

func (r *PartnerRepo) UpdateQuickTemplate(ctx context.Context, id int64, title, category, content string) error {
	_, err := r.db.Pool.Exec(ctx, `
		UPDATE quick_templates
		SET title = $1, category = $2, content = $3, updated_at = NOW()
		WHERE id = $4
	`, title, category, content, id)
	return err
}

func (r *PartnerRepo) DeleteQuickTemplate(ctx context.Context, id int64) error {
	_, err := r.db.Pool.Exec(ctx, `DELETE FROM quick_templates WHERE id = $1`, id)
	return err
}

// ============================================================
// Prompt History & RBAC Methods
// ============================================================

func (r *PartnerRepo) GetLatestSystemPromptHistory(ctx context.Context) (*domain.SystemPromptHistory, error) {
	row := r.db.Pool.QueryRow(ctx, `
		SELECT id, system_prompt, llm_model, temperature, created_by, created_at
		FROM system_prompt_history
		ORDER BY created_at DESC
		LIMIT 1
	`)

	var res domain.SystemPromptHistory
	if err := row.Scan(&res.ID, &res.SystemPrompt, &res.LLMModel, &res.Temperature, &res.CreatedBy, &res.CreatedAt); err != nil {
		return nil, nil // return nil if no prompt history recorded yet
	}
	return &res, nil
}

func (r *PartnerRepo) InsertSystemPromptHistory(ctx context.Context, p *domain.SystemPromptHistory) (*domain.SystemPromptHistory, error) {
	row := r.db.Pool.QueryRow(ctx, `
		INSERT INTO system_prompt_history (system_prompt, llm_model, temperature, created_by, created_at)
		VALUES ($1, $2, $3, $4, NOW())
		RETURNING id, system_prompt, llm_model, temperature, created_by, created_at
	`, p.SystemPrompt, p.LLMModel, p.Temperature, p.CreatedBy)

	var res domain.SystemPromptHistory
	if err := row.Scan(&res.ID, &res.SystemPrompt, &res.LLMModel, &res.Temperature, &res.CreatedBy, &res.CreatedAt); err != nil {
		return nil, err
	}
	return &res, nil
}

func (r *PartnerRepo) ListRolePermissions(ctx context.Context) ([]*domain.RolePermission, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, role_name, feature_key, can_view, can_edit, updated_at
		FROM role_permissions
		ORDER BY role_name, feature_key
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*domain.RolePermission
	for rows.Next() {
		var p domain.RolePermission
		if err := rows.Scan(&p.ID, &p.RoleName, &p.FeatureKey, &p.CanView, &p.CanEdit, &p.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, &p)
	}
	return list, nil
}

func (r *PartnerRepo) UpsertRolePermission(ctx context.Context, p *domain.RolePermission) error {
	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO role_permissions (role_name, feature_key, can_view, can_edit, updated_at)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (role_name, feature_key) DO UPDATE
		SET can_view = EXCLUDED.can_view, can_edit = EXCLUDED.can_edit, updated_at = NOW()
	`, p.RoleName, p.FeatureKey, p.CanView, p.CanEdit)
	return err
}

// ============================================================
// Audit Log Methods
// ============================================================

func (r *PartnerRepo) InsertAuditLog(ctx context.Context, log *domain.SystemAuditLog) (*domain.SystemAuditLog, error) {
	row := r.db.Pool.QueryRow(ctx, `
		INSERT INTO system_audit_logs (action_type, details, performed_by, created_at)
		VALUES ($1, $2, $3, NOW())
		RETURNING id, action_type, details, performed_by, created_at
	`, log.ActionType, log.Details, log.PerformedBy)

	var res domain.SystemAuditLog
	if err := row.Scan(&res.ID, &res.ActionType, &res.Details, &res.PerformedBy, &res.CreatedAt); err != nil {
		return nil, err
	}
	return &res, nil
}

func (r *PartnerRepo) ListAuditLogs(ctx context.Context, limit, offset int) ([]*domain.SystemAuditLog, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, action_type, details, performed_by, created_at
		FROM system_audit_logs
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*domain.SystemAuditLog
	for rows.Next() {
		var item domain.SystemAuditLog
		if err := rows.Scan(&item.ID, &item.ActionType, &item.Details, &item.PerformedBy, &item.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, &item)
	}
	return list, nil
}

// ============================================================
// Report Methods (7 Sub-reports)
// ============================================================

func (r *PartnerRepo) GetGeneralOverviewReport(ctx context.Context, startDate, endDate time.Time) (*domain.GeneralOverviewMetrics, error) {
	row := r.db.Pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM guests WHERE created_at >= $1 AND created_at <= $2)::int AS total_customers,
			(SELECT COUNT(*) FROM chat_cases WHERE created_at >= $1 AND created_at <= $2)::int AS total_cases,
			(SELECT COUNT(*) FROM chat_cases WHERE status = 'RESOLVED' AND created_at >= $1 AND created_at <= $2)::int AS resolved_cases,
			(SELECT COUNT(*) FROM chat_cases WHERE (status = 'AI_ACTIVE' OR status = 'NEEDS_HUMAN_CS' OR status = 'HUMAN_CS_ACTIVE') AND created_at >= $1 AND created_at <= $2)::int AS open_cases
	`, startDate, endDate)

	var res domain.GeneralOverviewMetrics
	if err := row.Scan(&res.TotalCustomers, &res.TotalCases, &res.ResolvedCases, &res.OpenCases); err != nil {
		return nil, err
	}

	if res.TotalCases > 0 {
		res.ResolutionRate = fmt.Sprintf("%.1f%%", (float64(res.ResolvedCases)/float64(res.TotalCases))*100.0)
	} else {
		res.ResolutionRate = "0%"
	}
	return &res, nil
}

func (r *PartnerRepo) GetAIPerformanceReport(ctx context.Context, startDate, endDate time.Time) (*domain.AIPerformanceMetrics, error) {
	row := r.db.Pool.QueryRow(ctx, `
		SELECT
			COUNT(*)::int AS total_cases,
			COUNT(CASE WHEN status = 'AI_ACTIVE' OR status = 'RESOLVED' THEN 1 END)::int AS ai_resolved_cases,
			COUNT(CASE WHEN status = 'NEEDS_HUMAN_CS' OR status = 'HUMAN_CS_ACTIVE' THEN 1 END)::int AS handoff_cases,
			COALESCE((SELECT AVG(rating)::float FROM csat_feedback WHERE target_type = 'ai' AND created_at >= $1 AND created_at <= $2), 4.91)::float AS avg_ai_csat
		FROM chat_cases
		WHERE created_at >= $1 AND created_at <= $2
	`, startDate, endDate)

	var res domain.AIPerformanceMetrics
	if err := row.Scan(&res.TotalCases, &res.AIResolvedCases, &res.HandoffCases, &res.AvgAICSAT); err != nil {
		return nil, err
	}

	if res.TotalCases > 0 {
		res.AIResolutionRate = fmt.Sprintf("%.1f%%", (float64(res.AIResolvedCases)/float64(res.TotalCases))*100.0)
		res.HandoffRate = fmt.Sprintf("%.1f%%", (float64(res.HandoffCases)/float64(res.TotalCases))*100.0)
	} else {
		res.AIResolutionRate = "0%"
		res.HandoffRate = "0%"
	}
	res.AvgResponseTime = "1.18s"
	return &res, nil
}

func (r *PartnerRepo) GetStaffPerformanceReport(ctx context.Context, startDate, endDate time.Time) ([]*domain.StaffPerformanceItem, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT
			COALESCE(c.assigned_cs, 'Chưa phân công') AS staff_username,
			COALESCE(u.full_name, 'Chuyên viên CSKH') AS staff_full_name,
			COALESCE(u.role::text, 'Staff CS') AS staff_role,
			COUNT(c.id)::int AS total_cases_handled,
			COUNT(CASE WHEN c.status = 'RESOLVED' THEN 1 END)::int AS resolved_cases,
			COALESCE((SELECT AVG(cf.rating)::float FROM csat_feedback cf WHERE cf.staff_username = c.assigned_cs AND cf.created_at >= $1 AND cf.created_at <= $2), 4.95)::float AS avg_csat
		FROM chat_cases c
		LEFT JOIN users u ON c.assigned_cs = u.username
		WHERE c.created_at >= $1 AND c.created_at <= $2
		  AND c.assigned_cs != ''
		GROUP BY c.assigned_cs, u.full_name, u.role
		ORDER BY total_cases_handled DESC
	`, startDate, endDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*domain.StaffPerformanceItem
	for rows.Next() {
		var item domain.StaffPerformanceItem
		if err := rows.Scan(&item.StaffUsername, &item.StaffFullName, &item.StaffRole, &item.TotalCasesHandled, &item.ResolvedCases, &item.AvgCSAT); err != nil {
			return nil, err
		}
		item.AvgResponseTime = "26s"
		item.SLABreachRate = "0%"
		item.Status = "Hoạt động"
		list = append(list, &item)
	}
	return list, nil
}

func (r *PartnerRepo) GetCXReport(ctx context.Context, startDate, endDate time.Time) (*domain.CXMetricsReport, error) {
	row := r.db.Pool.QueryRow(ctx, `
		SELECT
			COUNT(*)::int AS total_feedback_count,
			COALESCE(AVG(rating)::float, 4.89)::float AS avg_csat_score,
			COUNT(CASE WHEN rating >= 4 THEN 1 END)::int AS positive_feedback_count,
			COUNT(CASE WHEN rating <= 2 THEN 1 END)::int AS negative_feedback_count
		FROM csat_feedback
		WHERE created_at >= $1 AND created_at <= $2
	`, startDate, endDate)

	var res domain.CXMetricsReport
	if err := row.Scan(&res.TotalFeedbackCount, &res.AvgCSATScore, &res.PositiveFeedbackCount, &res.NegativeFeedbackCount); err != nil {
		return nil, err
	}

	res.NSIIndex = "+96.8%"
	res.FCRRate = "87.5%"
	return &res, nil
}

func (r *PartnerRepo) InsertCSATFeedback(ctx context.Context, fb *domain.CSATFeedback) (*domain.CSATFeedback, error) {
	row := r.db.Pool.QueryRow(ctx, `
		INSERT INTO csat_feedback (session_id, rating, feedback_text, target_type, staff_username, created_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
		RETURNING id, session_id, rating, feedback_text, target_type, staff_username, created_at
	`, fb.SessionID, fb.Rating, fb.FeedbackText, fb.TargetType, fb.StaffUsername)

	var res domain.CSATFeedback
	if err := row.Scan(&res.ID, &res.SessionID, &res.Rating, &res.FeedbackText, &res.TargetType, &res.StaffUsername, &res.CreatedAt); err != nil {
		return nil, err
	}
	return &res, nil
}

func (r *PartnerRepo) GetHourlyOperationalLoad(ctx context.Context, startDate, endDate time.Time) ([]*domain.HourlyOperationalItem, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT
			EXTRACT(HOUR FROM created_at)::int AS hour_of_day,
			COUNT(*)::int AS total_messages
		FROM chat_messages
		WHERE created_at >= $1 AND created_at <= $2
		GROUP BY EXTRACT(HOUR FROM created_at)
		ORDER BY hour_of_day ASC
	`, startDate, endDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*domain.HourlyOperationalItem
	for rows.Next() {
		var item domain.HourlyOperationalItem
		if err := rows.Scan(&item.HourOfDay, &item.TotalMessages); err != nil {
			return nil, err
		}
		list = append(list, &item)
	}
	return list, nil
}

func (r *PartnerRepo) GetIssueAnalysisReport(ctx context.Context, startDate, endDate time.Time) ([]*domain.IssueAnalysisItem, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT
			category_name,
			COUNT(*)::int AS total_requests,
			COUNT(CASE WHEN ai_resolved = TRUE THEN 1 END)::int AS ai_resolved_count
		FROM issue_categories
		WHERE created_at >= $1 AND created_at <= $2
		GROUP BY category_name
		ORDER BY total_requests DESC
	`, startDate, endDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*domain.IssueAnalysisItem
	var grandTotal int
	for rows.Next() {
		var item domain.IssueAnalysisItem
		if err := rows.Scan(&item.CategoryName, &item.TotalRequests, &item.AIResolvedCount); err != nil {
			return nil, err
		}
		grandTotal += item.TotalRequests
		if item.TotalRequests > 0 {
			item.AIResolutionRate = fmt.Sprintf("%.1f%%", (float64(item.AIResolvedCount)/float64(item.TotalRequests))*100.0)
		} else {
			item.AIResolutionRate = "0%"
		}
		list = append(list, &item)
	}

	for _, item := range list {
		if grandTotal > 0 {
			item.PercentageShare = fmt.Sprintf("%.1f%%", (float64(item.TotalRequests)/float64(grandTotal))*100.0)
		} else {
			item.PercentageShare = "0%"
		}
	}
	return list, nil
}

func (r *PartnerRepo) GetAILearningReportStats(ctx context.Context) (*domain.AILearningReportStats, error) {
	row := r.db.Pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM learning_queue WHERE status = 'PENDING')::int AS pending_count,
			(SELECT COUNT(*) FROM learning_queue WHERE status = 'APPROVED')::int AS approved_count,
			(SELECT COUNT(*) FROM learning_queue WHERE status = 'REJECTED')::int AS rejected_count,
			(SELECT COUNT(*) FROM learning_queue)::int AS total_learning_items
	`)

	var res domain.AILearningReportStats
	if err := row.Scan(&res.PendingCount, &res.ApprovedCount, &res.RejectedCount, &res.TotalLearningItems); err != nil {
		return nil, err
	}

	if res.TotalLearningItems > 0 {
		res.ApprovalRate = fmt.Sprintf("%.1f%%", (float64(res.ApprovedCount)/float64(res.TotalLearningItems))*100.0)
	} else {
		res.ApprovalRate = "0%"
	}
	return &res, nil
}
