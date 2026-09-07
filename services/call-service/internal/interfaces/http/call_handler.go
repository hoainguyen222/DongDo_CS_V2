package http

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hoainguyen222/DongDo_CS_V2/services/call-service/internal/application"
	"github.com/hoainguyen222/DongDo_CS_V2/services/call-service/internal/infrastructure/redis"
	"github.com/hoainguyen222/DongDo_CS_V2/services/call-service/internal/interfaces/websocket"
	"github.com/rs/zerolog/log"
)

type CallHandler struct {
	callSvc    *application.CallService
	agentStore *redis.AgentStateStore
	wsHub      *websocket.WSHub
}

func NewCallHandler(callSvc *application.CallService, agentStore *redis.AgentStateStore, wsHub *websocket.WSHub) *CallHandler {
	return &CallHandler{
		callSvc:    callSvc,
		agentStore: agentStore,
		wsHub:      wsHub,
	}
}

func (h *CallHandler) RegisterRoutes(r *gin.Engine) {
	api := r.Group("/api/v1/calls")
	{
		api.POST("", h.CreateCall)
		api.POST("/:id/accept", h.AcceptCall)
		api.POST("/:id/reject", h.RejectCall)
		api.POST("/:id/hangup", h.HangupCall)
		api.GET("", h.ListCalls)
		api.POST("/upload-recording", h.UploadRecording)
	}

	// Legacy endpoint compatibility route
	r.POST("/api/voice/upload-recording", h.UploadRecording)

	// Agent state management
	r.POST("/api/v1/agents/:id/status", h.SetAgentStatus)

	// WebSocket route
	r.GET("/ws", func(c *gin.Context) {
		h.wsHub.HandleWS(c)
	})

	// Static serve recordings
	r.Static("/recordings", "./recordings")
}

func (h *CallHandler) CreateCall(c *gin.Context) {
	var req struct {
		CustomerID string `json:"customer_id" binding:"required"`
		SessionID  string `json:"session_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	call, err := h.callSvc.CreateCall(c.Request.Context(), req.SessionID, req.CustomerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, call)
}

func (h *CallHandler) AcceptCall(c *gin.Context) {
	callID := c.Param("id")
	var req struct {
		AgentID string `json:"agent_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.callSvc.AcceptCall(c.Request.Context(), callID, req.AgentID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "CONNECTING", "call_id": callID})
}

func (h *CallHandler) RejectCall(c *gin.Context) {
	callID := c.Param("id")
	var req struct {
		AgentID string `json:"agent_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.callSvc.RejectCall(c.Request.Context(), callID, req.AgentID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "WAITING", "call_id": callID})
}

func (h *CallHandler) HangupCall(c *gin.Context) {
	callID := c.Param("id")
	var req struct {
		DurationSeconds int `json:"duration_seconds"`
	}
	_ = c.ShouldBindJSON(&req)

	if err := h.callSvc.HangupCall(c.Request.Context(), callID, req.DurationSeconds); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ENDED", "call_id": callID})
}

func (h *CallHandler) ListCalls(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "100")
	limit, _ := strconv.Atoi(limitStr)

	calls, err := h.callSvc.ListCalls(c.Request.Context(), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"calls": calls, "total": len(calls)})
}

func (h *CallHandler) SetAgentStatus(c *gin.Context) {
	agentID := c.Param("id")
	var req struct {
		Status string `json:"status" binding:"required"` // AVAILABLE / OFFLINE / AWAY
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Status == "AVAILABLE" {
		_ = h.agentStore.SetAgentAvailable(c.Request.Context(), agentID)
	} else {
		_ = h.agentStore.ReleaseAgent(c.Request.Context(), agentID)
	}

	c.JSON(http.StatusOK, gin.H{"agent_id": agentID, "status": req.Status})
}

func (h *CallHandler) UploadRecording(c *gin.Context) {
	file, err := c.FormFile("audio")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing audio file"})
		return
	}

	callID := c.PostForm("call_id")
	sessionID := c.PostForm("session_id")
	durationStr := c.PostForm("duration_seconds")
	transcript := c.PostForm("transcript")
	durationSec, _ := strconv.Atoi(durationStr)

	if callID == "" && sessionID != "" {
		callID = sessionID
	}

	_ = os.MkdirAll("./recordings", 0755)
	filename := fmt.Sprintf("call_%s_%d%s", callID, time.Now().Unix(), filepath.Ext(file.Filename))
	dst := filepath.Join("./recordings", filename)

	if err := c.SaveUploadedFile(file, dst); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save recording file"})
		return
	}

	recordingURL := "/recordings/" + filename
	if callID != "" {
		_ = h.callSvc.SaveRecording(c.Request.Context(), callID, recordingURL, file.Size, transcript)
		if durationSec > 0 {
			_ = h.callSvc.HangupCall(c.Request.Context(), callID, durationSec)
		}
	}

	log.Info().Str("call_id", callID).Str("url", recordingURL).Msg("Call recording uploaded successfully")
	c.JSON(http.StatusOK, gin.H{
		"status":        "success",
		"recording_url": recordingURL,
		"transcript":    transcript,
	})
}
