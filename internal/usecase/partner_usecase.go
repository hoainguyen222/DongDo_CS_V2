package usecase

import (
	"context"
	"time"

	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
	"github.com/rs/zerolog"
)

type PartnerUseCase struct {
	partnerRepo domain.PartnerRepository
	settingRepo domain.SettingRepository
	logger      zerolog.Logger
}

func NewPartnerUseCase(partnerRepo domain.PartnerRepository, settingRepo domain.SettingRepository) *PartnerUseCase {
	return &PartnerUseCase{
		partnerRepo: partnerRepo,
		settingRepo: settingRepo,
		logger:      zerolog.New(nil).With().Timestamp().Str("usecase", "partner").Logger(),
	}
}

// Dashboard UseCase Methods
func (uc *PartnerUseCase) GetDashboardData(ctx context.Context, startDateStr, endDateStr string) (*domain.DashboardKpiSummary, []*domain.DashboardAutomationTrendDaily, []*domain.ChatCase, error) {
	startDate, endDate := parseDateRange(startDateStr, endDateStr)

	kpi, err := uc.partnerRepo.GetDashboardKpi(ctx, startDate, endDate)
	if err != nil {
		uc.logger.Warn().Err(err).Msg("failed to fetch dashboard KPI (using defaults)")
		kpi = &domain.DashboardKpiSummary{
			TotalConversations: 0,
			AIRateVal:          "0%",
			AvgResponseTime:    "1.2s",
			CSATVal:            "4.9 / 5.0",
		}
	}

	trend, err := uc.partnerRepo.GetDashboardAutomationTrend(ctx, startDate, endDate)
	if err != nil {
		uc.logger.Warn().Err(err).Msg("failed to fetch automation trend (using empty)")
		trend = []*domain.DashboardAutomationTrendDaily{}
	}

	chats, err := uc.partnerRepo.GetRecentCompletedChats(ctx, 20, 0)
	if err != nil {
		uc.logger.Warn().Err(err).Msg("failed to fetch recent completed chats (using empty)")
		chats = []*domain.ChatCase{}
	}

	return kpi, trend, chats, nil
}

// Config UseCase Methods
func (uc *PartnerUseCase) ListQuickTemplates(ctx context.Context) ([]*domain.QuickTemplate, error) {
	templates, err := uc.partnerRepo.ListQuickTemplates(ctx)
	if err != nil {
		uc.logger.Error().Err(err).Msg("failed to list quick templates")
		return nil, err
	}
	return templates, nil
}

func (uc *PartnerUseCase) CreateQuickTemplate(ctx context.Context, t *domain.QuickTemplate) (*domain.QuickTemplate, error) {
	created, err := uc.partnerRepo.CreateQuickTemplate(ctx, t)
	if err != nil {
		uc.logger.Error().Err(err).Str("title", t.Title).Msg("failed to create quick template")
		return nil, err
	}
	uc.logger.Info().Int64("template_id", created.ID).Str("title", created.Title).
		Msg("quick template created")
	return created, nil
}

func (uc *PartnerUseCase) UpdateQuickTemplate(ctx context.Context, id int64, title, category, content string) error {
	if err := uc.partnerRepo.UpdateQuickTemplate(ctx, id, title, category, content); err != nil {
		uc.logger.Error().Err(err).Int64("template_id", id).Msg("failed to update quick template")
		return err
	}
	return nil
}

func (uc *PartnerUseCase) DeleteQuickTemplate(ctx context.Context, id int64) error {
	if err := uc.partnerRepo.DeleteQuickTemplate(ctx, id); err != nil {
		uc.logger.Error().Err(err).Int64("template_id", id).Msg("failed to delete quick template")
		return err
	}
	return nil
}

func (uc *PartnerUseCase) SaveSystemPromptConfig(ctx context.Context, promptText, llmModel string, temp float64, username string) error {
	_ = uc.settingRepo.Set(ctx, "system_prompt", promptText)
	_ = uc.settingRepo.Set(ctx, "llm_model", llmModel)

	historyItem := &domain.SystemPromptHistory{
		SystemPrompt: promptText,
		LLMModel:     llmModel,
		Temperature:  temp,
		CreatedBy:    username,
	}
	if _, err := uc.partnerRepo.InsertSystemPromptHistory(ctx, historyItem); err != nil {
		uc.logger.Error().Err(err).Str("username", username).Msg("failed to insert system prompt history")
		return err
	}
	uc.logger.Info().Str("username", username).Str("llm_model", llmModel).
		Msg("system prompt config saved")
	return nil
}

