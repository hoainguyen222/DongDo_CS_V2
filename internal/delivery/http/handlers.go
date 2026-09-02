package http

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/usecase"
	"github.com/hoainguyen222/DongDo_CS_V2/pkg/httpx"
	"github.com/hoainguyen222/DongDo_CS_V2/pkg/security"
)

type Handler struct {
	authUC      *usecase.AuthUseCase
	sessionUC   *usecase.SessionUseCase
	chatUC      *usecase.ChatUseCase
	caseUC      *usecase.CaseUseCase
	learningUC  *usecase.LearningUseCase
	voiceUC     *usecase.VoiceUseCase
	analyticsUC *usecase.AnalyticsUseCase
	partnerUC   *usecase.PartnerUseCase
	ragUC       *usecase.RAGUseCase
	vectorStore domain.VectorStore
	embedder    domain.Embedder
	docsDir     string
	eventBus    domain.EventBus
}

func NewHandler(
	authUC *usecase.AuthUseCase,
	sessionUC *usecase.SessionUseCase,
	chatUC *usecase.ChatUseCase,
	caseUC *usecase.CaseUseCase,
	learningUC *usecase.LearningUseCase,
	voiceUC *usecase.VoiceUseCase,
	analyticsUC *usecase.AnalyticsUseCase,
	partnerUC *usecase.PartnerUseCase,
	ragUC *usecase.RAGUseCase,
	vectorStore domain.VectorStore,
	embedder domain.Embedder,
	docsDir string,
	eventBus domain.EventBus,
) *Handler {
	return &Handler{
		authUC:      authUC,
		sessionUC:   sessionUC,
		chatUC:      chatUC,
		caseUC:      caseUC,
		learningUC:  learningUC,
		voiceUC:     voiceUC,
		analyticsUC: analyticsUC,
		partnerUC:   partnerUC,
		ragUC:       ragUC,
		vectorStore: vectorStore,
		embedder:    embedder,
		docsDir:     docsDir,
		eventBus:    eventBus,
	}
}

// ============================================================
// Auth & Guest Handlers
// ============================================================

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// HandleStaffLogin handles POST /auth/staff/login
// Verifies credentials and issues JWT access + refresh tokens via httpOnly cookies.
// NEVER echoes token in JSON body.
func (h *Handler) HandleStaffLogin(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "Vui lòng nhập đầy đủ tên đăng nhập và mật khẩu"})
		return
	}

	accessToken, refreshToken, user, err := h.authUC.StaffLogin(c.Request.Context(), req.Username, req.Password)
	if err != nil {
  httpx.UnauthorizedResp(c, "Xác thực thất bại")
		return
	}

	// Set httpOnly cookies — NEVER echo tokens in JSON body
	c.SetCookie("access_token", accessToken, 15*60, "/", "", true, true)              // 15 min, HttpOnly, Secure
	c.SetCookie("refresh_token", refreshToken, 7*24*3600, "/auth/staff", "", true, true) // 7 days, HttpOnly, Secure, Path=/auth/staff

	c.JSON(http.StatusOK, gin.H{
		"user": gin.H{
			"username":  user.Username,
			"full_name": user.FullName,
			"role":      user.Role,
		},
	})
}

// HandleRefreshToken handles POST /auth/staff/refresh
// Reads refresh token from cookie, verifies it, issues new access token.
func (h *Handler) HandleRefreshToken(c *gin.Context) {
	refreshToken, err := c.Cookie("refresh_token")
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"detail": "missing refresh token"})
		return
	}

	newAccessToken, err := h.authUC.RefreshStaffToken(c.Request.Context(), refreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"detail": "invalid or expired refresh token"})
		return
	}

	// Set new access token cookie
	c.SetCookie("access_token", newAccessToken, 15*60, "/", "", true, true)

	c.JSON(http.StatusOK, gin.H{
		"message": "token refreshed",
	})
}

// HandleLogout handles POST /auth/staff/logout
// Revokes the JWT and clears cookies.
func (h *Handler) HandleLogout(c *gin.Context) {
	accessToken, _ := c.Cookie("access_token")

	// Revoke token in DB (best-effort)
	if accessToken != "" {
		_ = h.authUC.RevokeStaffToken(c.Request.Context(), accessToken, "user logout")
	}

	// Clear cookies
	c.SetCookie("access_token", "", -1, "/", "", true, true)
	c.SetCookie("refresh_token", "", -1, "/auth/staff", "", true, true)

	c.JSON(http.StatusOK, gin.H{"message": "Đã đăng xuất thành công."})
}

