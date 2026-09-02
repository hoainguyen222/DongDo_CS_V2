package ws

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/usecase"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 65536
)

// defaultUpgrader is used by dual-mode WS handlers with proper origin checks.
var defaultUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return false // Deny by default; handlers set per-endpoint whitelist
	},
}

// WSCheckOrigin returns a CheckOrigin func that only allows configured origins.
func WSCheckOrigin(allowedOrigins []string) func(r *http.Request) bool {
	return func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true // non-browser clients (curl, wscat, mobile)
		}
		for _, allowed := range allowedOrigins {
			if allowed == origin {
				return true
			}
		}
		log.Printf("⚠️ WS origin rejected: %s (allowed: %v)", origin, allowedOrigins)
		return false
	}
}

// StaffUpgrader creates an Upgrader for staff WS with origin whitelist.
func StaffUpgrader(allowedOrigins []string) websocket.Upgrader {
	u := defaultUpgrader
	u.CheckOrigin = WSCheckOrigin(allowedOrigins)
	return u
}

// CustomerUpgrader creates an Upgrader for customer WS with origin whitelist.
func CustomerUpgrader(allowedOrigins []string) websocket.Upgrader {
	u := defaultUpgrader
	u.CheckOrigin = WSCheckOrigin(allowedOrigins)
	return u
}

// Client is a middleman between the websocket connection and the hub.
type Client struct {
	hub       *Hub
	conn      *websocket.Conn
	send      chan *domain.WSEvent
	sessionID string
	userID    string
	userRole  string
	isStaff   bool // true = staff (JWT), false = customer (session)

	// Dependencies
	chatUC    *usecase.ChatUseCase
	voiceUC   *usecase.VoiceUseCase
	stateMgr  domain.StateManager
	eventBus  domain.EventBus
	authUC    *usecase.AuthUseCase   // nil for customers
	sessionUC *usecase.SessionUseCase // nil for staff
}

// newClient creates a Client with all dependencies wired.
func newClient(
	hub *Hub, conn *websocket.Conn, sessionID, userID, userRole string,
	isStaff bool,
	chatUC *usecase.ChatUseCase, voiceUC *usecase.VoiceUseCase,
	stateMgr domain.StateManager, eventBus domain.EventBus,
	authUC *usecase.AuthUseCase, sessionUC *usecase.SessionUseCase,
) *Client {
	return &Client{
		hub:       hub,
		conn:      conn,
		send:      make(chan *domain.WSEvent, 256),
		sessionID: sessionID,
		userID:    userID,
		userRole:  userRole,
		isStaff:   isStaff,
		chatUC:    chatUC,
		voiceUC:   voiceUC,
		stateMgr:  stateMgr,
		eventBus:  eventBus,
		authUC:    authUC,
		sessionUC: sessionUC,
	}
}

