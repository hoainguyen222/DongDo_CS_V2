package http

import (
	"context"
	"fmt"
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
	calluc "github.com/hoainguyen222/DongDo_CS_V2/internal/usecase/call"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/usecase"
	"github.com/rs/zerolog"
)

// Logger is a global logger instance for the http package
var Logger zerolog.Logger

// InitLogger initializes the global zerolog logger
func InitLogger(level string) {
	zerolog.TimeFieldFormat = time.RFC3339
	zerolog.TimestampFieldName = "timestamp"
	zerolog.LevelFieldName = "level"
	zerolog.MessageFieldName = "message"

	lvl, err := zerolog.ParseLevel(strings.ToLower(level))
	if err != nil {
		lvl = zerolog.InfoLevel
	}

	Logger = zerolog.New(os.Stdout).
		With().
		Timestamp().
		Str("service", "http-handler").
		Logger().
		Level(lvl)
}

// getClientIP was removed for log reduction (callers no longer need it).

type Handler struct {
	authUC      *usecase.AuthUseCase
	chatUC      *usecase.ChatUseCase
	caseUC      *usecase.CaseUseCase
	learningUC  *usecase.LearningUseCase
	voiceUC     *usecase.VoiceUseCase
	analyticsUC *usecase.AnalyticsUseCase
	partnerUC   *usecase.PartnerUseCase
	ragUC       *usecase.RAGUseCase
	tagUC       *usecase.ChatTagUseCase
	vectorStore domain.VectorStore
	embedder    domain.Embedder
	docsDir     string
	eventBus    domain.EventBus
	logger      zerolog.Logger
	// callUC is used only to flip an authenticated staff/agent into the
	// Redis "available" pool automatically — see HandleLogin/HandleLogout.
	// Nil is safe: it disables auto-availability (used in some tests).
	callUC *calluc.UseCase
}

func NewHandler(
	authUC *usecase.AuthUseCase,
	chatUC *usecase.ChatUseCase,
	caseUC *usecase.CaseUseCase,
	learningUC *usecase.LearningUseCase,
	voiceUC *usecase.VoiceUseCase,
	analyticsUC *usecase.AnalyticsUseCase,
	partnerUC *usecase.PartnerUseCase,
	ragUC *usecase.RAGUseCase,
	tagUC *usecase.ChatTagUseCase,
	vectorStore domain.VectorStore,
	embedder domain.Embedder,
	docsDir string,
	eventBus domain.EventBus,
	callUC *calluc.UseCase,
) *Handler {
	return &Handler{
		authUC:      authUC,
		chatUC:      chatUC,
		caseUC:      caseUC,
		learningUC:  learningUC,
		voiceUC:     voiceUC,
		analyticsUC: analyticsUC,
		partnerUC:   partnerUC,
		ragUC:       ragUC,
		tagUC:       tagUC,
		vectorStore: vectorStore,
		embedder:    embedder,
		docsDir:     docsDir,
		eventBus:    eventBus,
		callUC:      callUC,
		logger:      Logger.With().Str("component", "handler").Logger(),
	}
}

// ============================================================
// Auth & Guest Handlers
// ============================================================

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// isAgentRole reports whether the given role can answer customer calls in
// the Call v2 routing pool. Owners, admins, leaders, and CSKH staff are
// all eligible; customers and unauthenticated visitors are not.
func isAgentRole(role domain.UserRole) bool {
	switch role {
	case domain.RoleOwner, domain.RoleAdmin, domain.RoleLeader, domain.RoleCSKH:
		return true
	}
	return false
}

// markAgentAutoAvailable puts the user into the Redis AVAILABLE pool so
// the routing worker can immediately reserve them. Failures are logged
// and swallowed — login must never fail because of an availability flip.
func (h *Handler) markAgentAutoAvailable(ctx context.Context, username string) {
	if h.callUC == nil {
		return
	}
	if err := h.callUC.SetAgentStatusPublic(ctx, username, domain.AgentAvailable); err != nil {
		Logger.Warn().Err(err).Str("user", username).Msg("auto-AVAILABLE failed; agent will not receive routed calls")
		return
	}
	if err := h.callUC.HeartbeatAgentPublic(ctx, username); err != nil {
		Logger.Warn().Err(err).Str("user", username).Msg("agent heartbeat (auto) failed")
	}
}

