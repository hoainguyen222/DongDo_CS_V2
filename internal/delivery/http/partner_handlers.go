package http

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
	"github.com/hoainguyen222/DongDo_CS_V2/pkg/httpx"
)

// ============================================================
// 1. Dashboard Handlers
// ============================================================

func (h *Handler) HandleGetDashboardData(c *gin.Context) {
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	kpi, trend, recentChats, err := h.partnerUC.GetDashboardData(c.Request.Context(), startDate, endDate)
	if err != nil {
  httpx.InternalErrorResp(c, err, "HandleGetDashboardData")
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
  httpx.InternalErrorResp(c, err, "HandleListQuickTemplates")
		return
	}
	c.JSON(http.StatusOK, gin.H{"templates": list})
}

func (h *Handler) HandleCreateQuickTemplate(c *gin.Context) {
	var req domain.QuickTemplate
	if err := c.ShouldBindJSON(&req); err != nil {
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
  httpx.InternalErrorResp(c, err, "HandleCreateQuickTemplate")
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *Handler) HandleUpdateQuickTemplate(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID không hợp lệ"})
		return
	}

	var req domain.QuickTemplate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu không hợp lệ"})
		return
	}

	if err := h.partnerUC.UpdateQuickTemplate(c.Request.Context(), id, req.Title, req.Category, req.Content); err != nil {
  httpx.InternalErrorResp(c, err, "HandleUpdateQuickTemplate")
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Đã cập nhật tin nhắn mẫu thành công"})
}

func (h *Handler) HandleDeleteQuickTemplate(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID không hợp lệ"})
		return
	}

	if err := h.partnerUC.DeleteQuickTemplate(c.Request.Context(), id); err != nil {
  httpx.InternalErrorResp(c, err, "HandleDeleteQuickTemplate")
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
  httpx.InternalErrorResp(c, err, "HandleSaveSystemPromptHistory")
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Đã lưu lịch sử System Prompt thành công"})
}

func (h *Handler) HandleListRolePermissions(c *gin.Context) {
	list, err := h.partnerUC.ListRolePermissions(c.Request.Context())
	if err != nil {
  httpx.InternalErrorResp(c, err, "HandleListRolePermissions")
		return
	}
	c.JSON(http.StatusOK, gin.H{"permissions": list})
}

func (h *Handler) HandleUpsertRolePermission(c *gin.Context) {
	var req domain.RolePermission
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu không hợp lệ"})
		return
	}

	if err := h.partnerUC.UpsertRolePermission(c.Request.Context(), &req); err != nil {
  httpx.InternalErrorResp(c, err, "HandleUpsertRolePermission")
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success"})
}

func (h *Handler) HandleListAuditLogs(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	list, err := h.partnerUC.ListAuditLogs(c.Request.Context(), limit, offset)
	if err != nil {
  httpx.InternalErrorResp(c, err, "HandleListAuditLogs")
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
  httpx.InternalErrorResp(c, err, "HandleGetGeneralOverviewReport")
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *Handler) HandleGetAIPerformanceReport(c *gin.Context) {
	sDate := c.Query("start_date")
	eDate := c.Query("end_date")

	res, err := h.partnerUC.GetAIPerformanceReport(c.Request.Context(), sDate, eDate)
	if err != nil {
  httpx.InternalErrorResp(c, err, "HandleGetAIPerformanceReport")
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *Handler) HandleGetStaffPerformanceReport(c *gin.Context) {
	sDate := c.Query("start_date")
	eDate := c.Query("end_date")

	list, err := h.partnerUC.GetStaffPerformanceReport(c.Request.Context(), sDate, eDate)
	if err != nil {
  httpx.InternalErrorResp(c, err, "HandleGetStaffPerformanceReport")
		return
	}
	c.JSON(http.StatusOK, gin.H{"staff_reports": list})
}

func (h *Handler) HandleGetCXReport(c *gin.Context) {
	sDate := c.Query("start_date")
	eDate := c.Query("end_date")

	res, err := h.partnerUC.GetCXReport(c.Request.Context(), sDate, eDate)
	if err != nil {
  httpx.InternalErrorResp(c, err, "HandleGetCXReport")
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *Handler) HandleSubmitCSATFeedback(c *gin.Context) {
	var req domain.CSATFeedback
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu không hợp lệ"})
		return
	}

	res, err := h.partnerUC.SubmitCSATFeedback(c.Request.Context(), &req)
	if err != nil {
  httpx.InternalErrorResp(c, err, "HandleSubmitCSATFeedback")
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *Handler) HandleGetOperationalReport(c *gin.Context) {
	sDate := c.Query("start_date")
	eDate := c.Query("end_date")

	list, err := h.partnerUC.GetHourlyOperationalLoad(c.Request.Context(), sDate, eDate)
	if err != nil {
  httpx.InternalErrorResp(c, err, "HandleGetOperationalReport")
		return
	}
	c.JSON(http.StatusOK, gin.H{"hourly_load": list})
}

func (h *Handler) HandleGetIssueAnalysisReport(c *gin.Context) {
	sDate := c.Query("start_date")
	eDate := c.Query("end_date")

	list, err := h.partnerUC.GetIssueAnalysisReport(c.Request.Context(), sDate, eDate)
	if err != nil {
  httpx.InternalErrorResp(c, err, "HandleGetIssueAnalysisReport")
		return
	}
	c.JSON(http.StatusOK, gin.H{"issues": list})
}

func (h *Handler) HandleGetAILearningReportStats(c *gin.Context) {
	res, err := h.partnerUC.GetAILearningReportStats(c.Request.Context())
	if err != nil {
  httpx.InternalErrorResp(c, err, "HandleGetAILearningReportStats")
		return
	}
	c.JSON(http.StatusOK, res)
}
