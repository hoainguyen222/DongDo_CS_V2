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

	// `register` is unbuffered so a server with Hub.Run() not started will
	// fail fast instead of silently dropping clients. `unregister` is
	// buffered so a client whose conn dies while the Hub loop is busy with
	// a slow broadcast still gets cleaned up — otherwise we'd leak the
	// goroutine holding `client.send` indefinitely.
	register   chan *Client
	unregister chan *Client

	mu sync.RWMutex

	logger zerolog.Logger

	// disconnectHook is invoked once when the last client of a session leaves
	// the hub. It receives the sessionID and the disconnected client's userID
	// and role (best-effort: only the leaving client is known; for a multi-
	// client session this is the most-recent client to disconnect).
	//
	// The hook is intentionally synchronous so that consumers can rely on
	// it having finished by the time the unregister returns. Consumers MUST
	// NOT block for long; spawn a goroutine if heavy work is needed.
	disconnectHook func(sessionID, userID, role string)
}

// DisconnectHandler is implemented by anything that wants to react to a
// session becoming empty (e.g. the call subsystem wants to cancel calls
// whose owner just disconnected).
type DisconnectHandler interface {
	OnDisconnect(sessionID, userID, role string)
}

// SetDisconnectHandler registers a callback invoked once when the last
// client of a session unregisters. Pass nil to clear.
func (h *Hub) SetDisconnectHandler(fn func(sessionID, userID, role string)) {
	h.mu.Lock()
	h.disconnectHook = fn
	h.mu.Unlock()
}

func NewHub() *Hub {
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	logger = logger.With().Str("component", "ws_hub").Logger()

	logger.Info().Msg("WebSocket Hub initialized")

	return &Hub{
		sessions: make(map[string]map[*Client]bool),
		// `unregister` is buffered so a client whose conn dies while the Hub loop is
		// busy with a slow broadcast still gets cleaned up — otherwise we'd leak
		// the goroutine holding `client.send` indefinitely. `register` stays
		// unbuffered so a server with Hub.Run() not started will fail fast
		// instead of silently dropping clients.
		register:   make(chan *Client),
		unregister: make(chan *Client, 64),
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

		case client := <-h.unregister:
			h.mu.Lock()
			hook := h.disconnectHook
			lastClient := false
			leavingSession := client.sessionID
			leavingUser := client.userID
			leavingRole := client.userRole
			if clients, ok := h.sessions[client.sessionID]; ok {
				if _, ok := clients[client]; ok {
					delete(clients, client)

					closeClientSend(client, h.logger)

					if len(clients) == 0 {
						delete(h.sessions, client.sessionID)
						lastClient = true
					}
				}
			}
			h.mu.Unlock()

			// Fire the hook outside the lock so a slow handler cannot
			// block the hub loop. Skip admin_inbox because admin panels
			// coming and going must not affect call state.
			if lastClient && hook != nil && leavingSession != "" && leavingSession != "admin_inbox" && leavingRole == "guest" {
				h.fireDisconnectHook(hook, leavingSession, leavingUser, leavingRole)
			}
		}
	}
}

// BroadcastToSession sends an event to all clients connected to a given session ID or channel.
func (h *Hub) BroadcastToSession(sessionID string, event *domain.WSEvent) {
	h.BroadcastToSessionExcept(sessionID, event, "")
}

// BroadcastToChannel is an alias kept for symmetry with the
// call.UseCase.WebsocketBroadcaster contract. Implementation is identical
// because sessions and channels share the same keyspace in the hub.
func (h *Hub) BroadcastToChannel(channel string, event *domain.WSEvent) {
	h.BroadcastToSession(channel, event)
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
			// Legacy call_ring: sent by client.go via eventBus.PublishWS("admin_inbox", ...).
			// Add to whitelist so the hub mirrors it to admin_inbox in all configurations
			// (direct hub broadcast + eventBus path both end up here).
			event.Type == domain.WSEventCallRing ||
			// Call v2 lifecycle events. Without these the admin's WebRTC
			// hook (useAdminWebRTC) never sees an "incoming_call" so the
			// ringing banner never appears even though the backend did its
			// job (CallUseCase.AssignAgentForCall → publishAgentEvent).
			event.Type == domain.WSEventCallWaiting ||
			event.Type == domain.WSEventIncomingCall ||
			event.Type == domain.WSEventCallConnecting ||
			event.Type == domain.WSEventCallRingingV2 ||
			event.Type == domain.WSEventCallStarted ||
			event.Type == domain.WSEventCallEndedV2 ||
			event.Type == domain.WSEventCallFailed ||
			event.Type == domain.WSEventAgentStatusChange ||
			event.Type == domain.WSEventQueuePosition

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

// closeClientSend closes a client's outbound channel safely. The hub may
// process two `unregister` events for the same client in a race (e.g. the
// client times out twice); the second close panics on a closed channel
// which we recover and log here so the hub loop never dies because of
// double unregistration.
func closeClientSend(c *Client, logger zerolog.Logger) {
	defer func() {
		if r := recover(); r != nil {
			logger.Warn().
				Interface("recover", r).
				Str("session_id", c.sessionID).
				Msg("Recovered from close of closed client channel")
		}
	}()
	close(c.send)
}

// fireDisconnectHook invokes the registered disconnect hook with panic
// isolation so a misbehaving callback cannot take down the hub loop.
func (h *Hub) fireDisconnectHook(fn func(string, string, string), sessionID, userID, role string) {
	defer func() {
		if r := recover(); r != nil {
			h.logger.Warn().Interface("recover", r).Msg("disconnect hook panicked")
		}
	}()
	fn(sessionID, userID, role)
}
