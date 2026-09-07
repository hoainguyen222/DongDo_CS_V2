package asterisk

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
	"github.com/rs/zerolog"
)

// EventHandler is invoked for every parsed ARI event.
type EventHandler func(ctx context.Context, ev domain.ARIEvent)

// WSConsumer is the long-running ARI WebSocket consumer.
//
// One consumer per Go process. Reconnects with exponential backoff on
// disconnect. Stops cleanly on ctx.Done().
type WSConsumer struct {
	cfg    Config
	logger zerolog.Logger
	handle EventHandler
}

// NewWSConsumer wires the consumer; Start() begins the read loop.
func NewWSConsumer(cfg Config, handle EventHandler, logger zerolog.Logger) *WSConsumer {
	return &WSConsumer{
		cfg:    cfg,
		logger: logger.With().Str("component", "ari_ws_consumer").Logger(),
		handle: handle,
	}
}

// Start runs until ctx is cancelled or a fatal error occurs.
func (c *WSConsumer) Start(ctx context.Context) error {
	backoff := c.cfg.WSReconnectBackoff
	for {
		if err := c.connectOnce(ctx); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			c.logger.Warn().Err(err).Dur("backoff", backoff).Msg("ARI WS disconnected, will retry")
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
			if backoff < 30*time.Second {
				backoff *= 2
			}
			continue
		}
		// Clean exit (context cancelled)
		return nil
	}
}

func (c *WSConsumer) connectOnce(ctx context.Context) error {
	wsURL := wsBaseURL(c.cfg.BaseURL) + "/events?app=" + c.cfg.AppName + "&subscribeAll=true"

	dialer := *websocket.DefaultDialer
	dialer.HandshakeTimeout = 5 * time.Second

	conn, resp, err := dialer.DialContext(ctx, wsURL, http.Header{
		"Authorization": {basicAuth(c.cfg.Username, c.cfg.Password)},
	})
	if err != nil {
		return fmt.Errorf("ari ws dial: %w", err)
	}
	if resp != nil {
		_ = resp.Body.Close()
	}

	c.logger.Info().Str("url", wsURL).Msg("ARI WS connected")

	// Pinger.
	pingDone := make(chan struct{})
	go func() {
		defer close(pingDone)
		ticker := time.NewTicker(20 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_ = conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second))
			}
		}
	}()

	// Read loop. We must respect ctx cancellation: gorilla websocket does
	// not natively, so we close the conn from the pinger goroutine when
	// ctx is done — that unblocks ReadMessage with an error we then treat
	// as a clean shutdown.
	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			_ = conn.Close()
			<-pingDone
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("ari ws read: %w", err)
		}

		var ev domain.ARIEvent
		if err := json.Unmarshal(raw, &ev); err != nil {
			c.logger.Warn().Err(err).Bytes("raw", raw).Msg("ari: skip malformed event")
			continue
		}
		if ev.Timestamp.IsZero() {
			ev.Timestamp = time.Now().UTC()
		}

		// Use a short ctx derived from the parent so a stuck handler can't
		// block the read loop indefinitely. The handler must do its own
		// work and return; the read loop is the heartbeat of the consumer.
		hctx, hcancel := context.WithTimeout(ctx, 5*time.Second)
		c.handle(hctx, ev)
		hcancel()
	}
}

// wsBaseURL converts http://host:port/ari → ws://host:port/ari (or https→wss).
func wsBaseURL(base string) string {
	if len(base) >= 7 && base[:7] == "http://" {
		return "ws://" + base[7:]
	}
	if len(base) >= 8 && base[:8] == "https://" {
		return "wss://" + base[8:]
	}
	return base
}

func basicAuth(u, p string) string {
	const basicPrefix = "Basic "
	return basicPrefix + base64.StdEncoding.EncodeToString([]byte(u+":"+p))
}

var _ = http.MethodGet
