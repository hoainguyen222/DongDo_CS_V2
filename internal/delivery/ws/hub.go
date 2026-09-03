package ws

import (
	"log"
	"sync"

	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
)

// Hub maintains the set of active WebSocket clients and broadcasts messages to sessions.
type Hub struct {
	// Registered clients mapped by sessionID -> set of clients
	sessions map[string]map[*Client]bool

	// Inbound messages from clients
	broadcast chan *domain.WSEvent

	// Register requests from clients
	register chan *Client

	// Unregister requests from clients
	unregister chan *Client

	mu sync.RWMutex
}

func NewHub() *Hub {
	return &Hub{
		sessions:   make(map[string]map[*Client]bool),
		broadcast:  make(chan *domain.WSEvent, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			if _, ok := h.sessions[client.sessionID]; !ok {
				h.sessions[client.sessionID] = make(map[*Client]bool)
			}
			h.sessions[client.sessionID][client] = true
			h.mu.Unlock()
			log.Printf("🔌 WS Client connected: %s (session: %s)", client.userID, client.sessionID)

		case client := <-h.unregister:
			h.mu.Lock()
			if clients, ok := h.sessions[client.sessionID]; ok {
				if _, ok := clients[client]; ok {
					delete(clients, client)
					// Đóng channel an toàn — dùng recover để chống "close of closed channel"
					// trong trường hợp hub đã close ở drop path trước đó (defensive).
					func() {
						defer func() {
							if r := recover(); r != nil {
								log.Printf("⚠️ WS hub: recover on close client.send for session=%s user=%s",
									client.sessionID, client.userID)
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
			log.Printf("🔌 WS Client disconnected: %s (session: %s)", client.userID, client.sessionID)

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

	// 1. Direct session clients
	if clients, ok := h.sessions[sessionID]; ok {
		for client := range clients {
			if excludeUserID != "" && client.userID == excludeUserID {
				continue
			}
			select {
			case client.send <- event:
			default:
				log.Printf("⚠️ WS hub: client.send buffer full, dropping event type=%s for session=%s user=%s",
					event.Type, sessionID, client.userID)
			}
		}
	}

	// 2. Broadcast to admin_inbox cho message/case_update/typing/call events
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
				for client := range adminClients {
					if excludeUserID != "" && client.userID == excludeUserID {
						continue
					}
					select {
					case client.send <- event:
					default:
						log.Printf("⚠️ WS hub: admin_inbox buffer full, dropping event type=%s for user=%s",
							event.Type, client.userID)
					}
				}
			}
		}
	}
}
