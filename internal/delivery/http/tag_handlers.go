package http

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
)

// ============================================================
// Tag CRUD Handlers
// ============================================================

// GET /api/admin/chat/tags
func (h *Handler) HandleListChatTags(c *gin.Context) {
	tags, err := h.tagUC.ListTags(c.Request.Context())
	if err != nil {
		Logger.Error().Err(err).Msg("Failed to list chat tags")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if tags == nil {
		tags = []*domain.ChatTag{}
	}
	c.JSON(http.StatusOK, gin.H{"tags": tags})
}

type CreateTagReq struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	Color       string `json:"color"`
}

// POST /api/admin/chat/tags
func (h *Handler) HandleCreateChatTag(c *gin.Context) {
	var req CreateTagReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Logger.Warn().Err(err).Msg("Create chat tag: invalid request")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu không hợp lệ: " + err.Error()})
		return
	}

	createdBy := ""
	if u, ok := c.Get("user"); ok {
		if su, ok := u.(*domain.SessionUser); ok {
			createdBy = su.Username
		}
	}

	tag, err := h.tagUC.CreateTag(c.Request.Context(), req.Name, req.Description, req.Color, createdBy)
	if err != nil {
		Logger.Error().Err(err).Msg("Failed to create chat tag")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, tag)
}

type UpdateTagReq struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	Color       string `json:"color"`
}

// PUT /api/admin/chat/tags/:id
func (h *Handler) HandleUpdateChatTag(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID không hợp lệ"})
		return
	}

	var req UpdateTagReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu không hợp lệ"})
		return
	}

	if err := h.tagUC.UpdateTag(c.Request.Context(), id, req.Name, req.Description, req.Color); err != nil {
		Logger.Error().Err(err).Int64("id", id).Msg("Failed to update chat tag")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Đã cập nhật tag thành công"})
}

// DELETE /api/admin/chat/tags/:id
func (h *Handler) HandleDeleteChatTag(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ID không hợp lệ"})
		return
	}

	if err := h.tagUC.DeleteTag(c.Request.Context(), id); err != nil {
		Logger.Error().Err(err).Int64("id", id).Msg("Failed to delete chat tag")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Đã xóa tag thành công"})
}

// ============================================================
// Case Tag Handlers
// ============================================================

// GET /api/admin/cases/:session_id/tags
func (h *Handler) HandleGetCaseTags(c *gin.Context) {
	sessionID := c.Param("session_id")
	tags, err := h.tagUC.GetCaseTags(c.Request.Context(), sessionID)
	if err != nil {
		Logger.Error().Err(err).Str("session_id", sessionID).Msg("Failed to get case tags")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if tags == nil {
		tags = []*domain.CaseTag{}
	}
	c.JSON(http.StatusOK, gin.H{"tags": tags})
}

type AttachTagReq struct {
	TagID int64 `json:"tag_id" binding:"required"`
}

// POST /api/admin/cases/:session_id/tags
func (h *Handler) HandleAttachCaseTag(c *gin.Context) {
	sessionID := c.Param("session_id")
	var req AttachTagReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu không hợp lệ"})
		return
	}

	assignedBy := ""
	if u, ok := c.Get("user"); ok {
		if su, ok := u.(*domain.SessionUser); ok {
			assignedBy = su.Username
		}
	}

	if err := h.tagUC.AttachTag(c.Request.Context(), sessionID, req.TagID, assignedBy); err != nil {
		Logger.Error().Err(err).Str("session_id", sessionID).Int64("tag_id", req.TagID).Msg("Failed to attach tag")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Đã gắn tag thành công"})
}

// DELETE /api/admin/cases/:session_id/tags/:tag_id
func (h *Handler) HandleDetachCaseTag(c *gin.Context) {
	sessionID := c.Param("session_id")
	tagID, err := strconv.ParseInt(c.Param("tag_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tag ID không hợp lệ"})
		return
	}

	performedBy := ""
	if u, ok := c.Get("user"); ok {
		if su, ok := u.(*domain.SessionUser); ok {
			performedBy = su.Username
		}
	}

	if err := h.tagUC.DetachTag(c.Request.Context(), sessionID, tagID, performedBy); err != nil {
		Logger.Error().Err(err).Str("session_id", sessionID).Int64("tag_id", tagID).Msg("Failed to detach tag")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Đã gỡ tag thành công"})
}

// ============================================================
// Alert Config Handlers
// ============================================================

// GET /api/admin/chat/alert-config
func (h *Handler) HandleGetAlertConfig(c *gin.Context) {
	cfg, err := h.tagUC.GetAlertConfig(c.Request.Context())
	if err != nil {
		Logger.Error().Err(err).Msg("Failed to get alert config")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, cfg)
}

type SaveAlertConfigReq struct {
	IsEnabled      bool   `json:"is_enabled"`
	TimeoutSeconds int    `json:"timeout_seconds"`
	AlertContent   string `json:"alert_content"`
}

// POST /api/admin/chat/alert-config
func (h *Handler) HandleSaveAlertConfig(c *gin.Context) {
	var req SaveAlertConfigReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu không hợp lệ"})
		return
	}

	updatedBy := ""
	if u, ok := c.Get("user"); ok {
		if su, ok := u.(*domain.SessionUser); ok {
			updatedBy = su.Username
		}
	}

	cfg, err := h.tagUC.SaveAlertConfig(c.Request.Context(), req.IsEnabled, req.TimeoutSeconds, req.AlertContent, updatedBy)
	if err != nil {
		Logger.Error().Err(err).Msg("Failed to save alert config")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, cfg)
}

// ============================================================
// Alert Event Handlers
// ============================================================

// POST /api/admin/chat/alert-events — called by frontend to record an alert trigger
type CreateAlertEventReq struct {
	SessionID      string `json:"session_id" binding:"required"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}

func (h *Handler) HandleCreateAlertEvent(c *gin.Context) {
	var req CreateAlertEventReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu không hợp lệ"})
		return
	}
	if req.TimeoutSeconds <= 0 {
		req.TimeoutSeconds = 60
	}

	evt, err := h.tagUC.CreateAlertEvent(c.Request.Context(), req.SessionID, req.TimeoutSeconds)
	if err != nil {
		Logger.Error().Err(err).Str("session_id", req.SessionID).Msg("Failed to create alert event")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, evt)
}

// POST /api/admin/chat/alert-events/:session_id/resolve
func (h *Handler) HandleResolveAlertEvent(c *gin.Context) {
	sessionID := c.Param("session_id")
	if err := h.tagUC.ResolveAlertEvent(c.Request.Context(), sessionID); err != nil {
		Logger.Error().Err(err).Str("session_id", sessionID).Msg("Failed to resolve alert event")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "success"})
}