// ReadPump pumps messages from the websocket connection to the hub.
func (c *Client) ReadPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	ctx := context.Background()

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WS read error: %v", err)
			}
			break
		}

		var incoming struct {
			Type        domain.WSEventType `json:"type"`
			SessionID   string            `json:"session_id,omitempty"`
			Content     string            `json:"content"`
			ClientMsgID *string           `json:"client_msg_id,omitempty"`
			Payload     interface{}       `json:"payload,omitempty"`
		}

		if err := json.Unmarshal(message, &incoming); err != nil {
			log.Printf("Invalid WS JSON: %v", err)
			continue
		}

		var clientMsgUUID *uuid.UUID
		if incoming.ClientMsgID != nil && *incoming.ClientMsgID != "" {
			if u, err := uuid.Parse(*incoming.ClientMsgID); err == nil {
				clientMsgUUID = &u
			} else {
				u2 := uuid.NewSHA1(uuid.NameSpaceOID, []byte(*incoming.ClientMsgID))
				clientMsgUUID = &u2
			}
		}

		switch incoming.Type {
		case domain.WSEventMessage:
			if c.userRole == "guest" || c.userRole == "customer" {
				_, _ = c.chatUC.SendGuestMessage(ctx, c.sessionID, c.userID, incoming.Content, clientMsgUUID)
			} else {
				_, _ = c.chatUC.SendCSReply(ctx, c.sessionID, c.userID, c.userID, incoming.Content)
			}

		case domain.WSEventTyping:
			_ = c.stateMgr.SetTyping(ctx, c.sessionID, c.userID)
			c.hub.BroadcastToSession(c.sessionID, &domain.WSEvent{
				Type:      domain.WSEventTyping,
				SessionID: c.sessionID,
				SenderID:  c.userID,
				Timestamp: time.Now(),
			})

		case domain.WSEventCallOffer:
			targetSession := c.sessionID
			if incoming.SessionID != "" {
				targetSession = incoming.SessionID
			}
			callerType := domain.CallerGuest
			callerID := c.userID
			calleeType := domain.CallerCSKH
			calleeID := "CSKH"
			if c.userRole == "cskh" || c.userRole == "admin" {
				callerType = domain.CallerCSKH
				callerID = c.userID
				calleeType = domain.CallerGuest
				calleeID = targetSession
			}
			if c.voiceUC != nil {
				call, err := c.voiceUC.InitiateCall(ctx, targetSession, callerType, callerID, calleeType, calleeID)
				if err != nil {
					log.Printf("⚠️ Voice call initiate error: %v", err)
				} else if call != nil {
					log.Printf("📞 Voice call initiated: ID=%d, Session=%s", call.ID, targetSession)
				}
			}
			c.hub.BroadcastToSessionExcept(targetSession, &domain.WSEvent{
				Type:      incoming.Type,
				SessionID: targetSession,
				Payload:   incoming.Payload,
				SenderID:  c.userID,
				Timestamp: time.Now(),
			}, c.userID)
			if callerType == domain.CallerGuest {
				_ = c.eventBus.PublishWS(ctx, "admin_inbox", domain.WSEventCallRing, map[string]interface{}{
					"session_id":  targetSession,
					"caller_id":   callerID,
					"caller_type": callerType,
					"offer":       incoming.Payload,
				}, c.userID)
			}

		case domain.WSEventCallEnd:
			targetSession := c.sessionID
			if incoming.SessionID != "" {
				targetSession = incoming.SessionID
			}
			if c.voiceUC != nil {
				calls, _ := c.voiceUC.GetCallsBySession(ctx, targetSession)
				if len(calls) > 0 {
					lastCall := calls[0]
					if lastCall.Status != domain.CallEnded {
						dur := int(time.Since(lastCall.CreatedAt).Seconds())
						if dur < 1 {
							dur = 1
						}
						_ = c.voiceUC.EndCall(ctx, lastCall.ID, targetSession, dur, "")
					}
				}
			}
			c.hub.BroadcastToSessionExcept(targetSession, &domain.WSEvent{
				Type:      incoming.Type,
				SessionID: targetSession,
				Payload:   incoming.Payload,
				SenderID:  c.userID,
				Timestamp: time.Now(),
			}, c.userID)
			_ = c.eventBus.PublishWS(ctx, "admin_inbox", domain.WSEventCallEnd, map[string]interface{}{
				"session_id": targetSession,
			}, c.userID)

		case domain.WSEventCallAnswer, domain.WSEventCallICE:
			targetSession := c.sessionID
			if incoming.SessionID != "" {
				targetSession = incoming.SessionID
			}
			c.hub.BroadcastToSessionExcept(targetSession, &domain.WSEvent{
				Type:      incoming.Type,
				SessionID: targetSession,
				Payload:   incoming.Payload,
				SenderID:  c.userID,
				Timestamp: time.Now(),
			}, c.userID)
		}
	}
}

// WritePump pumps messages from the hub to the websocket connection.
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case event, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}

			eventBytes, _ := json.Marshal(event)
			_, _ = w.Write(eventBytes)

			// Add queued events to the current websocket message
			n := len(c.send)
			for i := 0; i < n; i++ {
				_, _ = w.Write([]byte{'\n'})
				nextBytes, _ := json.Marshal(<-c.send)
				_, _ = w.Write(nextBytes)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// ─── Dual-mode WebSocket handlers ────────────────────────────────────────────

// ServeStaffWS handles WebSocket connections for staff (admin/cskh) receiving messages.
// Auth: JWT (cookie `access_token` or query `?token=`).
// Staff always connects to the `admin_inbox` session to receive all guest messages.
// Allowed roles: `admin`, `cskh`.
//
//	GET /ws/staff
func ServeStaffWS(
	hub *Hub,
	chatUC *usecase.ChatUseCase,
	voiceUC *usecase.VoiceUseCase,
	stateMgr domain.StateManager,
	eventBus domain.EventBus,
	authUC *usecase.AuthUseCase,
	allowedOrigins []string,
	adminInboxSession string,
) gin.HandlerFunc {
	upgrader := StaffUpgrader(allowedOrigins)

	return func(c *gin.Context) {
		// 1. Extract JWT from cookie or query param
		var tokenString string
		if cookie, err := c.Cookie("access_token"); err == nil && cookie != "" {
			tokenString = cookie
		} else {
			tokenString = c.Query("token")
		}
		if tokenString == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "missing JWT token",
				"code":  "WS_TOKEN_REQUIRED",
			})
			return
		}

		// 2. Verify JWT
		user, err := authUC.VerifyStaffToken(c.Request.Context(), tokenString)
		if err != nil || user == nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "invalid or expired JWT",
				"code":  "WS_TOKEN_INVALID",
			})
			return
		}

		// 3. Role must be admin or cskh
		role := strings.ToLower(string(user.Role))
		if role != "admin" && role != "cskh" {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "staff role required (admin or cskh)",
				"code":  "WS_ROLE_FORBIDDEN",
			})
			return
		}

		// 4. Upgrade to WebSocket
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Printf("WS staff upgrade error: %v", err)
			return
		}

		// 5. Register client — always on admin_inbox
		client := newClient(
			hub, conn, adminInboxSession,
			user.Username, string(user.Role), true,
			chatUC, voiceUC, stateMgr, eventBus,
			authUC, nil, // sessionUC nil for staff
		)

		hub.register <- client
		go client.WritePump()
		go client.ReadPump()

		log.Printf("🔌 Staff WS connected: %s (role=%s, session=%s)", user.Username, string(user.Role), adminInboxSession)
	}
}