// HandleGetMe handles GET /auth/staff/me
// Returns current authenticated user info from JWT claims.
func (h *Handler) HandleGetMe(c *gin.Context) {
	user := c.MustGet("user").(*domain.SessionUser)
	c.JSON(http.StatusOK, gin.H{
		"username":  user.Username,
		"full_name": user.FullName,
		"role":      user.Role,
	})
}

// HandleLogin is the legacy login handler (DEPRECATED, kept for backward compat).
// For new staff login, use HandleStaffLogin (JWT via httpOnly cookie).
func (h *Handler) HandleLogin(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "Vui lòng nhập đầy đủ tên đăng nhập và mật khẩu"})
		return
	}

	user, err := h.authUC.Login(c.Request.Context(), req.Username, req.Password)
	if err != nil {
  httpx.UnauthorizedResp(c, "Xác thực thất bại")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":     user.Token,
		"username":  user.Username,
		"full_name": user.FullName,
		"role":      user.Role,
	})
}

type GuestRegisterRequest struct {
	DisplayName string `json:"display_name"`
	Phone       string `json:"phone"`
}

func (h *Handler) HandleGuestRegister(c *gin.Context) {
	var req GuestRegisterRequest
	_ = c.ShouldBindJSON(&req)

	guest, token, err := h.authUC.RegisterGuest(c.Request.Context(), req.DisplayName, req.Phone)
	if err != nil {
  httpx.InternalErrorResp(c, err, "HandleGuestRegister")
		return
	}

	sessionID := "session-" + guest.GuestID.String()[:8] + "-" + strconv.FormatInt(time.Now().UnixMilli(), 10)

	// Initialize case record with phone
	_, _ = h.caseUC.InitCase(c.Request.Context(), sessionID, &guest.GuestID, req.DisplayName, req.Phone)

	c.JSON(http.StatusOK, gin.H{
		"guest_id":     guest.GuestID,
		"display_name": guest.DisplayName,
		"phone":        guest.Phone,
		"session_id":   sessionID,
		"token":        token,
	})
}

// ============================================================
// Customer Session Handlers (POST /chat/guest-session, /chat/session/logout)
// ============================================================

type UpdateSessionRequest struct {
	DisplayName string `json:"display_name"`
}

// HandleGuestSession creates a new guest session or resumes an existing one.
// POST /chat/guest-session
// Sets cookie `guest_session` (NOT HttpOnly so JS can read for WebSocket).
func (h *Handler) HandleGuestSession(c *gin.Context) {
	// If there's an existing cookie, try to resume
	if existingID, _ := c.Cookie("guest_session"); existingID != "" {
		session, err := h.sessionUC.ValidateSession(c.Request.Context(), existingID)
		if err == nil && session != nil {
			// Refresh cookie expiry
			maxAge := int(time.Until(session.ExpiresAt).Seconds())
			if maxAge < 0 {
				maxAge = 0
			}
			c.SetCookie("guest_session", session.SessionID, maxAge, "/", "", false, false)
			c.JSON(http.StatusOK, gin.H{
				"session_id":   session.SessionID,
				"display_name": session.DisplayName,
				"expires_at":   session.ExpiresAt,
				"resumed":      true,
			})
			return
		}
		// Invalid cookie — fall through and create new
	}

	// Create new session
	session, _, err := h.sessionUC.EnsureSession(
		c.Request.Context(),
		c.ClientIP(),
		c.Request.UserAgent(),
	)
	if err != nil {
  httpx.InternalErrorResp(c, err, "HandleGuestSession")
		return
	}

	maxAge := int(time.Until(session.ExpiresAt).Seconds())
	c.SetCookie("guest_session", session.SessionID, maxAge, "/", "", false, false)

	c.JSON(http.StatusOK, gin.H{
		"session_id":   session.SessionID,
		"display_name": session.DisplayName,
		"expires_at":   session.ExpiresAt,
		"resumed":      false,
	})
}

// HandleLogoutSession deactivates the current session and clears the cookie.
// POST /chat/session/logout
func (h *Handler) HandleLogoutSession(c *gin.Context) {
	if sessionID, _ := c.Cookie("guest_session"); sessionID != "" {
		_ = h.sessionUC.LogoutSession(c.Request.Context(), sessionID)
	}

	c.SetCookie("guest_session", "", -1, "/", "", false, false)
	c.JSON(http.StatusOK, gin.H{"message": "Đã đăng xuất phiên khách."})
}

// HandleUpdateSession updates display name for the current session.
// PATCH /chat/session
func (h *Handler) HandleUpdateSession(c *gin.Context) {
	var req UpdateSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "Vui lòng cung cấp display_name."})
		return
	}

	sessionID, _ := c.Cookie("guest_session")
	if sessionID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"detail": "missing session"})
		return
	}

	if err := h.sessionUC.UpdateDisplayName(c.Request.Context(), sessionID, req.DisplayName); err != nil {
  httpx.InternalErrorResp(c, err, "HandleUpdateSession")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"session_id":   sessionID,
		"display_name": req.DisplayName,
	})
}

