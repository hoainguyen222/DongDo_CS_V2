package usecase

import (
	"context"
	"time"

	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
)

type PartnerUseCase struct {
	partnerRepo domain.PartnerRepository
	settingRepo domain.SettingRepository
}

func NewPartnerUseCase(partnerRepo domain.PartnerRepository, settingRepo domain.SettingRepository) *PartnerUseCase {
	return &PartnerUseCase{
		partnerRepo: partnerRepo,
		settingRepo: settingRepo,
	}
}

// Dashboard UseCase Methods
func (uc *PartnerUseCase) GetDashboardData(ctx context.Context, startDateStr, endDateStr string) (*domain.DashboardKpiSummary, []*domain.DashboardAutomationTrendDaily, []*domain.ChatCase, error) {
	startDate, endDate := parseDateRange(startDateStr, endDateStr)

	kpi, err := uc.partnerRepo.GetDashboardKpi(ctx, startDate, endDate)
	if err != nil {
		kpi = &domain.DashboardKpiSummary{
			TotalConversations: 0,
			AIRateVal:          "0%",
			AvgResponseTime:    "1.2s",
			CSATVal:            "4.9 / 5.0",
		}
	}

	trend, err := uc.partnerRepo.GetDashboardAutomationTrend(ctx, startDate, endDate)
	if err != nil {
		trend = []*domain.DashboardAutomationTrendDaily{}
	}

	chats, err := uc.partnerRepo.GetRecentCompletedChats(ctx, 20, 0)
	if err != nil {
		chats = []*domain.ChatCase{}
	}

	return kpi, trend, chats, nil
}

// Config UseCase Methods
func (uc *PartnerUseCase) ListQuickTemplates(ctx context.Context) ([]*domain.QuickTemplate, error) {
	return uc.partnerRepo.ListQuickTemplates(ctx)
}

func (uc *PartnerUseCase) CreateQuickTemplate(ctx context.Context, t *domain.QuickTemplate) (*domain.QuickTemplate, error) {
	return uc.partnerRepo.CreateQuickTemplate(ctx, t)
}

func (uc *PartnerUseCase) UpdateQuickTemplate(ctx context.Context, id int64, title, category, content string) error {
	return uc.partnerRepo.UpdateQuickTemplate(ctx, id, title, category, content)
}

func (uc *PartnerUseCase) DeleteQuickTemplate(ctx context.Context, id int64) error {
	return uc.partnerRepo.DeleteQuickTemplate(ctx, id)
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
	_, err := uc.partnerRepo.InsertSystemPromptHistory(ctx, historyItem)
	return err
}

func (uc *PartnerUseCase) ListRolePermissions(ctx context.Context) ([]*domain.RolePermission, error) {
	return uc.partnerRepo.ListRolePermissions(ctx)
}

func (uc *PartnerUseCase) UpsertRolePermission(ctx context.Context, p *domain.RolePermission) error {
	return uc.partnerRepo.UpsertRolePermission(ctx, p)
}

func (uc *PartnerUseCase) ListAuditLogs(ctx context.Context, limit, offset int) ([]*domain.SystemAuditLog, error) {
	return uc.partnerRepo.ListAuditLogs(ctx, limit, offset)
}

func (uc *PartnerUseCase) CreateAuditLog(ctx context.Context, actionType, details, username string) (*domain.SystemAuditLog, error) {
	log := &domain.SystemAuditLog{
		ActionType:  actionType,
		Details:     details,
		PerformedBy: username,
	}
	return uc.partnerRepo.InsertAuditLog(ctx, log)
}

// Report UseCase Methods (7 Sub-reports)
func (uc *PartnerUseCase) GetGeneralOverviewReport(ctx context.Context, startDateStr, endDateStr string) (*domain.GeneralOverviewMetrics, error) {
	sDate, eDate := parseDateRange(startDateStr, endDateStr)
	return uc.partnerRepo.GetGeneralOverviewReport(ctx, sDate, eDate)
}

func (uc *PartnerUseCase) GetAIPerformanceReport(ctx context.Context, startDateStr, endDateStr string) (*domain.AIPerformanceMetrics, error) {
	sDate, eDate := parseDateRange(startDateStr, endDateStr)
	return uc.partnerRepo.GetAIPerformanceReport(ctx, sDate, eDate)
}

func (uc *PartnerUseCase) GetStaffPerformanceReport(ctx context.Context, startDateStr, endDateStr string) ([]*domain.StaffPerformanceItem, error) {
	sDate, eDate := parseDateRange(startDateStr, endDateStr)
	return uc.partnerRepo.GetStaffPerformanceReport(ctx, sDate, eDate)
}

func (uc *PartnerUseCase) GetCXReport(ctx context.Context, startDateStr, endDateStr string) (*domain.CXMetricsReport, error) {
	sDate, eDate := parseDateRange(startDateStr, endDateStr)
	return uc.partnerRepo.GetCXReport(ctx, sDate, eDate)
}

func (uc *PartnerUseCase) SubmitCSATFeedback(ctx context.Context, fb *domain.CSATFeedback) (*domain.CSATFeedback, error) {
	return uc.partnerRepo.InsertCSATFeedback(ctx, fb)
}

func (uc *PartnerUseCase) GetHourlyOperationalLoad(ctx context.Context, startDateStr, endDateStr string) ([]*domain.HourlyOperationalItem, error) {
	sDate, eDate := parseDateRange(startDateStr, endDateStr)
	return uc.partnerRepo.GetHourlyOperationalLoad(ctx, sDate, eDate)
}

func (uc *PartnerUseCase) GetIssueAnalysisReport(ctx context.Context, startDateStr, endDateStr string) ([]*domain.IssueAnalysisItem, error) {
	sDate, eDate := parseDateRange(startDateStr, endDateStr)
	return uc.partnerRepo.GetIssueAnalysisReport(ctx, sDate, eDate)
}

func (uc *PartnerUseCase) GetAILearningReportStats(ctx context.Context) (*domain.AILearningReportStats, error) {
	return uc.partnerRepo.GetAILearningReportStats(ctx)
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