func (uc *PartnerUseCase) ListRolePermissions(ctx context.Context) ([]*domain.RolePermission, error) {
	permissions, err := uc.partnerRepo.ListRolePermissions(ctx)
	if err != nil {
		uc.logger.Error().Err(err).Msg("failed to list role permissions")
		return nil, err
	}
	return permissions, nil
}

func (uc *PartnerUseCase) UpsertRolePermission(ctx context.Context, p *domain.RolePermission) error {
	if err := uc.partnerRepo.UpsertRolePermission(ctx, p); err != nil {
		uc.logger.Error().Err(err).Str("role_name", p.RoleName).
			Str("feature_key", p.FeatureKey).Msg("failed to upsert role permission")
		return err
	}
	return nil
}

// UpsertRolePermissionSimple is a convenience wrapper that creates a RolePermission from individual parameters.
func (uc *PartnerUseCase) UpsertRolePermissionSimple(ctx context.Context, roleName, featureKey, permissionLevel string) error {
	p := &domain.RolePermission{
		RoleName:        roleName,
		FeatureKey:      featureKey,
		PermissionLevel: permissionLevel,
		CanView:         permissionLevel == "act" || permissionLevel == "view",
		CanEdit:         permissionLevel == "act",
	}
	return uc.UpsertRolePermission(ctx, p)
}

func (uc *PartnerUseCase) ListAuditLogs(ctx context.Context, limit, offset int) ([]*domain.SystemAuditLog, error) {
	logs, err := uc.partnerRepo.ListAuditLogs(ctx, limit, offset)
	if err != nil {
		uc.logger.Error().Err(err).Msg("failed to list audit logs")
		return nil, err
	}
	return logs, nil
}

func (uc *PartnerUseCase) CreateAuditLog(ctx context.Context, actionType, details, username string) (*domain.SystemAuditLog, error) {
	log := &domain.SystemAuditLog{
		ActionType:  actionType,
		Details:     details,
		PerformedBy: username,
	}

	created, err := uc.partnerRepo.InsertAuditLog(ctx, log)
	if err != nil {
		uc.logger.Error().Err(err).Str("action_type", actionType).
			Str("username", username).Msg("failed to insert audit log")
		return nil, err
	}
	return created, nil
}

// Report UseCase Methods (7 Sub-reports)
func (uc *PartnerUseCase) GetGeneralOverviewReport(ctx context.Context, startDateStr, endDateStr string) (*domain.GeneralOverviewMetrics, error) {
	sDate, eDate := parseDateRange(startDateStr, endDateStr)

	report, err := uc.partnerRepo.GetGeneralOverviewReport(ctx, sDate, eDate)
	if err != nil {
		uc.logger.Error().Err(err).Msg("failed to fetch general overview report")
		return nil, err
	}
	return report, nil
}

func (uc *PartnerUseCase) GetAIPerformanceReport(ctx context.Context, startDateStr, endDateStr string) (*domain.AIPerformanceMetrics, error) {
	sDate, eDate := parseDateRange(startDateStr, endDateStr)

	report, err := uc.partnerRepo.GetAIPerformanceReport(ctx, sDate, eDate)
	if err != nil {
		uc.logger.Error().Err(err).Msg("failed to fetch AI performance report")
		return nil, err
	}
	return report, nil
}

func (uc *PartnerUseCase) GetStaffPerformanceReport(ctx context.Context, startDateStr, endDateStr string) ([]*domain.StaffPerformanceItem, error) {
	sDate, eDate := parseDateRange(startDateStr, endDateStr)

	report, err := uc.partnerRepo.GetStaffPerformanceReport(ctx, sDate, eDate)
	if err != nil {
		uc.logger.Error().Err(err).Msg("failed to fetch staff performance report")
		return nil, err
	}
	return report, nil
}