// ============================================================
// Chat Handlers
// ============================================================

type ChatRequest struct {
	SessionID   string     `json:"session_id"`
	CustomerName string    `json:"customer_name"`
	Message     string     `json:"message" binding:"required"`
	ClientMsgID *uuid.UUID `json:"client_msg_id"`
}

func (h *Handler) HandleChat(c *gin.Context) {
	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "Tin nhắn không được để trống"})
		return
	}

	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = fmt.Sprintf("session-%d-%s", time.Now().UnixMilli(), uuid.New().String()[:6])
	}

	custName := req.CustomerName
	if custName == "" {
		custName = "Khách hàng"
	}

	msg, err := h.chatUC.SendGuestMessage(c.Request.Context(), sessionID, custName, req.Message, req.ClientMsgID)
	if err != nil {
  httpx.InternalErrorResp(c, err, "HandleChat")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"session_id": sessionID,
		"message_id": msg.ID,
		"status":     "RECEIVED",
	})
}

func (h *Handler) HandleGetHistory(c *gin.Context) {
	sessionID := c.Param("session_id")
	messages, chatCase, err := h.chatUC.GetHistory(c.Request.Context(), sessionID)
	if err != nil {
  httpx.InternalErrorResp(c, err, "HandleGetHistory")
		return
	}

	status := "AI_ACTIVE"
	var assignedCS string
	if chatCase != nil {
		status = string(chatCase.Status)
		assignedCS = chatCase.AssignedCS
	}

	c.JSON(http.StatusOK, gin.H{
		"session_id":  sessionID,
		"status":      status,
		"assigned_cs": assignedCS,
		"messages":    messages,
	})
}

// ============================================================
// Case Handlers (CS Studio)
// ============================================================

func (h *Handler) HandleListCases(c *gin.Context) {
	statusFilter := domain.CaseStatus(c.Query("status"))
	search := strings.TrimSpace(c.Query("search"))
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}

	allCases, err := h.caseUC.ListCases(c.Request.Context(), statusFilter)
	if err != nil {
  httpx.InternalErrorResp(c, err, "HandleListCases")
		return
	}

	var filtered []*domain.ChatCase
	if search != "" {
		sLower := strings.ToLower(search)
		for _, cs := range allCases {
			if strings.Contains(strings.ToLower(cs.CustomerName), sLower) ||
				strings.Contains(strings.ToLower(cs.CustomerPhone), sLower) ||
				strings.Contains(strings.ToLower(cs.SessionID), sLower) ||
				strings.Contains(strings.ToLower(cs.LastMessage), sLower) {
				filtered = append(filtered, cs)
			}
		}
	} else {
		filtered = allCases
	}

	total := int64(len(filtered))
	totalPages := int(math.Ceil(float64(total) / float64(limit)))
	if totalPages < 1 {
		totalPages = 1
	}

	startIndex := (page - 1) * limit
	var pagedCases []*domain.ChatCase
	if startIndex < len(filtered) {
		endIndex := startIndex + limit
		if endIndex > len(filtered) {
			endIndex = len(filtered)
		}
		pagedCases = filtered[startIndex:endIndex]
	} else {
		pagedCases = []*domain.ChatCase{}
	}

	c.JSON(http.StatusOK, gin.H{
		"cases":       pagedCases,
		"total":       total,
		"page":        page,
		"limit":       limit,
		"total_pages": totalPages,
	})
}

func (h *Handler) HandleTakeCase(c *gin.Context) {
	sessionID := c.Param("session_id")
	user := c.MustGet("user").(*domain.SessionUser)

	err := h.caseUC.TakeCase(c.Request.Context(), sessionID, user.Username, user.FullName)
	if err != nil {
  httpx.InternalErrorResp(c, err, "HandleTakeCase")
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Đã tiếp nhận case thành công"})
}

type ReplyRequest struct {
	Message string `json:"message" binding:"required"`
}

func (h *Handler) HandleReplyCase(c *gin.Context) {
	sessionID := c.Param("session_id")
	user := c.MustGet("user").(*domain.SessionUser)

	var req ReplyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "Tin nhắn không được để trống"})
		return
	}

	_, err := h.chatUC.SendCSReply(c.Request.Context(), sessionID, user.Username, user.FullName, req.Message)
	if err != nil {
  httpx.InternalErrorResp(c, err, "HandleReplyCase")
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Đã gửi tin nhắn thành công"})
}

