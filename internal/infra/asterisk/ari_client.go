// ari_client.go — Asterisk REST Interface (ARI) client.
//
// Responsibilities:
//   1. Maintain a long-lived WebSocket connection to Asterisk's
//      `/ari/events` endpoint.  The events stream delivers StasisStart /
//      StasisEnd, channel state transitions, and bridge events.
//   2. Provide typed REST methods (answer, hangup, originate, bridge,
//      record, …) used by the higher-level service to drive calls.
//   3. Auto-reconnect with backoff and dispatch JSON events into Go
//      handlers.
//
// The client is deliberately small: ARI is just HTTP + WebSocket with
// JSON payloads, so we don't pull a heavy third-party library.
package asterisk

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
)

// ARIEvent is the generic envelope parsed off the ARI WebSocket.
type ARIEvent struct {
	Type      string          `json:"type"`
	Timestamp string          `json:"timestamp,omitempty"`
	// Application-specific fields follow; we keep them raw so the
	// dispatcher can decode into typed structs as needed.
	Raw json.RawMessage `json:"-"`
}

// EventHandler is the user-supplied callback invoked for each event of a
// registered type.  The event argument contains the already-decoded JSON
// payload (full message body minus the envelope).
type EventHandler func(e ARIEvent)

// ARIClient is a thread-safe Asterisk REST Interface client.
type ARIClient struct {
	cfg    ARIConfig
	logger zerolog.Logger
	http   *http.Client

	// WebSocket state.
	mu        sync.Mutex
	conn      *websocket.Conn
	connected atomic.Bool
	stopCh    chan struct{}
	stopOnce  sync.Once

	// Handlers keyed by event type (e.g. "StasisStart").  A "*" key
	// receives every event — useful for debugging.
	handlersMu sync.RWMutex
	handlers   map[string][]EventHandler
}

// NewARIClient builds a client from the supplied config.
func NewARIClient(cfg ARIConfig) (*ARIClient, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &ARIClient{
		cfg:    cfg,
		logger: newLogger().With().Str("component", "ari_client").Logger(),
		http:   &http.Client{Timeout: 30 * time.Second},
		stopCh: make(chan struct{}),
		handlers: map[string][]EventHandler{
			"*": {},
		},
	}, nil
}

// Validate returns an error if the config is missing required fields.
func (c *ARIConfig) Validate() error {
	if c.BaseURL == "" {
		return errors.New("ari: BaseURL is required")
	}
	if c.WSURL == "" {
		return errors.New("ari: WSURL is required")
	}
	if c.Username == "" || c.Password == "" {
		return errors.New("ari: username and password are required")
	}
	if c.AppName == "" {
		return errors.New("ari: AppName is required")
	}
	return nil
}

// On registers a handler for a specific event type (or "*" for all).
// Multiple handlers may be registered per type; they are invoked in
// registration order.
func (c *ARIClient) On(eventType string, h EventHandler) {
	c.handlersMu.Lock()
	defer c.handlersMu.Unlock()
	c.handlers[eventType] = append(c.handlers[eventType], h)
}

// Connect opens the ARI WebSocket, starts the read loop, and blocks
// until Stop is called or the stream is irrecoverably broken (in which
// case it auto-reconnects with exponential backoff).
func (c *ARIClient) Connect(ctx context.Context) error {
	go c.supervisor(ctx)
	return nil
}

// supervisor keeps the WebSocket connection alive.  Each iteration
// establishes the socket, runs the read loop until an error, and
// retries with exponential backoff (capped at 30s).
func (c *ARIClient) supervisor(ctx context.Context) {
	backoff := time.Second
	maxBackoff := 30 * time.Second
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		default:
		}

		if err := c.connectOnce(ctx); err != nil {
			c.logger.Warn().Err(err).
				Dur("retry_in", backoff).
				Msg("ARI websocket closed; reconnecting")
			select {
			case <-ctx.Done():
				return
			case <-c.stopCh:
				return
			case <-time.After(backoff):
			}
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}
		backoff = time.Second
	}
}

