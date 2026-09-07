package asterisk

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

type ARIEvent struct {
	Type      string `json:"type"`
	Application string `json:"application"`
	Timestamp string `json:"timestamp"`
	Channel   *struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		State string `json:"state"`
	} `json:"channel,omitempty"`
	Bridge *struct {
		ID string `json:"id"`
	} `json:"bridge,omitempty"`
}

type ARIEventConsumer struct {
	baseURL     string
	username    string
	password    string
	appName     string
	eventHandler func(event *ARIEvent)
}

func NewARIEventConsumer(baseURL, username, password, appName string, eventHandler func(event *ARIEvent)) *ARIEventConsumer {
	return &ARIEventConsumer{
		baseURL:      baseURL,
		username:     username,
		password:     password,
		appName:      appName,
		eventHandler: eventHandler,
	}
}

func (c *ARIEventConsumer) Start(ctx context.Context) {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		log.Error().Err(err).Msg("Invalid Asterisk base URL for WebSocket")
		return
	}

	wsScheme := "ws"
	if u.Scheme == "https" {
		wsScheme = "wss"
	}
	wsURL := fmt.Sprintf("%s://%s/ari/events?api_key=%s:%s&app=%s",
		wsScheme, u.Host, c.username, c.password, c.appName)

	backoff := 1 * time.Second

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("Stopping ARI Event Consumer")
			return
		default:
			log.Info().Str("url", wsURL).Msg("Connecting to Asterisk ARI WebSocket events...")
			conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, nil)
			if err != nil {
				log.Warn().Err(err).Dur("backoff", backoff).Msg("Failed to connect Asterisk ARI WebSocket, retrying...")
				time.Sleep(backoff)
				if backoff < 30*time.Second {
					backoff *= 2
				}
				continue
			}

			log.Info().Msg("Connected to Asterisk ARI WebSocket successfully")
			backoff = 1 * time.Second

			for {
				_, message, err := conn.ReadMessage()
				if err != nil {
					log.Warn().Err(err).Msg("ARI WebSocket connection closed, reconnecting...")
					_ = conn.Close()
					break
				}

				var event ARIEvent
				if err := json.Unmarshal(message, &event); err == nil {
					if c.eventHandler != nil {
						c.eventHandler(&event)
					}
				}
			}
		}
	}
}