// ServeCustomerWS handles WebSocket connections for customers (guests).
// Auth: session_id from query `?session=` validated against chat_sessions DB.
// Customer connects to their own session to receive AI replies and staff messages.
//
//	GET /ws/customer
func ServeCustomerWS(
	hub *Hub,
	chatUC *usecase.ChatUseCase,
	voiceUC *usecase.VoiceUseCase,
	stateMgr domain.StateManager,
	eventBus domain.EventBus,
	sessionUC *usecase.SessionUseCase,
	allowedOrigins []string,
	adminInboxSession string,
) gin.HandlerFunc {
	upgrader := CustomerUpgrader(allowedOrigins)

	return func(c *gin.Context) {
		// 1. Get session_id from query param (NOT from cookie — must be explicit)
		sessionID := c.Query("session")
		if sessionID == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "missing session parameter (?session=...)",
				"code":  "WS_SESSION_REQUIRED",
			})
			return
		}

		// 2. Customer cannot access admin_inbox
		if sessionID == adminInboxSession {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "access denied",
				"code":  "WS_ADMIN_ACCESS_DENIED",
			})
			return
		}

		// 3. Validate session in DB (exists, active, not expired)
		session, err := sessionUC.ValidateSession(c.Request.Context(), sessionID)
		if err != nil {
			if errors.Is(err, usecase.ErrSessionExpired) {
				c.JSON(http.StatusUnauthorized, gin.H{
					"error": "session expired",
					"code":  "WS_SESSION_EXPIRED",
				})
				return
			}
			if errors.Is(err, usecase.ErrSessionNotFound) || errors.Is(err, usecase.ErrSessionInactive) {
				c.JSON(http.StatusUnauthorized, gin.H{
					"error": "invalid session",
					"code":  "WS_SESSION_INVALID",
				})
				return
			}
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "session validation failed",
				"code":  "WS_SESSION_ERROR",
			})
			return
		}

		// 4. Upgrade to WebSocket
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Printf("WS customer upgrade error: %v", err)
			return
		}

		// 5. Register client — on their own session
		displayName := session.DisplayName
		if displayName == "" {
			displayName = "Khách"
		}

		client := newClient(
			hub, conn, sessionID,
			sessionID, "guest", false, // userID=sessionID, role=guest, isStaff=false
			chatUC, voiceUC, stateMgr, eventBus,
			nil, sessionUC, // authUC nil for customer
		)
		_ = displayName // used in display but stored in session if needed

		hub.register <- client
		go client.WritePump()
		go client.ReadPump()

		log.Printf("🔌 Customer WS connected: session=%s (guest)", sessionID)
	}
}

// ─── Legacy ServeWS — DEPRECATED ─────────────────────────────────────────────
// ServeWS is kept for backward compatibility with existing integrations.
// New code should use ServeStaffWS and ServeCustomerWS.
// This handler trusts role and session_id from query params — a security risk.
func ServeWS(
	hub *Hub,
	chatUC *usecase.ChatUseCase,
	voiceUC *usecase.VoiceUseCase,
	stateMgr domain.StateManager,
	eventBus domain.EventBus,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionID := c.Query("session_id")
		if sessionID == "" {
			sessionID = "guest-" + uuid.New().String()
		}

		userID := c.Query("user_id")
		if userID == "" {
			userID = "Khách hàng"
		}

		userRole := c.Query("role")
		if userRole == "" {
			userRole = "guest"
		}

		conn, err := defaultUpgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Printf("WS upgrade error: %v", err)
			return
		}

		client := &Client{
			hub:       hub,
			conn:      conn,
			send:      make(chan *domain.WSEvent, 256),
			sessionID: sessionID,
			userID:    userID,
			userRole:  userRole,
			isStaff:   false,
			chatUC:    chatUC,
			voiceUC:   voiceUC,
			stateMgr:  stateMgr,
			eventBus:  eventBus,
		}

		hub.register <- client
		go client.WritePump()
		go client.ReadPump()
	}
}