// connectOnce dials the ARI WebSocket, subscribes to the configured app,
// and runs the read loop until it errors out.
func (c *ARIClient) connectOnce(ctx context.Context) error {
	u, err := url.Parse(c.cfg.WSURL)
	if err != nil {
		return fmt.Errorf("ari: parse ws url: %w", err)
	}
	q := u.Query()
	q.Set("app", c.cfg.AppName)
	q.Set("subscribeAll", "true") // also receive events for other apps' channels
	u.RawQuery = q.Encode()

	headers := http.Header{}
	headers.Set("Authorization", basicAuth(c.cfg.Username, c.cfg.Password))

	dialer := &websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}
	conn, _, err := dialer.DialContext(ctx, u.String(), headers)
	if err != nil {
		return fmt.Errorf("ari: dial: %w", err)
	}
	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()
	c.connected.Store(true)
	c.logger.Info().Str("app", c.cfg.AppName).Str("url", u.String()).Msg("ARI websocket connected")

	defer func() {
		c.connected.Store(false)
		c.mu.Lock()
		if c.conn != nil {
			_ = c.conn.Close()
			c.conn = nil
		}
		c.mu.Unlock()
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-c.stopCh:
			return nil
		default:
		}

		_, data, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("ari: read: %w", err)
		}
		c.dispatch(data)
	}
}

// dispatch decodes a raw ARI event and routes it to matching handlers.
func (c *ARIClient) dispatch(raw []byte) {
	var env struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		c.logger.Warn().Err(err).Bytes("raw", raw).Msg("ari: invalid event json")
		return
	}
	ev := ARIEvent{Type: env.Type, Raw: raw}

	c.handlersMu.RLock()
	defer c.handlersMu.RUnlock()
	for _, h := range c.handlers[env.Type] {
		safeCall(h, ev)
	}
	for _, h := range c.handlers["*"] {
		safeCall(h, ev)
	}
}

func safeCall(h EventHandler, ev ARIEvent) {
	defer func() {
		// swallow handler panics so the read loop survives.
		if r := recover(); r != nil {
			// no logger here; use stderr so we still see something
			fmt.Printf("ari: handler panic on %s: %v\n", ev.Type, r)
		}
	}()
	h(ev)
}

// Stop terminates the WebSocket and prevents future reconnects.
func (c *ARIClient) Stop() {
	c.stopOnce.Do(func() {
		close(c.stopCh)
		c.mu.Lock()
		if c.conn != nil {
			_ = c.conn.Close()
			c.conn = nil
		}
		c.mu.Unlock()
	})
}

// IsConnected returns true if the WebSocket is currently up.
func (c *ARIClient) IsConnected() bool { return c.connected.Load() }

// basicAuth returns the Basic auth header value for the supplied creds.
func basicAuth(u, p string) string {
	const basic = "Basic "
	// We use stdlib base64-equivalent via http.Request.SetBasicAuth; here
	// we just encode manually for the header value.
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	auth := u + ":" + p
	enc := make([]byte, 0, len(auth)*4/3+4)
	src := []byte(auth)
	for i := 0; i < len(src); i += 3 {
		var b [3]byte
		n := copy(b[:], src[i:])
		enc = append(enc, alphabet[b[0]>>2])
		enc = append(enc, alphabet[((b[0]&0x03)<<4)|(b[1]>>4)])
		if n > 1 {
			enc = append(enc, alphabet[((b[1]&0x0f)<<2)|(b[2]>>6)])
		} else {
			enc = append(enc, '=')
		}
		if n > 2 {
			enc = append(enc, alphabet[b[2]&0x3f])
		} else {
			enc = append(enc, '=')
		}
	}
	return basic + string(enc)
}

// ============================================================================
// REST API helpers
// ============================================================================

// doRequest issues an HTTP request to the ARI REST API.  body may be nil
// for GET/DELETE.  The response is returned as the raw bytes; callers
// decode into typed structs.
func (c *ARIClient) doRequest(ctx context.Context, method, path string, query url.Values, body interface{}) ([]byte, error) {
	u, err := url.Parse(c.cfg.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("ari: invalid base url: %w", err)
	}
	u.Path = strings.TrimRight(u.Path, "/") + path
	if query != nil {
		u.RawQuery = query.Encode()
	}

	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("ari: marshal body: %w", err)
		}
		reader = bytes.NewReader(buf)
	}

	req, err := http.NewRequestWithContext(ctx, method, u.String(), reader)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.cfg.Username, c.cfg.Password)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ari: %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ari: read response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return respBody, fmt.Errorf("ari: %s %s → HTTP %d: %s", method, path, resp.StatusCode, string(respBody))
	}
	return respBody, nil
}

// Channel represents an ARI Channel object — the public JSON shape.
type Channel struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	State     string            `json:"state"`
	Caller    map[string]string `json:"caller"`
	Connected map[string]string `json:"connected"`
	Dialplan  map[string]string `json:"dialplan"`
	Channelvars map[string]string `json:"channelvars"`
}

// Bridge represents an ARI Bridge object.
type Bridge struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Channels []string `json:"channels"`
}

