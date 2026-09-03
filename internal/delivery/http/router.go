package http

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/delivery/http/middleware"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/delivery/ws"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
	infraRedis "github.com/hoainguyen222/DongDo_CS_V2/internal/infra/redis"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/usecase"
	"github.com/hoainguyen222/DongDo_CS_V2/pkg/security"
	"github.com/redis/go-redis/v9"
)

// SetupRouter initializes Gin engine with middlewares and routes.
func SetupRouter(
	handler *Handler,
	hub *ws.Hub,
	chatUC *usecase.ChatUseCase,
	voiceUC *usecase.VoiceUseCase,
	stateMgr domain.StateManager,
	eventBus domain.EventBus,
	authUC *usecase.AuthUseCase,
	sessionUC *usecase.SessionUseCase,
	corsAllowedOrigins []string,
	wsAllowedOrigins []string,
	wsAdminInboxSession string,
	// Rate limiting
	redisClient *infraRedis.Client,
	rateLimitLogin, rateLimitChat, rateLimitAdmin, rateLimitUpload int,
) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	r.Use(gin.Recovery())
	r.Use(middleware.SecurityHeaders())
	r.Use(middleware.RequestID())
	r.Use(middleware.StructuredLogger()) // replaces plain gin.Logger with structured JSON
	r.Use(CORSMiddleware(corsAllowedOrigins))

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "healthy",
			"engine": "Golang Clean Architecture (Gin + SQLC)",
		})
	})

	// ─── WebSocket Endpoints (dual-mode, placed early) ───────────────
	// Staff WS: JWT auth → admin_inbox
	r.GET("/ws/staff", ws.ServeStaffWS(
		hub, chatUC, voiceUC, stateMgr, eventBus,
		authUC, wsAllowedOrigins, wsAdminInboxSession,
	))

	// Customer WS: session auth via ?session= query param
	r.GET("/ws/customer", ws.ServeCustomerWS(
		hub, chatUC, voiceUC, stateMgr, eventBus,
		sessionUC, wsAllowedOrigins, wsAdminInboxSession,
	))

	// Legacy WS endpoint (DEPRECATED — trusts query params, security risk)
	r.GET("/ws", ws.ServeWS(hub, chatUC, voiceUC, stateMgr, eventBus))

	// ─── Rate Limiters ─────────────────────────────────────────────
	// Nil-safe: if Redis not configured, limiters fall back to allow-all
	var redis *redis.Client
	if redisClient != nil {
		redis = redisClient.RDB()
	}

	// Login: strict — 5 req/min per IP (no user scoping needed here)
	loginLimiter := middleware.RateLimitByIPSimple(redis, middleware.RateLimiterConfig{
		RequestsPerMinute: rateLimitLogin,
		KeyPrefix:         "rl:login",
	})

	// Chat: medium — 30 req/min per IP (auth required)
	chatLimiter := middleware.RateLimitByIP(redis, middleware.RateLimiterConfig{
		RequestsPerMinute: rateLimitChat,
		KeyPrefix:         "rl:chat",
	})

	// Upload: low — 10 req/min per IP (admin only)
	uploadLimiter := middleware.RateLimitByIP(redis, middleware.RateLimiterConfig{
		RequestsPerMinute: rateLimitUpload,
		KeyPrefix:         "rl:upload",
	})

	// ─── Customer (guest) routes — SessionAuth ─────────────────────
	// /chat/guest-session and /chat/session/logout do NOT require auth
	// (you can't log out without first having a session)
	r.POST("/chat/guest-session", handler.HandleGuestSession)
	r.POST("/chat/session/logout", handler.HandleLogoutSession)
	r.PATCH("/chat/session", handler.HandleUpdateSession)

	// Customer-facing chat endpoints (require valid guest_session cookie + rate limit)
	customerGroup := r.Group("/chat")
	customerGroup.Use(middleware.SessionAuth(sessionUC))
	customerGroup.Use(chatLimiter)
	{
		customerGroup.POST("", handler.HandleChat)
	}

	// History endpoint also requires session auth
	r.GET("/history/:session_id", middleware.SessionAuth(sessionUC), handler.HandleGetHistory)

	// Legacy guest register (kept for backward compat — creates guest without session)
	r.POST("/guest/register", handler.HandleGuestRegister)

	// ─── Auth Endpoints (with login rate limit) ────────────────────
	// Legacy (DEPRECATED — returns token in JSON body)
	r.POST("/auth/login", loginLimiter, handler.HandleLogin)

	// Staff auth (JWT via httpOnly cookie)
	r.POST("/auth/staff/login", loginLimiter, handler.HandleStaffLogin)
	r.POST("/auth/staff/refresh", handler.HandleRefreshToken)
	r.POST("/auth/staff/logout", handler.HandleLogout)
	r.GET("/auth/staff/me", JWTAuthMiddleware(authUC), handler.HandleGetMe)

	// ─── Voice Call Endpoints (with upload rate limit) ─────────────
	r.POST("/api/voice/initiate", handler.HandleInitiateCall)
	r.POST("/api/voice/end", handler.HandleEndCall)
	r.POST("/api/voice/upload-recording", JWTAuthMiddleware(authUC), uploadLimiter, handler.HandleUploadRecording)

	// ─── Static Recordings (SECURE — Task 07) ──────────────────────
	// Auth required + path traversal protection
	r.GET("/static/recordings/:filename",
		JWTAuthMiddleware(authUC),
		func(c *gin.Context) {
			// 1. Sanitize filename
			raw := c.Param("filename")
			safeName, err := security.ValidateAndSanitizeFilename(raw)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid filename"})
				return
			}

			// 2. Build path + prefix check
			recordingsDir := filepath.Join(handler.docsDir, "..", "recordings")
			absDir, _ := filepath.Abs(recordingsDir)
			filePath := filepath.Join(absDir, safeName)

			if err := security.CheckPrefix(absDir, filePath); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "access denied"})
				return
			}

			// 3. Check file exists
			if _, err := os.Stat(filePath); os.IsNotExist(err) {
				c.JSON(http.StatusNotFound, gin.H{"error": "File ghi âm không tồn tại"})
				return
			}

			// 4. Serve
			c.Header("Content-Type", "audio/webm")
			c.Header("Accept-Ranges", "bytes")
			c.Header("Cache-Control", "public, max-age=31536000")
			c.File(filePath)
		},
	)

	// ─── Protected CSKH & Admin API Group ─────────────────────────
	// (uses JWT via httpOnly cookie + admin rate limit)
	admin := r.Group("/")
	admin.Use(JWTAuthMiddleware(authUC))
	admin.Use(middleware.RateLimitByIP(redis, middleware.RateLimiterConfig{
		RequestsPerMinute: rateLimitAdmin,
		KeyPrefix:         "rl:admin",
	}))
	{
		admin.GET("/auth/me", handler.HandleGetMe)
		admin.POST("/auth/logout", handler.HandleLogout)

		// Cases (Live Inbox)
		admin.GET("/api/admin/cases", handler.HandleListCases)
		admin.POST("/api/admin/cases/:session_id/take", handler.HandleTakeCase)
		admin.POST("/api/admin/cases/:session_id/reply", handler.HandleReplyCase)
		admin.POST("/api/admin/cases/:session_id/resolve", handler.HandleResolveCase)
		admin.PUT("/api/admin/cases/:session_id/customer", handler.HandleUpdateCaseCustomer)
		admin.DELETE("/api/admin/cases/:session_id", handler.HandleDeleteCase)
		admin.POST("/api/admin/cases/clear-all", handler.HandleClearAllCases)

		// Customer Profiles Management
		admin.GET("/api/admin/customers", handler.HandleListCustomers)
		admin.PUT("/api/admin/customers/:guest_id", handler.HandleUpdateCustomer)
		admin.DELETE("/api/admin/customers/:guest_id", handler.HandleDeleteCustomer)

		// Voice Call History
		admin.GET("/api/admin/voice/calls", handler.HandleGetCalls)
		admin.DELETE("/api/admin/voice/calls/:call_id", handler.HandleDeleteCall)

		// Continuous Learning Queue
		admin.GET("/api/admin/learning/pending", handler.HandleListPendingLearning)
		admin.PUT("/api/admin/learning/:item_id", handler.HandleUpdateLearning)
		admin.POST("/api/admin/learning/approve/:item_id", handler.HandleApproveLearning)
		admin.POST("/api/admin/learning/reject/:item_id", handler.HandleRejectLearning)
		admin.GET("/api/admin/learning/settings", handler.HandleGetLearningSettings)
		admin.POST("/api/admin/learning/settings", handler.HandleSetLearningSettings)
		admin.POST("/api/admin/learning/reset", handler.HandleResetLearnedKnowledge)

		// Knowledge Base
		admin.GET("/api/admin/knowledge", handler.HandleGetKnowledgeOverview)
		admin.POST("/api/admin/knowledge/upload", handler.HandleUploadDocument)

		// Analytics
		admin.GET("/api/admin/analytics", handler.HandleGetAnalytics)

		// System Configuration
		admin.GET("/api/admin/config", handler.HandleGetConfig)
		admin.POST("/api/admin/config", handler.HandleSaveConfig)

		// Partner Dashboard APIs
		admin.GET("/api/admin/partner/dashboard", handler.HandleGetDashboardData)

		// Partner Config APIs
		admin.GET("/api/admin/partner/config/templates", handler.HandleListQuickTemplates)
		admin.POST("/api/admin/partner/config/templates", handler.HandleCreateQuickTemplate)
		admin.PUT("/api/admin/partner/config/templates/:id", handler.HandleUpdateQuickTemplate)
		admin.DELETE("/api/admin/partner/config/templates/:id", handler.HandleDeleteQuickTemplate)
		admin.POST("/api/admin/partner/config/prompt-history", handler.HandleSaveSystemPromptHistory)
		admin.GET("/api/admin/partner/config/permissions", handler.HandleListRolePermissions)
		admin.POST("/api/admin/partner/config/permissions", handler.HandleUpsertRolePermission)
		admin.GET("/api/admin/partner/config/audit-logs", handler.HandleListAuditLogs)

		// Partner Reports APIs (7 Sub-reports)
		admin.GET("/api/admin/partner/reports/overview", handler.HandleGetGeneralOverviewReport)
		admin.GET("/api/admin/partner/reports/ai-performance", handler.HandleGetAIPerformanceReport)
		admin.GET("/api/admin/partner/reports/staff-performance", handler.HandleGetStaffPerformanceReport)
		admin.GET("/api/admin/partner/reports/cx", handler.HandleGetCXReport)
		admin.POST("/api/admin/partner/reports/csat", handler.HandleSubmitCSATFeedback)
		admin.GET("/api/admin/partner/reports/operational", handler.HandleGetOperationalReport)
		admin.GET("/api/admin/partner/reports/issue-analysis", handler.HandleGetIssueAnalysisReport)
		admin.GET("/api/admin/partner/reports/ai-learning", handler.HandleGetAILearningReportStats)
	}

	return r
}

