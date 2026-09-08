package ws

import (
	"os"
	"sync"

	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/observability"
	"github.com/rs/zerolog"
)

// Hub maintains the set of active WebSocket clients and broadcasts messages to sessions.
type Hub struct {
	sessions map[string]map[*Client]bool

	broadcast chan *domain.WSEvent

	register chan *Client

	unregister chan *Client

	mu sync.RWMutex

	logger zerolog.Logger
}

func NewHub() *Hub {
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	logger = logger.With().Str("component", "ws_hub").Logger()

	logger.Info().Msg("WebSocket Hub initialized")

	return &Hub{
		sessions:   make(map[string]map[*Client]bool),
		broadcast:  make(chan *domain.WSEvent, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		logger:     logger,
	}
}

func (h *Hub) Run() {
	h.logger.Info().Msg("WebSocket Hub started")
	defer h.logger.Info().Msg("WebSocket Hub stopped")

	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			if _, ok := h.sessions[client.sessionID]; !ok {
				h.sessions[client.sessionID] = make(map[*Client]bool)
			}
			h.sessions[client.sessionID][client] = true
			h.mu.Unlock()

			// Counter increments are goroutine-safe (atomic) so calling
			// them outside the lock is fine.
			if ws := observability.WS(); ws != nil {
				ws.OnConnect(client.userRole)
			}

		case client := <-h.unregister:
			h.mu.Lock()
			if clients, ok := h.sessions[client.sessionID]; ok {
				if _, ok := clients[client]; ok {
					delete(clients, client)

					func() {
						defer func() {
							if r := recover(); r != nil {
								h.logger.Warn().Interface("recover", r).Msg("Recovered from close of closed client channel")
							}
						}()
						close(client.send)
					}()

					if len(clients) == 0 {
						delete(h.sessions, client.sessionID)
					}
				}
			}
			h.mu.Unlock()

			if ws := observability.WS(); ws != nil {
				ws.OnDisconnect(client.userRole)
			}

		case event := <-h.broadcast:
			h.BroadcastToSession(event.SessionID, event)
		}
	}
}

// OnlineStaffCount returns the number of currently connected WebSocket
// clients with a staff-equivalent role (cskh/admin/leader/owner). Excludes
// guests and unknown roles so the gauge does not include customers.
//
// The implementation walks the sessions map under RLock; the cost is
// proportional to the number of currently connected clients, which is
// bounded and small (thousands at worst).
func (h *Hub) OnlineStaffCount() int {
	if h == nil {
		return 0
	}
	h.mu.RLock()
	defer h.mu.RUnlock()

	seen := make(map[string]struct{})
	for _, clients := range h.sessions {
		for client := range clients {
			r := client.userRole
			if r == "cskh" || r == "admin" || r == "leader" || r == "owner" {
				seen[client.userID] = struct{}{}
			}
		}
	}
	return len(seen)
}

// BroadcastToSession sends an event to all clients connected to a given session ID or channel.
func (h *Hub) BroadcastToSession(sessionID string, event *domain.WSEvent) {
	h.BroadcastToSessionExcept(sessionID, event, "")
}

// BroadcastToSessionExcept sends an event to session clients while excluding the sender to avoid reflection.
//
// Lưu ý: trước đây hub drop theo kiểu `close(client.send) + delete(client)` khi channel đầy,
// làm client mất kết nối cho đến khi reconnect — khiến các event sau (typing/AI reply/call)
// bị mất hoàn toàn. Giờ chỉ drop event đó và log, giữ client online.
func (h *Hub) BroadcastToSessionExcept(sessionID string, event *domain.WSEvent, excludeUserID string) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if clients, ok := h.sessions[sessionID]; ok {
		for client := range clients {
			if excludeUserID != "" && client.userID == excludeUserID {
				continue
			}
			select {
			case client.send <- event:
				if ws := observability.WS(); ws != nil {
					ws.OnMessageSent(string(event.Type), sessionID)
				}
			default:
				h.logger.Warn().
					Str("event_type", string(event.Type)).
					Str("session_id", sessionID).
					Msg("WS hub: client buffer full, dropping event")
				if ws := observability.WS(); ws != nil {
					ws.OnBufferedDrop(sessionID)
				}
			}
		}
	}

	if sessionID != "admin_inbox" {
		isBroadcastToAdmin := event.Type == domain.WSEventMessage ||
			event.Type == domain.WSEventCaseUpdate ||
			event.Type == domain.WSEventTyping ||
			event.Type == domain.WSEventCallOffer ||
			event.Type == domain.WSEventCallAnswer ||
			event.Type == domain.WSEventCallICE ||
			event.Type == domain.WSEventCallEnd

		if isBroadcastToAdmin {
			if adminClients, ok := h.sessions["admin_inbox"]; ok {
				for adminClient := range adminClients {
					if excludeUserID != "" && adminClient.userID == excludeUserID {
						continue
					}
					select {
					case adminClient.send <- event:
						if ws := observability.WS(); ws != nil {
							ws.OnMessageSent(string(event.Type), "admin_inbox")
						}
					default:
						h.logger.Warn().
							Str("event_type", string(event.Type)).
							Str("target", "admin_inbox").
							Msg("WS hub: admin_inbox buffer full, dropping event")
						if ws := observability.WS(); ws != nil {
							ws.OnBufferedDrop("admin_inbox")
						}
					}
				}
			}
		}
	}
}