// Answer answers a ringing channel.  No-op if the channel is already
// answered (Asterisk returns a 409 which we ignore as success).
func (c *ARIClient) Answer(ctx context.Context, channelID string) error {
	_, err := c.doRequest(ctx, http.MethodPost, "/ari/channels/"+channelID+"/answer", nil, nil)
	if err != nil && !strings.Contains(err.Error(), "409") {
		return err
	}
	return nil
}

// Hangup terminates a channel.
func (c *ARIClient) Hangup(ctx context.Context, channelID string) error {
	_, err := c.doRequest(ctx, http.MethodDelete, "/ari/channels/"+channelID, nil, nil)
	return err
}

// Continue in the dialplan after Stasis — Asterisk will resume execution
// of the channel where Stasis() was called.
func (c *ARIClient) Continue(ctx context.Context, channelID string) error {
	_, err := c.doRequest(ctx, http.MethodPost, "/ari/channels/"+channelID+"/continue", nil, nil)
	return err
}

// CreateBridge creates a holding bridge (no mixing; we'll add channels
// explicitly).  `name` is optional.
func (c *ARIClient) CreateBridge(ctx context.Context, name string) (*Bridge, error) {
	q := url.Values{}
	if name != "" {
		q.Set("name", name)
		q.Set("type", "holding")
	} else {
		q.Set("type", "holding")
	}
	body, err := c.doRequest(ctx, http.MethodPost, "/ari/bridges", q, nil)
	if err != nil {
		return nil, err
	}
	var b Bridge
	if err := json.Unmarshal(body, &b); err != nil {
		return nil, err
	}
	return &b, nil
}

// AddChannelToBridge bridges a channel into an existing bridge.
func (c *ARIClient) AddChannelToBridge(ctx context.Context, bridgeID, channelID string) error {
	q := url.Values{}
	q.Set("channel", channelID)
	_, err := c.doRequest(ctx, http.MethodPost, "/ari/bridges/"+bridgeID+"/addChannel", q, nil)
	return err
}

// DestroyBridge tears down a bridge.  Asterisk hangs up all members.
func (c *ARIClient) DestroyBridge(ctx context.Context, bridgeID string) error {
	_, err := c.doRequest(ctx, http.MethodDelete, "/ari/bridges/"+bridgeID, nil, nil)
	return err
}

// OriginateParams configures a new outgoing channel.
type OriginateParams struct {
	Endpoint    string            // SIP endpoint, e.g. "PJSIP/1001" or "SIP/1001"
	App         string            // Stasis app name (default: our app)
	AppArgs     string            // Comma-separated args passed to StasisStart
	CallerID    string            // e.g. "\"Agent\" <1001>"
	Timeout     int               // seconds before giving up
	ChannelVars map[string]string // Set channel variables
	Variables   map[string]string // Set as channel vars (legacy)
}

// Originate creates a new outgoing channel.  When the channel answers
// (or fails), the ARI app will receive a StasisStart event (success) or
// a ChannelHangupRequest (failure).
func (c *ARIClient) Originate(ctx context.Context, p OriginateParams) (*Channel, error) {
	if p.App == "" {
		p.App = c.cfg.AppName
	}
	q := url.Values{}
	q.Set("endpoint", p.Endpoint)
	q.Set("app", p.App)
	if p.AppArgs != "" {
		q.Set("appArgs", p.AppArgs)
	}
	if p.CallerID != "" {
		q.Set("callerId", p.CallerID)
	}
	if p.Timeout > 0 {
		q.Set("timeout", fmt.Sprintf("%d", p.Timeout))
	}
	for k, v := range p.ChannelVars {
		q.Set("channel."+k, v)
	}
	for k, v := range p.Variables {
		q.Set("variable", k+"="+v) // ARI legacy form
	}
	body, err := c.doRequest(ctx, http.MethodPost, "/ari/channels", q, nil)
	if err != nil {
		return nil, err
	}
	var ch Channel
	if err := json.Unmarshal(body, &ch); err != nil {
		return nil, err
	}
	return &ch, nil
}

// SetChannelVar sets a channel variable on a live channel.
func (c *ARIClient) SetChannelVar(ctx context.Context, channelID, name, value string) error {
	q := url.Values{}
	q.Set("variable", name+"="+value)
	_, err := c.doRequest(ctx, http.MethodPost, "/ari/channels/"+channelID+"/variable", q, nil)
	return err
}

// ChannelState queries the current state of a channel.
func (c *ARIClient) ChannelState(ctx context.Context, channelID string) (*Channel, error) {
	body, err := c.doRequest(ctx, http.MethodGet, "/ari/channels/"+channelID, nil, nil)
	if err != nil {
		return nil, err
	}
	var ch Channel
	if err := json.Unmarshal(body, &ch); err != nil {
		return nil, err
	}
	return &ch, nil
}