// markAgentAutoOffline removes the user from the pool on logout.
func (h *Handler) markAgentAutoOffline(ctx context.Context, username string) {
	if h.callUC == nil {
		return
	}
	if err := h.callUC.SetAgentStatusPublic(ctx, username, domain.AgentOffline); err != nil {
		Logger.Warn().Err(err).Str("user", username).Msg("auto-OFFLINE on logout failed")
	}
}

func (h *Handler) HandleLogin(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Logger.Warn().Err(err).Msg("Login validation failed")
		c.JSON(http.StatusBadRequest, gin.H{"detail": "Vui lòng nhập đầy đủ tên đăng nhập và mật khẩu"})
		return
	}

	req.Username = strings.ToLower(strings.TrimSpace(req.Username))
	user, err := h.authUC.Login(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		Logger.Warn().Str("username", req.Username).Err(err).Msg("Login failed")
		c.JSON(http.StatusUnauthorized, gin.H{"detail": err.Error()})
		return
	}

	Logger.Info().Str("user", user.Username).Msg("Login success")

	// Auto-AVAILABLE for staff+ roles so the call router can pick them up
	// without any extra UI action. The state is held in Redis and a
	// periodic heartbeat from the admin client keeps it fresh. Logout
	// (or explicit SetAgentStatus OFFLINE) removes them from the pool.
	if isAgentRole(user.Role) {
		h.markAgentAutoAvailable(c.Request.Context(), user.Username)
	}

	c.JSON(http.StatusOK, gin.H{
		"token":     user.Token,
		"username":  user.Username,
		"full_name": user.FullName,
		"role":      user.Role,
	})
}

func (h *Handler) HandleGetMe(c *gin.Context) {
	user := c.MustGet("user").(*domain.SessionUser)
	c.JSON(http.StatusOK, gin.H{
		"username":  user.Username,
		"full_name": user.FullName,
		"role":      user.Role,
	})
}

func (h *Handler) HandleLogout(c *gin.Context) {
	user := c.MustGet("user").(*domain.SessionUser)
	_ = h.authUC.Logout(c.Request.Context(), user.Token)
	if isAgentRole(user.Role) {
		h.markAgentAutoOffline(c.Request.Context(), user.Username)
	}
	c.JSON(http.StatusOK, gin.H{"message": "Đã đăng xuất thành công."})
}