type ResolveRequest struct {
	ResolutionNote string         `json:"resolution_note"`
	ExtractPairs   []domain.QAPair `json:"extract_pairs"`
}

func (h *Handler) HandleResolveCase(c *gin.Context) {
	sessionID := c.Param("session_id")
	user := c.MustGet("user").(*domain.SessionUser)

	var req ResolveRequest
	_ = c.ShouldBindJSON(&req)

	autoLearned, count, err := h.caseUC.ResolveCase(c.Request.Context(), sessionID, user.Username, user.FullName, req.ResolutionNote, req.ExtractPairs)
	if err != nil {
  httpx.InternalErrorResp(c, err, "HandleResolveCase")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"auto_learned":  autoLearned,
		"learned_count": count,
		"message":       fmt.Sprintf("Đã đóng case thành công (%d mẩu tri thức xử lý).", count),
	})
}

func (h *Handler) HandleDeleteCase(c *gin.Context) {
	sessionID := c.Param("session_id")
	err := h.caseUC.DeleteCase(c.Request.Context(), sessionID)
	if err != nil {
  httpx.InternalErrorResp(c, err, "HandleDeleteCase")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Đã xóa case thành công"})
}

type UpdateCustomerRequest struct {
	CustomerName  string `json:"customer_name"`
	CustomerPhone string `json:"customer_phone"`
}

func (h *Handler) HandleUpdateCaseCustomer(c *gin.Context) {
	sessionID := c.Param("session_id")
	var req UpdateCustomerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "Dữ liệu không hợp lệ"})
		return
	}

	err := h.caseUC.UpdateCustomerInfo(c.Request.Context(), sessionID, req.CustomerName, req.CustomerPhone)
	if err != nil {
  httpx.InternalErrorResp(c, err, "HandleUpdateCaseCustomer")
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Đã cập nhật thông tin khách hàng thành công"})
}

func (h *Handler) HandleClearAllCases(c *gin.Context) {
	err := h.caseUC.ClearAllCases(c.Request.Context())
	if err != nil {
  httpx.InternalErrorResp(c, err, "HandleClearAllCases")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Đã xóa toàn bộ danh sách case thành công"})
}

func (h *Handler) HandleListCustomers(c *gin.Context) {
	search := strings.TrimSpace(c.Query("search"))
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}

	allCustomers, err := h.caseUC.ListCustomers(c.Request.Context())
	if err != nil {
  httpx.InternalErrorResp(c, err, "HandleListCustomers")
		return
	}

	var filtered []*domain.CustomerProfile
	if search != "" {
		sLower := strings.ToLower(search)
		for _, cust := range allCustomers {
			if strings.Contains(strings.ToLower(cust.DisplayName), sLower) ||
				strings.Contains(strings.ToLower(cust.Phone), sLower) ||
				strings.Contains(strings.ToLower(cust.GuestID), sLower) ||
				strings.Contains(strings.ToLower(cust.LastMessage), sLower) {
				filtered = append(filtered, cust)
			}
		}
	} else {
		filtered = allCustomers
	}

	total := int64(len(filtered))
	totalPages := int(math.Ceil(float64(total) / float64(limit)))
	if totalPages < 1 {
		totalPages = 1
	}

	startIndex := (page - 1) * limit
	var pagedCustomers []*domain.CustomerProfile
	if startIndex < len(filtered) {
		endIndex := startIndex + limit
		if endIndex > len(filtered) {
			endIndex = len(filtered)
		}
		pagedCustomers = filtered[startIndex:endIndex]
	} else {
		pagedCustomers = []*domain.CustomerProfile{}
	}

	c.JSON(http.StatusOK, gin.H{
		"customers":   pagedCustomers,
		"total":       total,
		"page":        page,
		"limit":       limit,
		"total_pages": totalPages,
	})
}

func (h *Handler) HandleUpdateCustomer(c *gin.Context) {
	guestID := c.Param("guest_id")
	var req UpdateCustomerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "Dữ liệu không hợp lệ"})
		return
	}

	err := h.caseUC.UpdateCustomer(c.Request.Context(), guestID, req.CustomerName, req.CustomerPhone)
	if err != nil {
  httpx.InternalErrorResp(c, err, "HandleUpdateCustomer")
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Đã cập nhật thông tin khách hàng thành công"})
}

