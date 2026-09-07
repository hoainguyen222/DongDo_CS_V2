package websocket

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type Client struct {
	ID     string
	Hub    *WSHub
	Conn   *websocket.Conn
	Send   chan []byte
	Topics map[string]bool
}

type WSHub struct {
	clients    map[*Client]bool
	topicMap   map[string]map[*Client]bool
	register   chan *Client
	unregister chan *Client
	broadcast  chan BroadcastMessage
	mu         sync.RWMutex
}

type BroadcastMessage struct {
	Topic   string      `json:"topic"`
	Event   string      `json:"event"`
	Payload interface{} `json:"payload"`
}

func NewWSHub() *WSHub {
	return &WSHub{
		clients:    make(map[*Client]bool),
		topicMap:   make(map[string]map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan BroadcastMessage, 256),
	}
}

func (h *WSHub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			log.Info().Str("client_id", client.ID).Msg("WebSocket client registered")

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.Send)
				for topic, set := range h.topicMap {
					delete(set, client)
					if len(set) == 0 {
						delete(h.topicMap, topic)
					}
				}
			}
			h.mu.Unlock()
			log.Info().Str("client_id", client.ID).Msg("WebSocket client unregistered")

		case msg := <-h.broadcast:
			bytes, err := json.Marshal(msg)
			if err != nil {
				continue
			}

			h.mu.RLock()
			targets := h.topicMap[msg.Topic]
			if msg.Topic == "" || msg.Topic == "*" {
				for client := range h.clients {
					select {
					case client.Send <- bytes:
					default:
					}
				}
			} else if targets != nil {
				for client := range targets {
					select {
					case client.Send <- bytes:
					default:
					}
				}
			}
			h.mu.RUnlock()
		}
	}
}

func (h *WSHub) Subscribe(client *Client, topic string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.topicMap[topic] == nil {
		h.topicMap[topic] = make(map[*Client]bool)
	}
	h.topicMap[topic][client] = true
	client.Topics[topic] = true
}

func (h *WSHub) Broadcast(topic, event string, payload interface{}) {
	h.broadcast <- BroadcastMessage{
		Topic:   topic,
		Event:   event,
		Payload: payload,
	}
}

func (h *WSHub) HandleWS(c *gin.Context) {
	clientID := c.Query("client_id")
	if clientID == "" {
		clientID = c.Query("agent_id")
	}
	if clientID == "" {
		clientID = "guest_" + c.ClientIP()
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Error().Err(err).Msg("Failed WebSocket upgrade")
		return
	}

	client := &Client{
		ID:     clientID,
		Hub:    h,
		Conn:   conn,
		Send:   make(chan []byte, 256),
		Topics: make(map[string]bool),
	}

	h.register <- client

	// Auto-subscribe to client_id topic and admin_inbox if agent
	h.Subscribe(client, clientID)
	h.Subscribe(client, "admin_inbox")
	h.Subscribe(client, "global_calls")

	go client.writePump()
	go client.readPump()
}

func (c *Client) readPump() {
	defer func() {
		c.Hub.unregister <- c
		c.Conn.Close()
	}()
	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			break
		}
		var req struct {
			Action string `json:"action"`
			Topic  string `json:"topic"`
		}
		if json.Unmarshal(message, &req) == nil && req.Action == "subscribe" && req.Topic != "" {
			c.Hub.Subscribe(c, req.Topic)
		}
	}
}

func (c *Client) writePump() {
	defer c.Conn.Close()
	for message := range c.Send {
		if err := c.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
			break
		}
	}
}
