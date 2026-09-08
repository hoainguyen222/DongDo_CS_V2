package http

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/delivery/ws"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/observability"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/usecase"
	"github.com/rs/zerolog"
)

// RequestLogMiddleware logs all incoming HTTP requests with timing information
func RequestLogMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Process request
		c.Next()

		// Skip noisy successful requests
		if c.Writer.Status() < 400 {
			return
		}

		var userID string
		if user, exists := c.Get("user"); exists {
			if u, ok := user.(*domain.SessionUser); ok {
				userID = u.Username
			}
		}

		Logger.Warn().
			Str("method", c.Request.Method).
			Str("path", c.Request.URL.Path).
			Int("status", c.Writer.Status()).
			Str("user_id", userID).
			Msg("HTTP request failed")
	}
}

// RecoveryLogMiddleware recovers from panics and logs them
func RecoveryLogMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				Logger.Error().
					Str("method", c.Request.Method).
					Str("path", c.Request.URL.Path).
					Interface("panic", err).
					Msg("Panic recovered in HTTP handler")

				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error": "Internal server error",
				})
			}
		}()
		c.Next()
	}
}

// SetupRouter initializes Gin engine with middlewares and routes.
//
// httpMetrics may be nil (the chain stays metric-less, e.g. tests).
// wsMetrics may be nil — SetupRouter only wires WS metrics into the
// websocket package via package-level setters.
func SetupRouter(
	handler *Handler,
	hub *ws.Hub,
	chatUC *usecase.ChatUseCase,
	voiceUC *usecase.VoiceUseCase,
	stateMgr domain.StateManager,
	eventBus domain.EventBus,
	authUC *usecase.AuthUseCase,
	httpMetrics *observability.HTTPMetrics,
	wsMetrics *observability.WSMetrics,
) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	// Initialize zerolog if not already done
	if Logger.GetLevel() == zerolog.NoLevel {
		InitLogger("info")
	}

	// Order matters:
	//   1. Recovery first so panics in any later middleware/handler are caught.
	//   2. RequestID early so it is available to every later step.
	//   3. CORS so preflight requests are answered before logging/metrics.
	//   4. Structured access log (one line per request, all statuses).
	//   5. Prometheus HTTP metrics.
	//   6. Project-specific log for warn/error (legacy shape).
	r.Use(RecoveryLogMiddleware())
	r.Use(observability.RequestIDMiddleware())
	r.Use(CORSMiddleware())
	r.Use(observability.AccessLogMiddleware())
	if httpMetrics != nil {
		r.Use(httpMetrics.GinMiddleware())
	}
	r.Use(RequestLogMiddleware())

	// Install the WS metrics on the hub/client packages so we don't have to
	// thread an extra parameter through every constructor. Nil-safe: the
	// package-level hooks no-op when nil.
	if wsMetrics != nil {
		observability.SetWSMetrics(wsMetrics)
	}

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "healthy",
			"engine": "Golang Clean Architecture (Gin + SQLC)",
		})
	})

	// Public Guest & Chat Endpoints
	r.POST("/guest/register", handler.HandleGuestRegister)
	r.POST("/chat", handler.HandleChat)
	r.POST("/api/chat/typing", handler.HandleSendTyping)
	r.GET("/history/:session_id", handler.HandleGetHistory)

	// Auth Endpoints
	r.POST("/auth/login", handler.HandleLogin)

	// WebSocket Endpoint
	r.GET("/ws", ws.ServeWS(hub, chatUC, voiceUC, stateMgr, eventBus))

	// Voice Call Endpoints
	r.POST("/api/voice/initiate", handler.HandleInitiateCall)
	r.POST("/api/voice/end", handler.HandleEndCall)
	r.POST("/api/voice/decline", handler.HandleDeclineCall)
	r.POST("/api/voice/upload-recording", handler.HandleUploadRecording)
	r.GET("/static/recordings/:filename", func(c *gin.Context) {
		filename := filepath.Base(c.Param("filename"))
		recordingsDir := filepath.Join(handler.docsDir, "..", "recordings")
		filePath := filepath.Join(recordingsDir, filename)

		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			Logger.Warn().Str("filename", filename).Msg("Recording file not found")
			c.JSON(http.StatusNotFound, gin.H{"error": "File ghi âm không tồn tại"})
			return
		}

		c.Header("Content-Type", "audio/webm")
		c.Header("Accept-Ranges", "bytes")
		c.Header("Cache-Control", "public, max-age=31536000")
		c.File(filePath)
	})

	// Protected CSKH & Admin API Group
	admin := r.Group("/")
	admin.Use(AuthMiddleware(authUC))
	{
		admin.GET("/auth/me", handler.HandleGetMe)
		admin.POST("/auth/logout", handler.HandleLogout)

		// User Accounts Management - Admin+ only
		admin.GET("/api/admin/users", RequireRoles(RoleAdmin, RoleOwner), handler.HandleListUsers)
		admin.POST("/api/admin/users", RequireRoles(RoleAdmin, RoleOwner), handler.HandleCreateUser)
		admin.PUT("/api/admin/users/:username", RequireRoles(RoleAdmin, RoleOwner), handler.HandleUpdateUser)
		admin.DELETE("/api/admin/users/:username", RequireRoles(RoleOwner), handler.HandleDeleteUser)

		// Cases (Live Inbox) - Staff+ can access
		admin.GET("/api/admin/cases", handler.HandleListCases)
		admin.POST("/api/admin/cases/:session_id/take", handler.HandleTakeCase)
		admin.POST("/api/admin/cases/:session_id/reply", handler.HandleReplyCase)
		admin.POST("/api/admin/cases/:session_id/resolve", handler.HandleResolveCase)
		admin.POST("/api/admin/cases/:session_id/helper", handler.HandleSubmitCaseHelper)
		admin.GET("/api/admin/cases/helper-cases", handler.HandleListHelperCases)
		admin.POST("/api/admin/cases/:session_id/process-helper", RequireRoles(RoleLeader, RoleAdmin, RoleOwner), handler.HandleProcessHelperCase)
		admin.PUT("/api/admin/cases/:session_id/customer", handler.HandleUpdateCaseCustomer)
		admin.DELETE("/api/admin/cases/:session_id", RequireRoles(RoleAdmin, RoleOwner), handler.HandleDeleteCase)
		admin.POST("/api/admin/cases/clear-all", RequireRoles(RoleAdmin, RoleOwner), handler.HandleClearAllCases)

		// Chat Alert Config - Staff+ can access
		admin.GET("/api/admin/chat/alert-config", handler.HandleGetAlertConfig)
		admin.POST("/api/admin/chat/alert-config", RequireRoles(RoleAdmin, RoleOwner), handler.HandleSaveAlertConfig)

		// Customer Profiles Management - Staff+ can access
		admin.GET("/api/admin/customers", handler.HandleListCustomers)
		admin.PUT("/api/admin/customers/:guest_id", handler.HandleUpdateCaseCustomer)
		admin.DELETE("/api/admin/customers/:guest_id", RequireRoles(RoleAdmin, RoleOwner), handler.HandleDeleteCustomer)

		// Voice Call History - Staff+ can access
		admin.GET("/api/admin/voice/calls", handler.HandleGetCalls)
		admin.DELETE("/api/admin/voice/calls/:call_id", RequireRoles(RoleAdmin, RoleOwner), handler.HandleDeleteCall)

		// Continuous Learning Queue - Staff+ can access
		admin.GET("/api/admin/learning/pending", handler.HandleListPendingLearning)
		admin.PUT("/api/admin/learning/:item_id", RequireRoles(RoleAdmin, RoleOwner), handler.HandleUpdateLearning)
		admin.POST("/api/admin/learning/approve/:item_id", RequireRoles(RoleAdmin, RoleOwner), handler.HandleApproveLearning)
		admin.POST("/api/admin/learning/reject/:item_id", RequireRoles(RoleAdmin, RoleOwner), handler.HandleRejectLearning)
		admin.GET("/api/admin/learning/settings", handler.HandleGetLearningSettings)
		admin.POST("/api/admin/learning/settings", RequireRoles(RoleAdmin, RoleOwner), handler.HandleSetLearningSettings)
		admin.POST("/api/admin/learning/reset", RequireRoles(RoleAdmin, RoleOwner), handler.HandleResetLearnedKnowledge)

		// Knowledge Base - Staff+ can view, Admin+ can upload
		admin.GET("/api/admin/knowledge", handler.HandleGetKnowledgeOverview)
		admin.POST("/api/admin/knowledge/upload", RequireRoles(RoleAdmin, RoleOwner), handler.HandleUploadDocument)

		// Analytics - Staff+ can view
		admin.GET("/api/admin/analytics", handler.HandleGetAnalytics)

		// System Configuration - Admin+ only
		admin.GET("/api/admin/config", RequireRoles(RoleAdmin, RoleOwner), handler.HandleGetConfig)
		admin.POST("/api/admin/config", RequireRoles(RoleAdmin, RoleOwner), handler.HandleSaveConfig)

		// Chat Tags Management
		admin.GET("/api/admin/chat/tags", handler.HandleListChatTags)
		admin.POST("/api/admin/chat/tags", RequireRoles(RoleAdmin, RoleOwner), handler.HandleCreateChatTag)
		admin.PUT("/api/admin/chat/tags/:id", RequireRoles(RoleAdmin, RoleOwner), handler.HandleUpdateChatTag)
		admin.DELETE("/api/admin/chat/tags/:id", RequireRoles(RoleOwner), handler.HandleDeleteChatTag)

		// Case Tags
		admin.GET("/api/admin/cases/:session_id/tags", handler.HandleGetCaseTags)
		admin.POST("/api/admin/cases/:session_id/tags", handler.HandleAttachCaseTag)
		admin.DELETE("/api/admin/cases/:session_id/tags/:tag_id", handler.HandleDetachCaseTag)

		// Alert Config & Events
		admin.POST("/api/admin/chat/alert-events", handler.HandleCreateAlertEvent)
		admin.POST("/api/admin/chat/alert-events/:session_id/resolve", handler.HandleResolveAlertEvent)

		// Partner Dashboard APIs - Staff+ can view
		admin.GET("/api/admin/partner/dashboard", handler.HandleGetDashboardData)

		// Partner Config APIs - Admin+ for templates, Owner for audit logs
		admin.GET("/api/admin/partner/config/templates", RequireRoles(RoleAdmin, RoleOwner), handler.HandleListQuickTemplates)
		admin.POST("/api/admin/partner/config/templates", RequireRoles(RoleAdmin, RoleOwner), handler.HandleCreateQuickTemplate)
		admin.PUT("/api/admin/partner/config/templates/:id", RequireRoles(RoleAdmin, RoleOwner), handler.HandleUpdateQuickTemplate)
		admin.DELETE("/api/admin/partner/config/templates/:id", RequireRoles(RoleOwner), handler.HandleDeleteQuickTemplate)
		admin.POST("/api/admin/partner/config/prompt-history", RequireRoles(RoleAdmin, RoleOwner), handler.HandleSaveSystemPromptHistory)
		admin.GET("/api/admin/partner/config/permissions", RequireRoles(RoleAdmin, RoleOwner), handler.HandleListRolePermissions)
		admin.POST("/api/admin/partner/config/permissions", RequireRoles(RoleAdmin, RoleOwner), handler.HandleUpsertRolePermission)
		admin.GET("/api/admin/partner/config/audit-logs", RequireRoles(RoleOwner), handler.HandleListAuditLogs)

		// Partner Reports APIs (7 Sub-reports) - Staff+ can view
		admin.GET("/api/admin/partner/reports/overview", handler.HandleGetGeneralOverviewReport)
		admin.GET("/api/admin/partner/reports/ai-performance", handler.HandleGetAIPerformanceReport)
		admin.GET("/api/admin/partner/reports/staff-performance", handler.HandleGetStaffPerformanceReport)
		admin.GET("/api/admin/partner/reports/cx", handler.HandleGetCXReport)
		admin.POST("/api/admin/partner/reports/csat", handler.HandleSubmitCSATFeedback)
		admin.GET("/api/admin/partner/reports/operational", handler.HandleGetOperationalReport)
		admin.GET("/api/admin/partner/reports/issue-analysis", handler.HandleGetIssueAnalysisReport)
		admin.GET("/api/admin/partner/reports/ai-learning", handler.HandleGetAILearningReportStats)

		// System Errors Management - Admin+ only
		admin.GET("/api/admin/system-errors", RequireRoles(RoleAdmin, RoleOwner), handler.HandleListSystemErrors)
		admin.POST("/api/admin/system-errors", RequireRoles(RoleAdmin, RoleOwner), handler.HandleCreateSystemError)
		admin.PUT("/api/admin/system-errors/:id/handled", RequireRoles(RoleAdmin, RoleOwner), handler.HandleMarkSystemErrorHandled)
	}

	return r
}

func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With, X-Auth-Token")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, PATCH")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

func AuthMiddleware(authUC *usecase.AuthUseCase) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		token := ""

		if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
			token = strings.TrimPrefix(authHeader, "Bearer ")
		} else if xToken := c.GetHeader("X-Auth-Token"); xToken != "" {
			token = xToken
		}

		if token == "" {
			Logger.Warn().
				Str("path", c.Request.URL.Path).
				Msg("Auth failed: no token")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"detail": "Vui lòng đăng nhập để sử dụng tính năng này."})
			return
		}

		user, err := authUC.VerifyToken(c.Request.Context(), token)
		if err != nil || user == nil {
			Logger.Warn().
				Str("path", c.Request.URL.Path).
				Err(err).
				Msg("Auth failed: invalid token")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"detail": "Phiên đăng nhập đã hết hạn hoặc không hợp lệ. Vui lòng đăng nhập lại."})
			return
		}

		c.Set("user", user)
		c.Next()
	}
}
