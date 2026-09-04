package http

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
)

// ============================================================
// 1. Dashboard Handlers
// ============================================================

func (h *Handler) HandleGetDashboardData(c *gin.Context) {
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	kpi, trend, recentChats, err := h.partnerUC.GetDashboardData(c.Request.Context(), startDate, endDate)
	if err != nil {
		Logger.Error().Err(err).Msg("Failed to get dashboard data")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Lỗi lấy dữ liệu Dashboard: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"kpi":          kpi,
		"trend":        trend,
		"recent_chats": recentChats,
	})
}

// ============================================================
// 2. Config Handlers (Prompt, Templates, RBAC, Audit)
// ============================================================

func (h *Handler) HandleListQuickTemplates(c *gin.Context) {
	list, err := h.partnerUC.ListQuickTemplates(c.Request.Context())
	if err != nil {
		Logger.Error().Err(err).Msg("Failed to list quick templates")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"templates": list})
}

func (h *Handler) HandleCreateQuickTemplate(c *gin.Context) {
	var req domain.QuickTemplate
	if err := c.ShouldBindJSON(&req); err != nil {
		Logger.Warn().Err(err).Msg("Create quick template validation failed")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu không hợp lệ"})
		return
	}

	userVal, exists := c.Get("user")
	if exists {
		if u, ok := userVal.(*domain.SessionUser); ok {
			req.CreatedBy = u.Username
		}
	}

	res, err := h.partnerUC.CreateQuickTemplate(c.Request.Context(), &req)
	if err != nil {
		Logger.Error().Err(err).Msg("Failed to create quick template")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}

func (h *Handler) HandleUpdateQuickTemplate(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		Logger.Warn().Err(err).Msg("Update quick template validation failed: invalid ID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID không hợp lệ"})
		return
	}

	var req domain.QuickTemplate
	if err := c.ShouldBindJSON(&req); err != nil {
		Logger.Warn().Err(err).Msg("Update quick template validation failed")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu không hợp lệ"})
		return
	}

	if err := h.partnerUC.UpdateQuickTemplate(c.Request.Context(), id, req.Title, req.Category, req.Content); err != nil {
		Logger.Error().Err(err).Msg("Failed to update quick template")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Đã cập nhật tin nhắn mẫu thành công"})
}

func (h *Handler) HandleDeleteQuickTemplate(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		Logger.Warn().Err(err).Msg("Delete quick template validation failed: invalid ID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID không hợp lệ"})
		return
	}

	if err := h.partnerUC.DeleteQuickTemplate(c.Request.Context(), id); err != nil {
		Logger.Error().Err(err).Msg("Failed to delete quick template")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Đã xóa tin nhắn mẫu"})
}

type SaveSystemPromptReq struct {
	SystemPrompt string  `json:"system_prompt"`
	LLMModel     string  `json:"llm_model"`
	Temperature  float64 `json:"temperature"`
}

func (h *Handler) HandleSaveSystemPromptHistory(c *gin.Context) {
	var req SaveSystemPromptReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Logger.Warn().Err(err).Msg("Save system prompt history validation failed")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu không hợp lệ"})
		return
	}

	username := "admin"
	if uVal, exists := c.Get("user"); exists {
		if u, ok := uVal.(*domain.SessionUser); ok {
			username = u.Username
		}
	}

	if err := h.partnerUC.SaveSystemPromptConfig(c.Request.Context(), req.SystemPrompt, req.LLMModel, req.Temperature, username); err != nil {
		Logger.Error().Err(err).Msg("Failed to save system prompt history")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Đã lưu lịch sử System Prompt thành công"})
}

func (h *Handler) HandleListRolePermissions(c *gin.Context) {
	list, err := h.partnerUC.ListRolePermissions(c.Request.Context())
	if err != nil {
		Logger.Error().Err(err).Msg("Failed to list role permissions")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"permissions": list})
}