func (h *Handler) HandleDeleteCustomer(c *gin.Context) {
	guestID := c.Param("guest_id")
	err := h.caseUC.DeleteCustomer(c.Request.Context(), guestID)
	if err != nil {
  httpx.InternalErrorResp(c, err, "HandleDeleteCustomer")
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Đã xóa khách hàng thành công"})
}

// ============================================================
// Continuous Learning Handlers
// ============================================================

func (h *Handler) HandleListPendingLearning(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}

	items, err := h.learningUC.ListPending(c.Request.Context())
	if err != nil {
  httpx.InternalErrorResp(c, err, "HandleListPendingLearning")
		return
	}

	total := int64(len(items))
	totalPages := int(math.Ceil(float64(total) / float64(limit)))
	if totalPages < 1 {
		totalPages = 1
	}

	startIndex := (page - 1) * limit
	var pagedItems []*domain.LearningItem
	if startIndex < len(items) {
		endIndex := startIndex + limit
		if endIndex > len(items) {
			endIndex = len(items)
		}
		pagedItems = items[startIndex:endIndex]
	} else {
		pagedItems = []*domain.LearningItem{}
	}

	c.JSON(http.StatusOK, gin.H{
		"pending_items": pagedItems,
		"items":         pagedItems,
		"total":         total,
		"page":          page,
		"limit":         limit,
		"total_pages":   totalPages,
	})
}

type UpdateLearningRequest struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

func (h *Handler) HandleUpdateLearning(c *gin.Context) {
	idStr := c.Param("item_id")
	id, _ := strconv.ParseInt(idStr, 10, 64)

	var req UpdateLearningRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "Dữ liệu không hợp lệ"})
		return
	}

	err := h.learningUC.UpdateContent(c.Request.Context(), id, req.Question, req.Answer)
	if err != nil {
  httpx.InternalErrorResp(c, err, "HandleUpdateLearning")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Đã cập nhật nội dung mẩu tri thức"})
}

