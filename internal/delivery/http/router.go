package http

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/delivery/ws"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/usecase"
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
) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	r.Use(gin.Recovery())
	r.Use(gin.Logger())
	r.Use(CORSMiddleware())

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
	r.GET("/history/:session_id", handler.HandleGetHistory)

	// Auth Endpoints
	r.POST("/auth/login", handler.HandleLogin)

	// WebSocket Endpoint
	r.GET("/ws", ws.ServeWS(hub, chatUC, voiceUC, stateMgr, eventBus))

	// Voice Call Endpoints
	r.POST("/api/voice/initiate", handler.HandleInitiateCall)
	r.POST("/api/voice/end", handler.HandleEndCall)
	r.POST("/api/voice/upload-recording", handler.HandleUploadRecording)
	r.GET("/static/recordings/:filename", func(c *gin.Context) {
		filename := filepath.Base(c.Param("filename"))
		recordingsDir := filepath.Join(handler.docsDir, "..", "recordings")
		filePath := filepath.Join(recordingsDir, filename)

		if _, err := os.Stat(filePath); os.IsNotExist(err) {
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
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"detail": "Vui lòng đăng nhập để sử dụng tính năng này."})
			return
		}

		user, err := authUC.VerifyToken(c.Request.Context(), token)
		if err != nil || user == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"detail": "Phiên đăng nhập đã hết hạn hoặc không hợp lệ. Vui lòng đăng nhập lại."})
			return
		}

		c.Set("user", user)
		c.Next()
	}
}