func (h *Handler) HandleUpsertRolePermission(c *gin.Context) {
	var req domain.RolePermission
	if err := c.ShouldBindJSON(&req); err != nil {
		Logger.Warn().Err(err).Msg("Upsert role permission validation failed")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu không hợp lệ"})
		return
	}

	userVal, exists := c.Get("user")
	if exists {
		if u, ok := userVal.(*domain.SessionUser); ok {
			performerRole := strings.ToLower(string(u.Role))
			if strings.EqualFold(req.RoleName, "Owner") && performerRole != "owner" {
				Logger.Warn().Msg("Upsert role permission forbidden: cannot modify Owner permissions")
				c.JSON(http.StatusForbidden, gin.H{"error": "Chỉ có Chủ sở hữu (Owner) mới có quyền chỉnh sửa ma trận phân quyền của Owner!"})
				return
			}
		}
	}

	if err := h.partnerUC.UpsertRolePermission(c.Request.Context(), &req); err != nil {
		Logger.Error().Err(err).Msg("Failed to upsert role permission")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func (h *Handler) HandleListAuditLogs(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	list, err := h.partnerUC.ListAuditLogs(c.Request.Context(), limit, offset)
	if err != nil {
		Logger.Error().Err(err).Msg("Failed to list audit logs")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"audit_logs": list})
}

// ============================================================
// 3. Report Handlers (7 Sub-reports)
// ============================================================

func (h *Handler) HandleGetGeneralOverviewReport(c *gin.Context) {
	sDate := c.Query("start_date")
	eDate := c.Query("end_date")

	res, err := h.partnerUC.GetGeneralOverviewReport(c.Request.Context(), sDate, eDate)
	if err != nil {
		Logger.Error().Err(err).Msg("Failed to get general overview report")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}

func (h *Handler) HandleGetAIPerformanceReport(c *gin.Context) {
	sDate := c.Query("start_date")
	eDate := c.Query("end_date")

	res, err := h.partnerUC.GetAIPerformanceReport(c.Request.Context(), sDate, eDate)
	if err != nil {
		Logger.Error().Err(err).Msg("Failed to get AI performance report")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}

func (h *Handler) HandleGetStaffPerformanceReport(c *gin.Context) {
	sDate := c.Query("start_date")
	eDate := c.Query("end_date")

	list, err := h.partnerUC.GetStaffPerformanceReport(c.Request.Context(), sDate, eDate)
	if err != nil {
		Logger.Error().Err(err).Msg("Failed to get staff performance report")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"staff_reports": list})
}

func (h *Handler) HandleGetCXReport(c *gin.Context) {
	sDate := c.Query("start_date")
	eDate := c.Query("end_date")

	res, err := h.partnerUC.GetCXReport(c.Request.Context(), sDate, eDate)
	if err != nil {
		Logger.Error().Err(err).Msg("Failed to get CX report")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}

func (h *Handler) HandleSubmitCSATFeedback(c *gin.Context) {
	var req domain.CSATFeedback
	if err := c.ShouldBindJSON(&req); err != nil {
		Logger.Warn().Err(err).Msg("Submit CSAT feedback validation failed")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu không hợp lệ"})
		return
	}

	res, err := h.partnerUC.SubmitCSATFeedback(c.Request.Context(), &req)
	if err != nil {
		Logger.Error().Err(err).Msg("Failed to submit CSAT feedback")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}

func (h *Handler) HandleGetOperationalReport(c *gin.Context) {
	sDate := c.Query("start_date")
	eDate := c.Query("end_date")

	list, err := h.partnerUC.GetHourlyOperationalLoad(c.Request.Context(), sDate, eDate)
	if err != nil {
		Logger.Error().Err(err).Msg("Failed to get operational report")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"hourly_load": list})
}

func (h *Handler) HandleGetIssueAnalysisReport(c *gin.Context) {
	sDate := c.Query("start_date")
	eDate := c.Query("end_date")

	list, err := h.partnerUC.GetIssueAnalysisReport(c.Request.Context(), sDate, eDate)
	if err != nil {
		Logger.Error().Err(err).Msg("Failed to get issue analysis report")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"issues": list})
}

func (h *Handler) HandleGetAILearningReportStats(c *gin.Context) {
	res, err := h.partnerUC.GetAILearningReportStats(c.Request.Context())
	if err != nil {
		Logger.Error().Err(err).Msg("Failed to get AI learning report stats")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}
