package http

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
	calluc "github.com/hoainguyen222/DongDo_CS_V2/internal/usecase/call"
)

// CallHandler holds the call use case. Separate from the monolithic Handler
// to keep responsibilities clean.
type CallHandler struct {
	uc *calluc.UseCase
}

// NewCallHandler returns a new HTTP handler for the call v2 API.
func NewCallHandler(uc *calluc.UseCase) *CallHandler {
	return &CallHandler{uc: uc}
}

// ---------------------------------------------------------------
// POST /api/calls
// ---------------------------------------------------------------

type createCallRequest struct {
	CustomerID string `json:"customer_id" binding:"required"`
	Priority   int    `json:"priority"`
}

func (h *CallHandler) CreateCall(c *gin.Context) {
	var req createCallRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "customer_id is required"})
		return
	}

	out, err := h.uc.RequestCall(c.Request.Context(), calluc.RequestCallInput{
		CustomerID:     req.CustomerID,
		Priority:       req.Priority,
		IdempotencyKey: c.GetHeader("Idempotency-Key"),
	})
	if err != nil {
		Logger.Error().Err(err).Msg("CreateCall failed")
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}
	status := http.StatusCreated
	if out.Replay {
		c.Header("Idempotent-Replayed", "true")
	}
	c.JSON(status, out)
}

// ---------------------------------------------------------------
// POST /api/calls/:id/accept  (agent)
// ---------------------------------------------------------------

func (h *CallHandler) AcceptCall(c *gin.Context) {
	id, err := parseUUIDParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid call_id"})
		return
	}
	user := mustUser(c)

	if err := h.uc.AcceptCall(c.Request.Context(), calluc.AcceptCallInput{
		CallID:  id,
		AgentID: user.Username,
	}); err != nil {
		writeCallError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Call accepted"})
}

// ---------------------------------------------------------------
// POST /api/calls/:id/reject  (agent)
// ---------------------------------------------------------------

func (h *CallHandler) RejectCall(c *gin.Context) {
	id, err := parseUUIDParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid call_id"})
		return
	}
	user := mustUser(c)

	if err := h.uc.RejectCall(c.Request.Context(), calluc.RejectCallInput{
		CallID:  id,
		AgentID: user.Username,
	}); err != nil {
		writeCallError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Call rejected"})
}

// ---------------------------------------------------------------
// POST /api/calls/:id/hangup  (customer or agent)
// ---------------------------------------------------------------

func (h *CallHandler) HangupCall(c *gin.Context) {
	id, err := parseUUIDParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid call_id"})
		return
	}

	byAgent := false
	userID := c.GetHeader("X-Customer-ID")
	if user, ok := c.Get("user"); ok {
		if u, ok2 := user.(*domain.SessionUser); ok2 {
			byAgent = true
			userID = u.Username
		}
	}

	if err := h.uc.HangupCall(c.Request.Context(), calluc.HangupCallInput{
		CallID:  id,
		ByAgent: byAgent,
		UserID:  userID,
	}); err != nil {
		writeCallError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Call ended"})
}

// ---------------------------------------------------------------
// POST /api/calls/:id/cancel  (customer)
// ---------------------------------------------------------------

