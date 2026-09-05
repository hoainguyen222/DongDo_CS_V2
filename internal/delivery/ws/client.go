package ws

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/usecase"
	"github.com/rs/zerolog"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 65536
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all CORS origins
	},
}

// Client is a middleman between the websocket connection and the hub.
type Client struct {
	hub       *Hub
	conn      *websocket.Conn
	send      chan *domain.WSEvent
	sessionID string
	userID    string
	userRole  string
	chatUC    *usecase.ChatUseCase
	voiceUC   *usecase.VoiceUseCase
	stateMgr  domain.StateManager
	eventBus  domain.EventBus
	logger    zerolog.Logger

	// extraSessions holds additional hub channels this client should
	// receive events from (e.g. "admin_inbox" for staff clients). All
	// channels share the same `send` buffer so the browser sees a
	// single ordered stream regardless of source.
	extraSessions []string
}

func NewClient(hub *Hub, conn *websocket.Conn, sessionID, userID, userRole string, chatUC *usecase.ChatUseCase, voiceUC *usecase.VoiceUseCase, stateMgr domain.StateManager, eventBus domain.EventBus) *Client {
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	logger = logger.With().
		Str("component", "ws_client").
		Str("session_id", sessionID).
		Str("user_id", userID).
		Logger()

	return &Client{
		hub:       hub,
		conn:      conn,
		send:      make(chan *domain.WSEvent, 256),
		sessionID: sessionID,
		userID:    userID,
		userRole:  userRole,
		chatUC:    chatUC,
		voiceUC:   voiceUC,
		stateMgr:  stateMgr,
		eventBus:  eventBus,
		logger:    logger,
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
				c.logger.Warn().Err(err).Msg("WS read error - unexpected close")
			}
			break
		}

		var incoming struct {
			Type        domain.WSEventType `json:"type"`
			SessionID   string             `json:"session_id,omitempty"`
			Content     string             `json:"content"`
			ClientMsgID *string            `json:"client_msg_id,omitempty"`
			Payload     interface{}        `json:"payload,omitempty"`
		}

		if err := json.Unmarshal(message, &incoming); err != nil {
			c.logger.Warn().Err(err).Msg("Invalid WS JSON message")
			continue
		}

		// Message sending has been migrated to REST API.
		// Only typing, call signaling, and ICE candidates are processed via WS.

		switch incoming.Type {
		case domain.WSEventTyping:
			if err := c.stateMgr.SetTyping(ctx, c.sessionID, c.userID); err != nil {
				c.logger.Warn().Err(err).Msg("Failed to set typing state")
			}
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
				if _, err := c.voiceUC.InitiateCall(ctx, targetSession, callerType, callerID, calleeType, calleeID); err != nil {
					c.logger.Error().Err(err).Str("session_id", targetSession).Msg("Voice call initiate error")
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
				if err := c.eventBus.PublishWS(ctx, "admin_inbox", domain.WSEventCallRing, map[string]interface{}{
					"session_id":  targetSession,
					"caller_id":   callerID,
					"caller_type": callerType,
					"offer":       incoming.Payload,
				}, c.userID); err != nil {
					c.logger.Warn().Err(err).Msg("Failed to publish call ring")
				}
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
						if err := c.voiceUC.EndCall(ctx, lastCall.ID, targetSession, dur, ""); err != nil {
							c.logger.Error().Err(err).Msg("Failed to end call")
						}
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
			if err := c.eventBus.PublishWS(ctx, "admin_inbox", domain.WSEventCallEnd, map[string]interface{}{
				"session_id": targetSession,
			}, c.userID); err != nil {
				c.logger.Warn().Err(err).Msg("Failed to publish call end")
			}

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
				c.logger.Error().Err(err).Msg("WS NextWriter error")
				return
			}

			eventBytes, _ := json.Marshal(event)
			_, _ = w.Write(eventBytes)

			n := len(c.send)
			for i := 0; i < n; i++ {
				_, _ = w.Write([]byte{'\n'})
				nextBytes, _ := json.Marshal(<-c.send)
				_, _ = w.Write(nextBytes)
			}

			if err := w.Close(); err != nil {
				c.logger.Error().Err(err).Msg("WS write batch close error")
				return
			}

		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				c.logger.Error().Err(err).Msg("WS Ping failed")
				return
			}
		}
	}
}

// ServeWS handles websocket requests from the peer.
func ServeWS(
	hub *Hub,
	chatUC *usecase.ChatUseCase,
	voiceUC *usecase.VoiceUseCase,
	stateMgr domain.StateManager,
	eventBus domain.EventBus,
) gin.HandlerFunc {
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	logger = logger.With().Str("component", "ws_handler").Logger()

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

		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			logger.Error().Err(err).Str("session_id", sessionID).Msg("WS upgrade error")
			return
		}

		clientLogger := zerolog.New(os.Stderr).With().Timestamp().Logger()
		clientLogger = clientLogger.With().
			Str("component", "ws_client").
			Str("session_id", sessionID).
			Str("user_id", userID).
			Logger()

		client := &Client{
			hub:       hub,
			conn:      conn,
			send:      make(chan *domain.WSEvent, 256),
			sessionID: sessionID,
			userID:    userID,
			userRole:  userRole,
			chatUC:    chatUC,
			voiceUC:   voiceUC,
			stateMgr:  stateMgr,
			eventBus:  eventBus,
			logger:    clientLogger,
		}

		// Admin and CSKH staff clients also listen on the "admin_inbox"
		// channel. This is critical for use cases like the /admin/login
		// page where staff haven't authenticated yet (so their WS
		// session_id is a random placeholder) but still need to see
		// incoming guest call rings + status transitions. The hub will
		// register this client under both channels, sharing one `send`
		// buffer so the browser sees a single ordered stream.
		if userRole == "admin" || userRole == "cskh" {
			client.extraSessions = append(client.extraSessions, "admin_inbox")
		}

		client.hub.register <- client

		go client.WritePump()
		go client.ReadPump()
	}
}

// truncateString truncates a string to the specified length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}