type CreateUserReq struct {
	FullName string `json:"full_name"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	Password string `json:"password"`
}

func (h *Handler) HandleListUsers(c *gin.Context) {
	users, err := h.authUC.ListUsers(c.Request.Context())
	if err != nil {
		Logger.Error().Err(err).Msg("Failed to list users")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Lỗi lấy danh sách tài khoản: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"users": users})
}

func (h *Handler) HandleCreateUser(c *gin.Context) {
	var req CreateUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Logger.Warn().Err(err).Msg("Create user validation failed")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu không hợp lệ"})
		return
	}

	var role domain.UserRole
	rLower := strings.ToLower(strings.TrimSpace(req.Role))
	if strings.Contains(rLower, "owner") {
		role = domain.RoleOwner
	} else if strings.Contains(rLower, "admin") {
		role = domain.RoleAdmin
	} else if strings.Contains(rLower, "leader") {
		role = domain.RoleLeader
	} else {
		role = domain.RoleCSKH
	}

	performer := c.MustGet("user").(*domain.SessionUser)
	if role == domain.RoleOwner && strings.ToLower(string(performer.Role)) != "owner" {
		Logger.Warn().Msg("Create user forbidden: insufficient permissions for owner role")
		c.JSON(http.StatusForbidden, gin.H{"error": "Chỉ có tài khoản Owner mới có quyền cấp vai trò Owner cho người khác!"})
		return
	}

	username := strings.ToLower(strings.TrimSpace(req.Email))
	if username == "" {
		Logger.Warn().Msg("Create user validation failed: email/username is empty")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email/Username không được để trống"})
		return
	}
	if req.Password == "" {
		req.Password = "12345678"
	}

	user, err := h.authUC.CreateUser(c.Request.Context(), username, req.Password, req.FullName, role)
	if err != nil {
		Logger.Error().Str("username", username).Err(err).Msg("Failed to create user")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	Logger.Info().Str("user", user.Username).Msg("User created")

	c.JSON(http.StatusOK, user)
}

func (h *Handler) HandleDeleteUser(c *gin.Context) {
	username := c.Param("username")
	if username == "" {
		Logger.Warn().Msg("Delete user validation failed: username is empty")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tên đăng nhập không hợp lệ"})
		return
	}
	performer := c.MustGet("user").(*domain.SessionUser)
	if err := h.authUC.DeleteUser(c.Request.Context(), string(performer.Role), username); err != nil {
		Logger.Error().Str("target", username).Err(err).Msg("Failed to delete user")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	Logger.Info().Str("user", username).Msg("User deleted")

	c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Đã xóa tài khoản thành công"})
}

type UpdateUserReq struct {
	FullName string `json:"full_name"`
	Role     string `json:"role"`
	IsActive bool   `json:"is_active"`
	Password string `json:"password"`
}

func (h *Handler) HandleUpdateUser(c *gin.Context) {
	username := c.Param("username")
	if username == "" {
		Logger.Warn().Msg("Update user validation failed: username is empty")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tên đăng nhập không hợp lệ"})
		return
	}

	var req UpdateUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		Logger.Warn().Err(err).Msg("Update user validation failed")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu không hợp lệ"})
		return
	}

	var role domain.UserRole
	rLower := strings.ToLower(strings.TrimSpace(req.Role))
	if strings.Contains(rLower, "owner") {
		role = domain.RoleOwner
	} else if strings.Contains(rLower, "admin") {
		role = domain.RoleAdmin
	} else if strings.Contains(rLower, "leader") {
		role = domain.RoleLeader
	} else {
		role = domain.RoleCSKH
	}

	performer := c.MustGet("user").(*domain.SessionUser)
	if role == domain.RoleOwner && strings.ToLower(string(performer.Role)) != "owner" {
		Logger.Warn().Msg("Update user forbidden: insufficient permissions for owner role")
		c.JSON(http.StatusForbidden, gin.H{"error": "Chỉ có tài khoản Owner mới có quyền cấp vai trò Owner cho người khác!"})
		return
	}

	user, err := h.authUC.UpdateUser(c.Request.Context(), string(performer.Role), username, req.FullName, role, req.IsActive, req.Password)
	if err != nil {
		Logger.Error().Str("target", username).Err(err).Msg("Failed to update user")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	Logger.Info().Str("user", user.Username).Msg("User updated")

	c.JSON(http.StatusOK, user)
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
		Logger.Error().Err(err).Msg("Failed to register guest")
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "Lỗi tạo phiên khách hàng: " + err.Error()})
		return
	}

	sessionID := "session-" + guest.GuestID.String()[:8] + "-" + strconv.FormatInt(time.Now().UnixMilli(), 10)

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
// Chat Handlers
// ============================================================

type ChatRequest struct {
	SessionID    string     `json:"session_id"`
	CustomerName string     `json:"customer_name"`
	Message      string     `json:"message" binding:"required"`
	ClientMsgID  *uuid.UUID `json:"client_msg_id"`
}

func (h *Handler) HandleChat(c *gin.Context) {
	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Logger.Warn().Err(err).Msg("Chat validation failed: message is empty")
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
		Logger.Error().Str("session_id", sessionID).Err(err).Msg("Failed to send guest message")
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
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
		Logger.Error().Str("session_id", sessionID).Err(err).Msg("Failed to get chat history")
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
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

	// Page + filter + search are pushed into SQL via CaseRepository.ListPage,
	// so we never load the full chat_cases table into Go memory. LastSenderType
	// is enriched inside the repo using a single bulk DISTINCT ON query — no N+1.
	pagedCases, total, err := h.caseUC.ListCasesPage(c.Request.Context(), domain.ListCasesParams{
		Page:     page,
		PageSize: limit,
		Status:   statusFilter,
		Search:   search,
	})
	if err != nil {
		Logger.Error().Err(err).Msg("Failed to list cases")
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}

	totalPages := int(math.Ceil(float64(total) / float64(limit)))
	if totalPages < 1 {
		totalPages = 1
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
		Logger.Error().Str("session_id", sessionID).Err(err).Msg("Failed to take case")
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}

	Logger.Info().Str("session_id", sessionID).Str("user", user.Username).Msg("Case taken")

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
		Logger.Warn().Str("session_id", sessionID).Err(err).Msg("Reply case validation failed")
		c.JSON(http.StatusBadRequest, gin.H{"detail": "Tin nhắn không được để trống"})
		return
	}

	_, err := h.chatUC.SendCSReply(c.Request.Context(), sessionID, user.Username, user.FullName, req.Message)
	if err != nil {
		Logger.Error().Str("session_id", sessionID).Err(err).Msg("Failed to send CS reply")
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Đã gửi tin nhắn thành công"})
}

type ResolveRequest struct {
	ResolutionNote string          `json:"resolution_note"`
	ExtractPairs   []domain.QAPair `json:"extract_pairs"`
}

func (h *Handler) HandleResolveCase(c *gin.Context) {
	sessionID := c.Param("session_id")
	user := c.MustGet("user").(*domain.SessionUser)

	var req ResolveRequest
	_ = c.ShouldBindJSON(&req)

	autoLearned, count, err := h.caseUC.ResolveCase(c.Request.Context(), sessionID, user.Username, user.FullName, req.ResolutionNote, req.ExtractPairs)
	if err != nil {
		Logger.Error().Str("session_id", sessionID).Err(err).Msg("Failed to resolve case")
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}

	Logger.Info().Str("session_id", sessionID).Str("user", user.Username).Int("learned_count", count).Msg("Case resolved")

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
		Logger.Error().Str("session_id", sessionID).Err(err).Msg("Failed to delete case")
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}

	Logger.Info().Str("session_id", sessionID).Msg("Case deleted")

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
		Logger.Warn().Str("session_id", sessionID).Err(err).Msg("Update case customer validation failed")
		c.JSON(http.StatusBadRequest, gin.H{"detail": "Dữ liệu không hợp lệ"})
		return
	}

	err := h.caseUC.UpdateCustomerInfo(c.Request.Context(), sessionID, req.CustomerName, req.CustomerPhone)
	if err != nil {
		Logger.Error().Str("session_id", sessionID).Err(err).Msg("Failed to update case customer")
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Đã cập nhật thông tin khách hàng thành công"})
}

func (h *Handler) HandleClearAllCases(c *gin.Context) {
	err := h.caseUC.ClearAllCases(c.Request.Context())
	if err != nil {
		Logger.Error().Err(err).Msg("Failed to clear all cases")
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}

	Logger.Warn().Msg("All cases cleared")

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
		Logger.Error().Err(err).Msg("Failed to list customers")
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
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
		Logger.Warn().Str("guest_id", guestID).Err(err).Msg("Update customer validation failed")
		c.JSON(http.StatusBadRequest, gin.H{"detail": "Dữ liệu không hợp lệ"})
		return
	}

	err := h.caseUC.UpdateCustomer(c.Request.Context(), guestID, req.CustomerName, req.CustomerPhone)
	if err != nil {
		Logger.Error().Str("guest_id", guestID).Err(err).Msg("Failed to update customer")
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Đã cập nhật thông tin khách hàng thành công"})
}

func (h *Handler) HandleDeleteCustomer(c *gin.Context) {
	guestID := c.Param("guest_id")
	err := h.caseUC.DeleteCustomer(c.Request.Context(), guestID)
	if err != nil {
		Logger.Error().Str("guest_id", guestID).Err(err).Msg("Failed to delete customer")
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
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
		Logger.Error().Err(err).Msg("Failed to list pending learning items")
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
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
		Logger.Warn().Int64("item_id", id).Err(err).Msg("Update learning validation failed")
		c.JSON(http.StatusBadRequest, gin.H{"detail": "Dữ liệu không hợp lệ"})
		return
	}

	err := h.learningUC.UpdateContent(c.Request.Context(), id, req.Question, req.Answer)
	if err != nil {
		Logger.Error().Int64("item_id", id).Err(err).Msg("Failed to update learning content")
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
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
		Logger.Error().Int64("item_id", id).Err(err).Msg("Failed to approve learning item")
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
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
		Logger.Error().Int64("item_id", id).Err(err).Msg("Failed to reject learning item")
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
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
		Logger.Warn().Err(err).Msg("Set learning settings validation failed")
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
		Logger.Error().Err(err).Msg("Failed to reset learned knowledge")
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}

	Logger.Warn().Int("deleted_count", count).Msg("Learned knowledge reset")

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
		Logger.Warn().Err(err).Msg("Upload document validation failed: no file selected")
		c.JSON(http.StatusBadRequest, gin.H{"detail": "Vui lòng chọn file tải lên"})
		return
	}

	if !strings.HasSuffix(strings.ToLower(file.Filename), ".docx") {
		Logger.Warn().Str("filename", file.Filename).Msg("Upload document rejected: invalid file format")
		c.JSON(http.StatusBadRequest, gin.H{"detail": "Chỉ hỗ trợ định dạng file Microsoft Word (.docx)"})
		return
	}

	_ = os.MkdirAll(h.docsDir, 0755)
	savePath := filepath.Join(h.docsDir, file.Filename)

	if err := c.SaveUploadedFile(file, savePath); err != nil {
		Logger.Error().Str("filename", file.Filename).Err(err).Msg("Failed to save uploaded document")
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "Lỗi lưu file: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"filename": file.Filename,
		"message":  fmt.Sprintf("Đã tải lên file '%s' thành công.", file.Filename),
	})
}

func (h *Handler) HandleDeleteKnowledgeDocument(c *gin.Context) {
	filename := c.Query("filename")
	if filename == "" {
		// Try to get from path param
		filename = c.Param("filename")
	}

	if filename == "" {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "Tên file không được để trống"})
		return
	}

	// Delete chunks from vector store by source
	deletedChunks := 0
	if h.vectorStore != nil {
		var err error
		deletedChunks, err = h.vectorStore.DeleteBySource(c.Request.Context(), filename)
		if err != nil {
			Logger.Error().Str("filename", filename).Err(err).Msg("Failed to delete chunks from vector store")
			c.JSON(http.StatusInternalServerError, gin.H{"detail": "Lỗi xóa chunks: " + err.Error()})
			return
		}
	}

	// Delete the file from disk
	filePath := filepath.Join(h.docsDir, filename)
	if _, err := os.Stat(filePath); err == nil {
		if err := os.Remove(filePath); err != nil {
			Logger.Error().Str("filename", filename).Err(err).Msg("Failed to delete document file")
			c.JSON(http.StatusInternalServerError, gin.H{"detail": "Lỗi xóa file: " + err.Error()})
			return
		}
	}

	Logger.Info().Str("filename", filename).Int("deleted_chunks", deletedChunks).
		Msg("Knowledge document and its chunks deleted")

	c.JSON(http.StatusOK, gin.H{
		"success":        true,
		"filename":       filename,
		"deleted_chunks": deletedChunks,
		"message":        fmt.Sprintf("Đã xóa tài liệu '%s' và %d chunks khỏi vector store.", filename, deletedChunks),
	})
}

// ============================================================
// Analytics & Config Handlers
// ============================================================

func (h *Handler) HandleGetAnalytics(c *gin.Context) {
	stats, err := h.analyticsUC.GetDashboardStats(c.Request.Context())
	if err != nil {
		Logger.Error().Err(err).Msg("Failed to get analytics")
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
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
		Logger.Warn().Err(err).Msg("Save config validation failed")
		c.JSON(http.StatusBadRequest, gin.H{"detail": "Dữ liệu cấu hình không hợp lệ"})
		return
	}

	err := h.analyticsUC.SaveSystemConfig(c.Request.Context(), req.SystemPrompt, req.LLMModel, req.Temperature)
	if err != nil {
		Logger.Error().Err(err).Msg("Failed to save system config")
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Đã lưu cấu hình hệ thống thành công."})
}

// ============================================================
// Voice Call Handlers — LEGACY (P2P WebRTC)
//
// The following legacy handlers are retained only for backwards-compatible
// audit reads. All call lifecycle endpoints (initiate/end/decline/missed)
// were replaced by the v2 API in internal/delivery/http/call_handler.go
// (AsteriskGateway-backed).  See docs/call-architecture.md.
// ============================================================

type InitiateCallRequest struct { //nolint:unused
	SessionID  string `json:"session_id" binding:"required"`
	CallerType string `json:"caller_type" binding:"required"`
	CallerID   string `json:"caller_id" binding:"required"`
	CalleeType string `json:"callee_type" binding:"required"`
	CalleeID   string `json:"callee_id"`
}

// HandleInitiateCall is disabled in v2. Returns 410 Gone to nudge clients to the new API.
// If you need it back, re-introduce via feature flag.
func (h *Handler) HandleInitiateCall(c *gin.Context) {
	c.JSON(http.StatusGone, gin.H{
		"detail":    "This endpoint is deprecated; use POST /api/calls instead.",
		"new_path":  "/api/calls",
		"migration": "See docs/call-architecture.md §4.",
	})
}

type EndCallRequest struct { //nolint:unused
	CallID          int64  `json:"call_id" binding:"required"`
	SessionID       string `json:"session_id" binding:"required"`
	DurationSeconds int    `json:"duration_seconds"`
	RecordingURL    string `json:"recording_url"`
}

// HandleEndCall is deprecated. Use POST /api/calls/{id}/hangup.
func (h *Handler) HandleEndCall(c *gin.Context) {
	c.JSON(http.StatusGone, gin.H{
		"detail":   "Deprecated. Use POST /api/calls/{id}/hangup.",
		"new_path": "/api/calls/{id}/hangup",
	})
}

type MarkMissedRequest struct { //nolint:unused
	CallID    int64  `json:"call_id" binding:"required"`
	SessionID string `json:"session_id" binding:"required"`
}

// HandleMarkMissedCall is deprecated. ARI events drive missed state in v2.
func (h *Handler) HandleMarkMissedCall(c *gin.Context) {
	c.JSON(http.StatusGone, gin.H{
		"detail": "Deprecated. Call state is now driven by ARI StasisStart/End events.",
	})
}

// HandleUploadRecording is deprecated in v2: Asterisk MixMonitor records
// to /var/spool/asterisk/recordings natively; the ARI RecordingFinished
// event populates call_recordings automatically.
func (h *Handler) HandleUploadRecording(c *gin.Context) {
	c.JSON(http.StatusGone, gin.H{
		"detail": "Deprecated. Asterisk now records natively; see call_recordings table.",
	})
}

func transcribeAudioFile(filePath string) string { //nolint:unused
	return ""
}

func extractQAFromVoiceTranscript(customerName, transcript string, durationSeconds int) (string, string) { //nolint:unused
	trimmed := strings.TrimSpace(transcript)

	if len(trimmed) > 5 {
		question := fmt.Sprintf("Nội dung tư vấn cuộc gọi thoại với khách hàng %s", customerName)
		answer := fmt.Sprintf("Văn bản lời thoại ghi âm cuộc gọi (%d giây):\n\n%s", durationSeconds, trimmed)
		return question, answer
	}

	question := fmt.Sprintf("Nội dung cuộc gọi tư vấn với khách hàng %s (%d giây)", customerName, durationSeconds)
	answer := fmt.Sprintf("Cuộc gọi thoại đàm thoại %d giây.", durationSeconds)
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
		Logger.Error().Err(err).Msg("Failed to get calls")
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
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
		Logger.Warn().Err(err).Msg("Delete call validation failed: invalid call ID")
		c.JSON(http.StatusBadRequest, gin.H{"detail": "ID cuộc gọi không hợp lệ"})
		return
	}

	if err := h.voiceUC.DeleteCall(c.Request.Context(), callID); err != nil {
		Logger.Error().Int64("call_id", callID).Err(err).Msg("Failed to delete call")
		c.JSON(http.StatusInternalServerError, gin.H{"detail": "Lỗi xóa lịch sử cuộc gọi: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Đã xóa lịch sử cuộc gọi thành công",
	})
}

// ============================================================
// Call Signaling & Chat Actions (REST API -> WebSocket)
// ============================================================

// HandleDeclineCall is deprecated in v2: agents reject via POST /api/calls/{id}/reject,
// customers cancel via POST /api/calls/{id}/cancel.
type DeclineCallRequest struct { //nolint:unused
	SessionID string `json:"session_id" binding:"required"`
}

func (h *Handler) HandleDeclineCall(c *gin.Context) {
	c.JSON(http.StatusGone, gin.H{
		"detail": "Deprecated. Use POST /api/calls/{id}/reject (agent) or /cancel (customer).",
	})
}

// HandleSendTyping - Send typing indicator via REST API
type TypingRequest struct {
	SessionID string `json:"session_id" binding:"required"`
}

func (h *Handler) HandleSendTyping(c *gin.Context) {
	var req TypingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Logger.Warn().Err(err).Msg("Send typing validation failed")
		c.JSON(http.StatusBadRequest, gin.H{"detail": "Dữ liệu không hợp lệ"})
		return
	}
	_ = h.eventBus.PublishWS(c.Request.Context(), req.SessionID, domain.WSEventTyping, map[string]interface{}{
		"typing": true,
	}, "guest")

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ============================================================
// System Errors Persistence Handlers
// ============================================================

func (h *Handler) HandleListSystemErrors(c *gin.Context) {
	errors, err := h.partnerUC.ListSystemErrors(c.Request.Context())
	if err != nil {
		Logger.Error().Err(err).Msg("Failed to list system errors")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"errors": errors})
}

func (h *Handler) HandleCreateSystemError(c *gin.Context) {
	var req domain.SystemErrorRecord
	if err := c.ShouldBindJSON(&req); err != nil {
		Logger.Warn().Err(err).Msg("Create system error validation failed")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Dữ liệu không hợp lệ"})
		return
	}

	created, err := h.partnerUC.CreateSystemError(c.Request.Context(), &req)
	if err != nil {
		Logger.Error().Err(err).Msg("Failed to create system error")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	Logger.Warn().Str("error_id", created.ID).Msg("System error created")

	c.JSON(http.StatusOK, created)
}

func (h *Handler) HandleMarkSystemErrorHandled(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		Logger.Warn().Msg("Mark system error handled validation failed: ID is empty")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Mã lỗi không hợp lệ"})
		return
	}

	if err := h.partnerUC.MarkSystemErrorHandled(c.Request.Context(), id); err != nil {
		Logger.Error().Str("error_id", id).Err(err).Msg("Failed to mark system error as handled")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Đã đánh dấu xử lý lỗi"})
}

// ============================================================
// Bootstrap / First-time Setup Handlers
// ============================================================

// HandleBootstrapStatus returns whether the system needs initial setup (no users exist yet).
func (h *Handler) HandleBootstrapStatus(c *gin.Context) {
	users, err := h.authUC.ListUsers(c.Request.Context())
	needsSetup := err == nil && len(users) == 0

	c.JSON(http.StatusOK, gin.H{
		"needs_setup":  needsSetup,
		"is_enabled":   os.Getenv("ENABLE_BOOTSTRAP") == "true",
		"owner_exists": false,
	})
}