func (h *Handler) HandleApproveLearning(c *gin.Context) {
	idStr := c.Param("item_id")
	id, _ := strconv.ParseInt(idStr, 10, 64)
	user := c.MustGet("user").(*domain.SessionUser)

	var req UpdateLearningRequest
	_ = c.ShouldBindJSON(&req)

	approverName := user.FullName
	if approverName == "" {
		approverName = user.Username
	}

	var err error
	if req.Question != "" && req.Answer != "" {
		err = h.learningUC.ApproveWithContent(c.Request.Context(), id, approverName, req.Question, req.Answer)
	} else {
		err = h.learningUC.Approve(c.Request.Context(), id, approverName)
	}

	if err != nil {
  httpx.InternalErrorResp(c, err, "HandleApproveLearning")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": fmt.Sprintf("Đã phê duyệt và nạp tri thức vào Qdrant thành công bởi %s!", approverName)})
}

func (h *Handler) HandleRejectLearning(c *gin.Context) {
	idStr := c.Param("item_id")
	id, _ := strconv.ParseInt(idStr, 10, 64)
	user := c.MustGet("user").(*domain.SessionUser)

	approverName := user.FullName
	if approverName == "" {
		approverName = user.Username
	}

	err := h.learningUC.Reject(c.Request.Context(), id, approverName)
	if err != nil {
  httpx.InternalErrorResp(c, err, "HandleRejectLearning")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Đã từ chối mẩu tri thức"})
}

func (h *Handler) HandleGetLearningSettings(c *gin.Context) {
	enabled, _ := h.learningUC.GetSettings(c.Request.Context())
	c.JSON(http.StatusOK, gin.H{"auto_learning_enabled": enabled})
}

func (h *Handler) HandleSetLearningSettings(c *gin.Context) {
	var req struct {
		AutoLearningEnabled bool `json:"auto_learning_enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "Dữ liệu không hợp lệ"})
		return
	}

	_ = h.learningUC.SetSettings(c.Request.Context(), req.AutoLearningEnabled)
	c.JSON(http.StatusOK, gin.H{
		"success":               true,
		"auto_learning_enabled": req.AutoLearningEnabled,
		"message":               "Đã cập nhật cài đặt học tri thức tự động.",
	})
}

func (h *Handler) HandleResetLearnedKnowledge(c *gin.Context) {
	count, err := h.learningUC.ResetLearnedKnowledge(c.Request.Context())
	if err != nil {
  httpx.InternalErrorResp(c, err, "HandleResetLearnedKnowledge")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"deleted_count": count,
		"message":       fmt.Sprintf("Đã đặt lại toàn bộ tri thức đã học (%d mẩu đã xóa khỏi Qdrant).", count),
	})
}

// ============================================================
// Knowledge Base Handlers
// ============================================================

func (h *Handler) HandleGetKnowledgeOverview(c *gin.Context) {
	var totalChunks int64
	if h.vectorStore != nil {
		totalChunks, _ = h.vectorStore.Count(c.Request.Context())
	}

	_ = os.MkdirAll(h.docsDir, 0755)
	files, _ := filepath.Glob(filepath.Join(h.docsDir, "*.docx"))
	docList := make([]gin.H, 0, len(files))

	for _, fpath := range files {
		info, err := os.Stat(fpath)
		if err == nil {
			sizeKB := float64(info.Size()) / 1024.0
			docList = append(docList, gin.H{
				"filename": filepath.Base(fpath),
				"size_kb":  fmt.Sprintf("%.1f", sizeKB),
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"total_chunks":    totalChunks,
		"total_documents": len(docList),
		"documents":       docList,
	})
}

func (h *Handler) HandleUploadDocument(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Vui lòng chọn file tải lên"})
		return
	}

	// 1. Size limit
	if file.Size > security.MaxDocUploadSize {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":       "file too large",
			"max_size_mb": security.MaxDocUploadSize / (1024 * 1024),
		})
		return
	}

	// 2. Sanitize filename
	safeName, err := security.ValidateAndSanitizeFilename(file.Filename)
	if err != nil {
  httpx.BadRequestResp(c, err)
		return
	}

	// 3. Extension whitelist
	if !strings.EqualFold(filepath.Ext(safeName), ".docx") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "only .docx files are allowed"})
		return
	}

	// 4. MIME magic bytes (ZIP signature for .docx)
	f, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot read file"})
		return
	}
	if err := security.ValidateDOCXMagicBytes(f); err != nil {
		f.Close()
  httpx.BadRequestResp(c, err)
		return
	}
	f.Close()

	// 5. Ensure directory exists + prefix check
	if err := security.EnsureDirExists(h.docsDir); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create upload directory"})
		return
	}
	savePath := filepath.Join(h.docsDir, safeName)
	if err := security.CheckPrefix(h.docsDir, savePath); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file path"})
		return
	}

	// 6. Save file
	if err := c.SaveUploadedFile(file, savePath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save file"})
		return
	}

	// TODO(security): integrate virus scanner here before processing
	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"filename": safeName,
		"message":  fmt.Sprintf("Đã tải lên file '%s' thành công.", safeName),
	})
}

// ============================================================
// Analytics & Config Handlers
// ============================================================

func (h *Handler) HandleGetAnalytics(c *gin.Context) {
	stats, err := h.analyticsUC.GetDashboardStats(c.Request.Context())
	if err != nil {
  httpx.InternalErrorResp(c, err, "HandleGetAnalytics")
		return
	}
	c.JSON(http.StatusOK, stats)
}

func (h *Handler) HandleGetConfig(c *gin.Context) {
	prompt, model, temp, _ := h.analyticsUC.GetSystemConfig(c.Request.Context(), "", "claude-haiku-4-5-20251001", 0.1)
	c.JSON(http.StatusOK, gin.H{
		"system_prompt": prompt,
		"llm_model":     model,
		"temperature":   temp,
	})
}

type ConfigSaveRequest struct {
	SystemPrompt string  `json:"system_prompt" binding:"required"`
	LLMModel     string  `json:"llm_model" binding:"required"`
	Temperature  float64 `json:"temperature"`
}

func (h *Handler) HandleSaveConfig(c *gin.Context) {
	var req ConfigSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "Dữ liệu cấu hình không hợp lệ"})
		return
	}

	err := h.analyticsUC.SaveSystemConfig(c.Request.Context(), req.SystemPrompt, req.LLMModel, req.Temperature)
	if err != nil {
  httpx.InternalErrorResp(c, err, "HandleSaveConfig")
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Đã lưu cấu hình hệ thống thành công."})
}

// ============================================================
// Voice Call Handlers
// ============================================================

type InitiateCallRequest struct {
	SessionID  string `json:"session_id" binding:"required"`
	CallerType string `json:"caller_type" binding:"required"`
	CallerID   string `json:"caller_id" binding:"required"`
	CalleeType string `json:"callee_type" binding:"required"`
	CalleeID   string `json:"callee_id"`
}

func (h *Handler) HandleInitiateCall(c *gin.Context) {
	var req InitiateCallRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "Dữ liệu cuộc gọi không hợp lệ"})
		return
	}

	call, err := h.voiceUC.InitiateCall(
		c.Request.Context(),
		req.SessionID,
		domain.CallerType(req.CallerType),
		req.CallerID,
		domain.CallerType(req.CalleeType),
		req.CalleeID,
	)
	if err != nil {
  httpx.InternalErrorResp(c, err, "HandleInitiateCall")
		return
	}

	c.JSON(http.StatusOK, call)
}

type EndCallRequest struct {
	CallID          int64  `json:"call_id" binding:"required"`
	SessionID       string `json:"session_id" binding:"required"`
	DurationSeconds int    `json:"duration_seconds"`
	RecordingURL    string `json:"recording_url"`
}

func (h *Handler) HandleEndCall(c *gin.Context) {
	var req EndCallRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "Dữ liệu kết thúc cuộc gọi không hợp lệ"})
		return
	}

	err := h.voiceUC.EndCall(c.Request.Context(), req.CallID, req.SessionID, req.DurationSeconds, req.RecordingURL)
	if err != nil {
  httpx.InternalErrorResp(c, err, "HandleEndCall")
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Cuộc gọi đã kết thúc"})
}

func (h *Handler) HandleUploadRecording(c *gin.Context) {
	file, err := c.FormFile("audio")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Không có file ghi âm"})
		return
	}

	// 1. Size check
	if file.Size > security.MaxAudioUploadSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file too large (max 100MB)"})
		return
	}

	// 2. Extension whitelist
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if !security.AllowedAudioExtensions[ext] {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":        "only audio files allowed: webm, ogg, wav, mp3, m4a",
			"allowed_exts": []string{".webm", ".ogg", ".wav", ".mp3", ".m4a"},
		})
		return
	}

	// 3. MIME magic bytes
	f, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot read file"})
		return
	}
	if err := security.ValidateAudioMagicBytes(f); err != nil {
		f.Close()
  httpx.BadRequestResp(c, err)
		return
	}
	f.Close()

	// 4. Server-generated filename (never trust user input)
	filename, err := security.GenerateSecureFilename("call", ext[1:]) // ext[1:] strips leading dot
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate filename"})
		return
	}

	// 5. Ensure directory + prefix check
	recordingsDir := filepath.Join(h.docsDir, "..", "recordings")
	if err := security.EnsureDirExists(recordingsDir); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create recordings directory"})
		return
	}
	savePath := filepath.Join(recordingsDir, filename)
	absDir, _ := filepath.Abs(recordingsDir)
	if err := security.CheckPrefix(absDir, savePath); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file path"})
		return
	}

	// 6. Save file
	if err := c.SaveUploadedFile(file, savePath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save recording"})
		return
	}

	recordingURL := "/static/recordings/" + filename

	// Parse form fields AFTER file save
	sessionID := c.PostForm("session_id")
	callIDStr := c.PostForm("call_id")
	durStr := c.PostForm("duration_seconds")
	durationSeconds, _ := strconv.Atoi(durStr)

	// Update call record in database
	var callID int64
	if callIDStr != "" {
		callID, _ = strconv.ParseInt(callIDStr, 10, 64)
	}

	if callID > 0 || sessionID != "" {
		if callID == 0 && sessionID != "" {
			calls, _ := h.voiceUC.GetCallsBySession(c.Request.Context(), sessionID)
			if len(calls) > 0 {
				callID = calls[0].ID
			}
		}
		if callID > 0 {
			_ = h.voiceUC.EndCall(c.Request.Context(), callID, sessionID, durationSeconds, recordingURL)
		}
	}

	transcript := strings.TrimSpace(c.PostForm("transcript"))

	// Automatic Q&A Learning extraction from Voice Call to Continuous Learning Queue
	if sessionID != "" {
		go func(sID, recURL, trans, filePath string, durSec int, cID int64) {
			ctx := context.Background()
			custName := "Khách hàng"
			chatCase, _ := h.caseUC.GetCase(ctx, sID)
			if chatCase != nil && chatCase.CustomerName != "" && chatCase.CustomerName != "cskh01" && chatCase.CustomerName != "cskh" {
				custName = chatCase.CustomerName
			}

			// If client transcript is empty, try backend AI transcription on the saved audio file
			if trans == "" && filePath != "" {
				trans = transcribeAudioFile(filePath)
			}

			// Save transcript to call record if available
			if trans != "" && cID > 0 {
				_ = h.voiceUC.SetTranscript(ctx, cID, trans)
			}

			question, answer := extractQAFromVoiceTranscript(custName, trans, durSec)
			_, _ = h.learningUC.UpsertVoiceLearning(ctx, sID, question, answer, durSec)
			_ = h.eventBus.PublishWS(ctx, "admin_inbox", domain.WSEventCaseUpdate, map[string]interface{}{
				"type":       "new_learning_from_call",
				"session_id": sID,
			}, "system")
		}(sessionID, recordingURL, transcript, savePath, durationSeconds, callID)
	}

	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"recording_url": recordingURL,
		"message":       "Ghi âm cuộc gọi đã được lưu và tự động bóc tách nội dung đưa vào hàng chờ Học Tri Thức Mới",
	})
}

func transcribeAudioFile(filePath string) string {
	audioBytes, err := os.ReadFile(filePath)
	if err != nil || len(audioBytes) == 0 {
		return ""
	}

	geminiKey := os.Getenv("GEMINI_API_KEY")
	if geminiKey != "" {
		b64Data := base64.StdEncoding.EncodeToString(audioBytes)
		reqBody := map[string]interface{}{
			"contents": []map[string]interface{}{
				{
					"parts": []map[string]interface{}{
						{
							"inline_data": map[string]string{
								"mime_type": "audio/webm",
								"data":      b64Data,
							},
						},
						{
							"text": "Hãy nghe đoạn ghi âm cuộc gọi này và chép lại toàn bộ văn bản nội dung cuộc trò chuyện giữa khách hàng và chuyên viên CSKH bằng tiếng Việt.",
						},
					},
				},
			},
		}
		bodyBytes, err := json.Marshal(reqBody)
		if err == nil {
			url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/gemini-1.5-flash:generateContent?key=%s", geminiKey)
			client := &http.Client{Timeout: 20 * time.Second}
			httpReq, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
			if err == nil {
				httpReq.Header.Set("Content-Type", "application/json")
				resp, err := client.Do(httpReq)
				if err == nil {
					defer resp.Body.Close()
					respBytes, _ := io.ReadAll(resp.Body)
					if resp.StatusCode == http.StatusOK {
						var geminiResp struct {
							Candidates []struct {
								Content struct {
									Parts []struct {
										Text string `json:"text"`
									} `json:"parts"`
								} `json:"content"`
							} `json:"candidates"`
						}
						if err := json.Unmarshal(respBytes, &geminiResp); err == nil && len(geminiResp.Candidates) > 0 {
							var sb strings.Builder
							for _, p := range geminiResp.Candidates[0].Content.Parts {
								sb.WriteString(p.Text)
							}
							return strings.TrimSpace(sb.String())
						}
					}
				}
			}
		}
	}
	return ""
}

func extractQAFromVoiceTranscript(customerName, transcript string, durationSeconds int) (string, string) {
	trimmed := strings.TrimSpace(transcript)

	// 1. If we have actual spoken transcript from audio transcription or speech recognition
	if len(trimmed) > 5 {
		question := fmt.Sprintf("Nội dung tư vấn cuộc gọi thoại với khách hàng %s", customerName)
		answer := fmt.Sprintf("Văn bản lời thoại ghi âm cuộc gọi (%d giây):\n\n%s", durationSeconds, trimmed)
		return question, answer
	}

	// 2. Fallback when speech was not recognized / silent call
	question := fmt.Sprintf("Nội dung cuộc gọi tư vấn với khách hàng %s (%d giây)", customerName, durationSeconds)
	answer := fmt.Sprintf("Cuộc gọi thoại đàm thoại %d giây. Bản ghi âm chưa nhận diện được văn bản lời thoại. Chuyên viên CSKH vui lòng nhập/chỉnh sửa nội dung tư vấn thực tế tại đây trước khi phê duyệt cho AI học.", durationSeconds)
	return question, answer
}

func (h *Handler) HandleGetCalls(c *gin.Context) {
	sessionID := c.Query("session_id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}

	var allCalls []*domain.VoiceCall
	var err error

	if sessionID != "" {
		allCalls, err = h.voiceUC.GetCallsBySession(c.Request.Context(), sessionID)
	} else {
		allCalls, err = h.voiceUC.ListAllCalls(c.Request.Context())
	}

	if err != nil {
  httpx.InternalErrorResp(c, err, "HandleGetCalls")
		return
	}
	if allCalls == nil {
		allCalls = []*domain.VoiceCall{}
	}

	total := int64(len(allCalls))
	totalPages := int(math.Ceil(float64(total) / float64(limit)))
	if totalPages < 1 {
		totalPages = 1
	}

	startIndex := (page - 1) * limit
	var pagedCalls []*domain.VoiceCall
	if startIndex < len(allCalls) {
		endIndex := startIndex + limit
		if endIndex > len(allCalls) {
			endIndex = len(allCalls)
		}
		pagedCalls = allCalls[startIndex:endIndex]
	} else {
		pagedCalls = []*domain.VoiceCall{}
	}

	c.JSON(http.StatusOK, gin.H{
		"calls":       pagedCalls,
		"total":       total,
		"page":        page,
		"limit":       limit,
		"total_pages": totalPages,
	})
}

func (h *Handler) HandleDeleteCall(c *gin.Context) {
	callIDStr := c.Param("call_id")
	callID, err := strconv.ParseInt(callIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "ID cuộc gọi không hợp lệ"})
		return
	}

	if err := h.voiceUC.DeleteCall(c.Request.Context(), callID); err != nil {
  httpx.InternalErrorResp(c, err, "HandleDeleteCall")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Đã xóa lịch sử cuộc gọi thành công",
	})
}
