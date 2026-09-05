package ws

import (
	"os"
	"sync"

	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
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
			// Register the client under its primary sessionID plus any
			// extra channels (e.g. "admin_inbox" for staff clients). All
			// channels share the same `send` buffer so the browser sees
			// one ordered stream regardless of source.
			channels := append([]string{client.sessionID}, client.extraSessions...)
			for _, sid := range channels {
				if sid == "" {
					continue
				}
				if _, ok := h.sessions[sid]; !ok {
					h.sessions[sid] = make(map[*Client]bool)
				}
				h.sessions[sid][client] = true
			}
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			// Remove the client from every channel it was registered
			// under. Close the shared `send` channel exactly once to
			// signal WritePump to exit.
			channels := append([]string{client.sessionID}, client.extraSessions...)
			closed := false
			for _, sid := range channels {
				if sid == "" {
					continue
				}
				if clients, ok := h.sessions[sid]; ok {
					if _, present := clients[client]; present {
						delete(clients, client)
						if !closed {
							func() {
								defer func() {
									if r := recover(); r != nil {
										h.logger.Warn().Interface("recover", r).Msg("Recovered from close of closed client channel")
									}
								}()
								close(client.send)
							}()
							closed = true
						}
						if len(clients) == 0 {
							delete(h.sessions, sid)
						}
					}
				}
			}
			h.mu.Unlock()

		case event := <-h.broadcast:
			h.BroadcastToSession(event.SessionID, event)
		}
	}
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
			default:
				h.logger.Warn().
					Str("event_type", string(event.Type)).
					Str("session_id", sessionID).
					Msg("WS hub: client buffer full, dropping event")
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
			event.Type == domain.WSEventCallEnd ||
			event.Type == domain.WSEventCallRing ||
			event.Type == domain.WSEventCallStatusUpdate

		if isBroadcastToAdmin {
			if adminClients, ok := h.sessions["admin_inbox"]; ok {
				for adminClient := range adminClients {
					if excludeUserID != "" && adminClient.userID == excludeUserID {
						continue
					}
					select {
					case adminClient.send <- event:
					default:
						h.logger.Warn().
							Str("event_type", string(event.Type)).
							Str("target", "admin_inbox").
							Msg("WS hub: admin_inbox buffer full, dropping event")
					}
				}
			}
		}
	}
}