func (h *CallHandler) CancelCall(c *gin.Context) {
	id, err := parseUUIDParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid call_id"})
		return
	}
	customerID := c.GetHeader("X-Customer-ID")
	if customerID == "" {
		var req struct {
			CustomerID string `json:"customer_id" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err == nil {
			customerID = req.CustomerID
		}
	}
	if customerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "customer_id required"})
		return
	}

	if err := h.uc.CancelCall(c.Request.Context(), calluc.CancelCallInput{
		CallID:     id,
		CustomerID: customerID,
	}); err != nil {
		writeCallError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Call cancelled"})
}

// ---------------------------------------------------------------
// GET /api/admin/calls — paginated call history (Call v2)
//
// Returns the most recently updated calls (any status, including
// non-terminal) so the admin can see who is ringing, who hung up, and
// who missed. The legacy `/api/admin/voice/calls` endpoint still works
// for the pre-Call-v2 voice_calls audit log.
// ---------------------------------------------------------------

func (h *CallHandler) ListCalls(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	calls, err := h.uc.ListCalls(c.Request.Context(), limit, offset)
	if err != nil {
		writeCallError(c, err)
		return
	}
	if calls == nil {
		calls = []*domain.Call{}
	}
	c.JSON(http.StatusOK, gin.H{
		"calls": calls,
		"page":  page,
		"limit": limit,
	})
}

// ---------------------------------------------------------------
// GET /api/calls/:id
// ---------------------------------------------------------------

func (h *CallHandler) GetCall(c *gin.Context) {
	id, err := parseUUIDParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "invalid call_id"})
		return
	}
	call, err := h.uc.GetCall(c.Request.Context(), id)
	if err != nil {
		writeCallError(c, err)
		return
	}
	if call == nil {
		c.JSON(http.StatusNotFound, gin.H{"detail": "call not found"})
		return
	}
	c.JSON(http.StatusOK, call)
}

// ---------------------------------------------------------------
// POST /api/agents/:id/heartbeat  (called periodically by the agent client)
// ---------------------------------------------------------------

func (h *CallHandler) AgentHeartbeat(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "id required"})
		return
	}
	if err := h.uc.HeartbeatAgentPublic(c.Request.Context(), id); err != nil {
		writeCallError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ---------------------------------------------------------------
// GET /api/agents/:id/active-calls
//
// Returns calls currently assigned to the agent that have not reached a
// terminal state. The admin layout polls this once on mount (and on
// WebSocket reconnect) to recover the ringing banner after a page reload
// — the WS broadcast for the original `incoming_call` is fire-and-forget
// and is missed if the agent was offline at that moment.
// ---------------------------------------------------------------

func (h *CallHandler) GetAgentActiveCalls(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "id required"})
		return
	}
	calls, err := h.uc.GetActiveCallsForAgent(c.Request.Context(), id)
	if err != nil {
		writeCallError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"calls": calls})
}

// ---------------------------------------------------------------
// POST /api/agents/:id/status  (agent toggles AVAILABLE / AWAY / OFFLINE)
// ---------------------------------------------------------------

type agentStatusRequest struct {
	Status string `json:"status" binding:"required"`
}

func (h *CallHandler) SetAgentStatus(c *gin.Context) {
	id := c.Param("id")
	var req agentStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"detail": "status is required"})
		return
	}
	if err := h.uc.SetAgentStatusPublic(c.Request.Context(), id, domain.AgentState(req.Status)); err != nil {
		writeCallError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ---------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------

func parseUUIDParam(c *gin.Context, name string) (uuid.UUID, error) {
	raw := c.Param(name)
	return uuid.Parse(raw)
}

func mustUser(c *gin.Context) *domain.SessionUser {
	if u, ok := c.Get("user"); ok {
		if su, ok2 := u.(*domain.SessionUser); ok2 {
			return su
		}
	}
	return nil
}

func writeCallError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrCallNotFound):
		c.JSON(http.StatusNotFound, gin.H{"detail": "call not found"})
	case errors.Is(err, domain.ErrInvalidTransition):
		c.JSON(http.StatusConflict, gin.H{"detail": "invalid state transition"})
	case errors.Is(err, domain.ErrAgentStaleReservation),
		errors.Is(err, domain.ErrAgentNotAvailable),
		errors.Is(err, domain.ErrAgentNotReserved):
		c.JSON(http.StatusConflict, gin.H{"detail": err.Error()})
	case errors.Is(err, domain.ErrDuplicateAction):
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "already processed"})
	case errors.Is(err, domain.ErrAsteriskUnavailable):
		c.JSON(http.StatusBadGateway, gin.H{"detail": "asterisk unavailable"})
	case errors.Is(err, domain.ErrIdempotencyMismatch):
		c.JSON(http.StatusConflict, gin.H{"detail": "idempotency key reused"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
	}
}

// silence unused import in some build configs
var _ = strconv.Itoa