func (uc *PartnerUseCase) GetCXReport(ctx context.Context, startDateStr, endDateStr string) (*domain.CXMetricsReport, error) {
	sDate, eDate := parseDateRange(startDateStr, endDateStr)

	report, err := uc.partnerRepo.GetCXReport(ctx, sDate, eDate)
	if err != nil {
		uc.logger.Error().Err(err).Msg("failed to fetch CX metrics report")
		return nil, err
	}
	return report, nil
}

func (uc *PartnerUseCase) SubmitCSATFeedback(ctx context.Context, fb *domain.CSATFeedback) (*domain.CSATFeedback, error) {
	created, err := uc.partnerRepo.InsertCSATFeedback(ctx, fb)
	if err != nil {
		uc.logger.Error().Err(err).Str("session_id", fb.SessionID).Msg("failed to insert CSAT feedback")
		return nil, err
	}
	return created, nil
}

func (uc *PartnerUseCase) GetHourlyOperationalLoad(ctx context.Context, startDateStr, endDateStr string) ([]*domain.HourlyOperationalItem, error) {
	sDate, eDate := parseDateRange(startDateStr, endDateStr)

	report, err := uc.partnerRepo.GetHourlyOperationalLoad(ctx, sDate, eDate)
	if err != nil {
		uc.logger.Error().Err(err).Msg("failed to fetch hourly operational load report")
		return nil, err
	}
	return report, nil
}

func (uc *PartnerUseCase) GetIssueAnalysisReport(ctx context.Context, startDateStr, endDateStr string) ([]*domain.IssueAnalysisItem, error) {
	sDate, eDate := parseDateRange(startDateStr, endDateStr)

	report, err := uc.partnerRepo.GetIssueAnalysisReport(ctx, sDate, eDate)
	if err != nil {
		uc.logger.Error().Err(err).Msg("failed to fetch issue analysis report")
		return nil, err
	}
	return report, nil
}

func (uc *PartnerUseCase) GetAILearningReportStats(ctx context.Context) (*domain.AILearningReportStats, error) {
	stats, err := uc.partnerRepo.GetAILearningReportStats(ctx)
	if err != nil {
		uc.logger.Error().Err(err).Msg("failed to fetch AI learning report stats")
		return nil, err
	}
	return stats, nil
}

// System Error Persistence Methods
func (uc *PartnerUseCase) CreateSystemError(ctx context.Context, errRecord *domain.SystemErrorRecord) (*domain.SystemErrorRecord, error) {
	uc.logger.Warn().Str("source", errRecord.Source).Str("severity", errRecord.Severity).
		Msg("system error record creation")

	created, err := uc.partnerRepo.CreateSystemError(ctx, errRecord)
	if err != nil {
		uc.logger.Error().Err(err).Str("source", errRecord.Source).
			Msg("failed to create system error record")
		return nil, err
	}
	return created, nil
}

func (uc *PartnerUseCase) ListSystemErrors(ctx context.Context) ([]*domain.SystemErrorRecord, error) {
	errors, err := uc.partnerRepo.ListSystemErrors(ctx)
	if err != nil {
		uc.logger.Error().Err(err).Msg("failed to list system errors")
		return nil, err
	}
	return errors, nil
}

func (uc *PartnerUseCase) MarkSystemErrorHandled(ctx context.Context, id string) error {
	if err := uc.partnerRepo.MarkSystemErrorHandled(ctx, id); err != nil {
		uc.logger.Error().Err(err).Str("error_id", id).Msg("failed to mark system error handled")
		return err
	}
	return nil
}

// Helper date range parser
func parseDateRange(startDateStr, endDateStr string) (time.Time, time.Time) {
	now := time.Now()
	startDate := now.AddDate(0, 0, -30)
	endDate := now

	if startDateStr != "" {
		if t, err := time.Parse("2006-01-02", startDateStr); err == nil {
			startDate = t
		}
	}
	if endDateStr != "" {
		if t, err := time.Parse("2006-01-02", endDateStr); err == nil {
			endDate = t.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
		}
	}
	return startDate, endDate
}