func CORSMiddleware(allowedOrigins []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		// Check if origin is in whitelist
		allowed := false
		for _, o := range allowedOrigins {
			if o == origin {
				allowed = true
				break
			}
		}

		if allowed {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Set("Vary", "Origin")
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
			c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
			c.Writer.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization, X-Request-ID, X-Auth-Token")
			c.Writer.Header().Set("Access-Control-Max-Age", "86400") // 24h preflight cache
		}

		if c.Request.Method == "OPTIONS" {
			if allowed {
				c.AbortWithStatus(http.StatusNoContent)
			} else {
				c.AbortWithStatus(http.StatusForbidden)
			}
			return
		}

		c.Next()
	}
}

// JWTAuthMiddleware verifies JWT access tokens for staff routes.
// Supports both httpOnly cookie and Authorization: Bearer header.
func JWTAuthMiddleware(authUC *usecase.AuthUseCase) gin.HandlerFunc {
	return func(c *gin.Context) {
		var token string

		// Try cookie first (browser)
		if cookie, err := c.Cookie("access_token"); err == nil && cookie != "" {
			token = cookie
		} else {
			// Fallback: Authorization: Bearer header (for API clients / Postman)
			authHeader := c.GetHeader("Authorization")
			if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
				token = strings.TrimPrefix(authHeader, "Bearer ")
			}
		}

		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"detail": "Vui lòng đăng nhập để sử dụng tính năng này."})
			return
		}

		user, err := authUC.VerifyStaffToken(c.Request.Context(), token)
		if err != nil || user == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"detail": "Phiên đăng nhập đã hết hạn hoặc không hợp lệ. Vui lòng đăng nhập lại."})
			return
		}

		c.Set("user", user)
		c.Next()
	}
}
