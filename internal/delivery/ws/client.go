package ws

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
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
			SessionID   string             `json:"session_id,omitempty"`
			Content     string             `json:"content"`
			ClientMsgID *string            `json:"client_msg_id,omitempty"`
			Payload     interface{}        `json:"payload,omitempty"`
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
			// Also notify admin_inbox if a guest is calling
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

// ServeWS handles websocket requests from the peer.
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

		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
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
			chatUC:    chatUC,
			voiceUC:   voiceUC,
			stateMgr:  stateMgr,
			eventBus:  eventBus,
		}

		client.hub.register <- client

		go client.WritePump()
		go client.ReadPump()
	}
}
