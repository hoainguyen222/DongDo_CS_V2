// Package asterisk contains the ARI (Asterisk REST Interface) integration.
//
// Two implementations of domain.AsteriskGateway live here:
//
//   - Client            : real HTTP+WebSocket client against an Asterisk ARI server
//     (e.g. running in the bundled docker-compose).
//   - MockGateway       : in-memory implementation for unit/integration tests and
//     for environments where Asterisk is intentionally absent.
//
// Both implementations emit domain.ARIEvent values that the consumer worker
// (see internal/infrastructure/asterisk/ari_ws.go) hands back to the use case.
//
// All HTTP calls are context-aware and the WebSocket consumer respects ctx.Done()
// for graceful shutdown.
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
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
	"github.com/rs/zerolog"
)

// Config bundles ARI connection settings. Loaded from environment.
type Config struct {
	BaseURL             string        // e.g. http://asterisk:8088/ari
	Username            string        // ARI user
	Password            string        // ARI password
	AppName             string        // Stasis app name; must match dialplan
	RecordingEnabled    bool          // auto-start MixMonitor on answer
	HealthCheckInterval time.Duration // background health check cadence
	WSReconnectBackoff  time.Duration // re-connect backoff for ARI WS
	HTTPTimeout         time.Duration // per-call HTTP timeout
}

func LoadConfigFromEnv() Config {
	return Config{
		BaseURL:             getenv("ASTERISK_ARI_URL", "http://asterisk:8088/ari"),
		Username:            getenv("ASTERISK_ARI_USER", "callservice"),
		Password:            getenv("ASTERISK_ARI_PASSWORD", "callsecret"),
		AppName:             getenv("ASTERISK_ARI_APP", "callapp"),
		RecordingEnabled:    getenv("ASTERISK_RECORDING_ENABLED", "false") == "true",
		HealthCheckInterval: 10 * time.Second,
		WSReconnectBackoff:  2 * time.Second,
		HTTPTimeout:         5 * time.Second,
	}
}

func getenv(k, fb string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fb
}

// Client is the real ARI HTTP + WebSocket client.
type Client struct {
	cfg    Config
	http   *http.Client
	logger zerolog.Logger

	mu     sync.Mutex
	closed bool
}

// NewClient returns an ARI HTTP/WebSocket client. It does NOT connect eagerly;
// ARI operations are performed lazily per-call.
func NewClient(cfg Config, logger zerolog.Logger) *Client {
	if cfg.HTTPTimeout <= 0 {
		cfg.HTTPTimeout = 5 * time.Second
	}
	if cfg.WSReconnectBackoff <= 0 {
		cfg.WSReconnectBackoff = 2 * time.Second
	}
	return &Client{
		cfg:    cfg,
		http:   &http.Client{Timeout: cfg.HTTPTimeout},
		logger: logger.With().Str("component", "ari_client").Logger(),
	}
}

// ----------------------------------------------------------------
// ARI HTTP wrappers
// ----------------------------------------------------------------

func (c *Client) do(ctx context.Context, method, path string, query url.Values, body any, out any) error {
	c.mu.Lock()
	closed := c.closed
	c.mu.Unlock()
	if closed {
		return domain.ErrAsteriskUnavailable
	}

	u := c.cfg.BaseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("ari: marshal body: %w", err)
		}
		reader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, u, reader)
	if err != nil {
		return fmt.Errorf("ari: build request: %w", err)
	}
	req.SetBasicAuth(c.cfg.Username, c.cfg.Password)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("ari: %s %s: %w", method, path, err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode/100 != 2 {
		raw, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ari: %s %s status=%d body=%s", method, path, resp.StatusCode, string(raw))
	}

	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("ari: decode response: %w", err)
		}
	}
	return nil
}

// ----------------------------------------------------------------
// AsteriskGateway implementation
// ----------------------------------------------------------------

// OriginateCustomer dials the customer's SIP endpoint and bridges to the Stasis app.
//
//	endpoint e.g. "PJSIP/customer-endpoint"
//
// POST /ari/channels?endpoint=...&app=callapp&appArgs=callID
func (c *Client) OriginateCustomer(ctx context.Context, callID uuid.UUID, customerEndpoint string) (string, error) {
	q := url.Values{}
	q.Set("endpoint", customerEndpoint)
	q.Set("app", c.cfg.AppName)
	q.Set("appArgs", callID.String())
	q.Set("callerId", "DongDo <cs>")
	q.Set("timeout", "30")
	q.Set("variables", `{"CALL_ID":"`+callID.String()+`","ROLE":"customer"}`)

	var out struct {
		ID string `json:"id"`
	}
	if err := c.do(ctx, http.MethodPost, "/channels", q, nil, &out); err != nil {
		return "", err
	}
	return out.ID, nil
}

// OriginateAgent dials the agent's SIP endpoint into the Stasis app.
func (c *Client) OriginateAgent(ctx context.Context, callID uuid.UUID, agentID string) (string, error) {
	q := url.Values{}
	q.Set("endpoint", agentSIPEndpoint(agentID))
	q.Set("app", c.cfg.AppName)
	q.Set("appArgs", callID.String())
	q.Set("callerId", "Customer <cs>")
	q.Set("timeout", "30")
	q.Set("variables", `{"CALL_ID":"`+callID.String()+`","ROLE":"agent","AGENT_ID":"`+agentID+`"}`)

	var out struct {
		ID string `json:"id"`
	}
	if err := c.do(ctx, http.MethodPost, "/channels", q, nil, &out); err != nil {
		return "", err
	}
	return out.ID, nil
}

// BridgeChannels adds channels into a holding bridge.
func (c *Client) BridgeChannels(ctx context.Context, bridgeID string, channelIDs ...string) error {
	for _, ch := range channelIDs {
		path := fmt.Sprintf("/bridges/%s/addChannel", bridgeID)
		q := url.Values{}
		q.Set("channel", ch)
		if err := c.do(ctx, http.MethodPost, path, q, nil, nil); err != nil {
			return err
		}
	}
	return nil
}

// CreateBridge creates a holding bridge.
func (c *Client) CreateBridge(ctx context.Context, bridgeType string) (string, error) {
	if bridgeType == "" {
		bridgeType = "holding"
	}
	q := url.Values{}
	q.Set("type", bridgeType)

	var out struct {
		ID string `json:"id"`
	}
	if err := c.do(ctx, http.MethodPost, "/bridges", q, nil, &out); err != nil {
		return "", err
	}
	return out.ID, nil
}

// Hangup terminates all channels belonging to a call by searching the call_id variable.
func (c *Client) Hangup(ctx context.Context, callID uuid.UUID) error {
	// List channels filtered by variable CALL_ID=<id>.
	q := url.Values{}
	q.Set("variable", "CALL_ID="+callID.String())

	var channels []struct {
		ID string `json:"id"`
	}
	if err := c.do(ctx, http.MethodGet, "/channels", q, nil, &channels); err != nil {
		return err
	}
	for _, ch := range channels {
		if err := c.do(ctx, http.MethodDelete, "/channels/"+ch.ID, nil, nil, nil); err != nil {
			return err
		}
	}
	return nil
}

// StartRecording triggers MixMonitor on both channels via local channel variables.
// For brevity we use the ARI recording API on the bridge.
func (c *Client) StartRecording(ctx context.Context, callID uuid.UUID, bridgeID string) error {
	if !c.cfg.RecordingEnabled {
		return nil
	}
	q := url.Values{}
	q.Set("name", "call_"+callID.String())
	q.Set("format", "wav")
	q.Set("ifExists", "fail")

	var out struct {
		Name string `json:"name"`
	}
	if err := c.do(ctx, http.MethodPost, "/bridges/"+bridgeID+"/record", q, nil, &out); err != nil {
		return err
	}
	return nil
}

// StopRecording stops the in-progress recording.
func (c *Client) StopRecording(ctx context.Context, callID uuid.UUID) error {
	name := "call_" + callID.String()
	return c.do(ctx, http.MethodDelete, "/recordings/live/"+name, nil, nil, nil)
}

// HealthCheck issues a cheap GET /ari/endpoints/SELF.
func (c *Client) HealthCheck(ctx context.Context) error {
	if err := c.do(ctx, http.MethodGet, "/endpoints/SELF", nil, nil, nil); err != nil {
		return domain.ErrAsteriskUnavailable
	}
	return nil
}

// Close marks the client as closed; ongoing calls are unaffected.
func (c *Client) Close() error {
	c.mu.Lock()
	c.closed = true
	c.mu.Unlock()
	return nil
}

// agentSIPEndpoint maps an agent_id to its PJSIP endpoint name.
// Convention: PJSIP/agent-<username>
func agentSIPEndpoint(agentID string) string {
	return "PJSIP/agent-" + agentID
}

// CustomerEndpoint maps a customer_id to its PJSIP endpoint.
// Convention: PJSIP/customer-<id>. Real deployments may source this from a
// directory (e.g. guests table) — keep the mapping here so the use case
// stays free of SIP-specific conventions.
func CustomerEndpoint(customerID string) string {
	return "PJSIP/customer-" + customerID
}

// ----------------------------------------------------------------
// ARI mock — domain.AsteriskGateway implementation for tests & dev.
// ----------------------------------------------------------------

// MockGateway is an in-memory AsteriskGateway for tests and dev environments
// without an Asterisk server. It is goroutine-safe.
type MockGateway struct {
	mu       sync.Mutex
	logger   zerolog.Logger
	bridges  map[string][]string // bridgeID → channelIDs
	channels map[string]struct{ callID uuid.UUID }
	custChan map[uuid.UUID]string // callID → customer channel ID (most recent)
	agentCh  map[uuid.UUID]string // callID → agent channel ID (most recent)
	hangups  []uuid.UUID
	recOn    map[uuid.UUID]bool
	healthy  bool

	// originateHook is called for every Originate{Agent,Customer} success.
	// Tests use this to count originations and assert idempotency.
	originateHook func()
}

// NewMockGateway returns a healthy in-memory gateway.
func NewMockGateway(logger zerolog.Logger) *MockGateway {
	return &MockGateway{
		logger:   logger.With().Str("component", "ari_mock").Logger(),
		bridges:  make(map[string][]string),
		channels: make(map[string]struct{ callID uuid.UUID }),
		custChan: make(map[uuid.UUID]string),
		agentCh:  make(map[uuid.UUID]string),
		recOn:    make(map[uuid.UUID]bool),
		healthy:  true,
	}
}

// SetHealthy toggles health (used by tests to simulate Asterisk downtime).
func (m *MockGateway) SetHealthy(v bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.healthy = v
}

func (m *MockGateway) OriginateCustomer(_ context.Context, callID uuid.UUID, _ string) (string, error) {
	id, err := m.originate(callID)
	if err != nil {
		return "", err
	}
	m.mu.Lock()
	m.custChan[callID] = id
	if m.originateHook != nil {
		fn := m.originateHook
		m.mu.Unlock()
		fn()
	} else {
		m.mu.Unlock()
	}
	return id, nil
}

func (m *MockGateway) OriginateAgent(_ context.Context, callID uuid.UUID, _ string) (string, error) {
	id, err := m.originate(callID)
	if err != nil {
		return "", err
	}
	m.mu.Lock()
	m.agentCh[callID] = id
	if m.originateHook != nil {
		fn := m.originateHook
		m.mu.Unlock()
		fn()
	} else {
		m.mu.Unlock()
	}
	return id, nil
}

func (m *MockGateway) originate(callID uuid.UUID) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.healthy {
		return "", domain.ErrAsteriskUnavailable
	}
	id := "ch-" + callID.String()[:8]
	m.channels[id] = struct{ callID uuid.UUID }{callID: callID}
	return id, nil
}

func (m *MockGateway) CreateBridge(_ context.Context, _ string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.healthy {
		return "", domain.ErrAsteriskUnavailable
	}
	id := fmt.Sprintf("bridge-%d", time.Now().UnixNano())
	m.bridges[id] = nil
	return id, nil
}

func (m *MockGateway) BridgeChannels(_ context.Context, bridgeID string, channelIDs ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.healthy {
		return domain.ErrAsteriskUnavailable
	}
	m.bridges[bridgeID] = append(m.bridges[bridgeID], channelIDs...)
	return nil
}

func (m *MockGateway) Hangup(_ context.Context, callID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.healthy {
		return domain.ErrAsteriskUnavailable
	}
	m.hangups = append(m.hangups, callID)
	for id, ch := range m.channels {
		if ch.callID == callID {
			delete(m.channels, id)
		}
	}
	return nil
}

func (m *MockGateway) StartRecording(_ context.Context, callID uuid.UUID, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.recOn[callID] = true
	return nil
}

func (m *MockGateway) StopRecording(_ context.Context, callID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.recOn, callID)
	return nil
}

func (m *MockGateway) HealthCheck(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.healthy {
		return domain.ErrAsteriskUnavailable
	}
	return nil
}

// SetOriginateHook installs a callback invoked once for every successful
// OriginateCustomer/OriginateAgent. Used by tests to assert idempotency.
func (m *MockGateway) SetOriginateHook(fn func()) {
	m.mu.Lock()
	m.originateHook = fn
	m.mu.Unlock()
}

// LastCustomerChannel returns the most recent customer channel ID for a
// call (for tests that need to drive ChannelStateChange events).
func (m *MockGateway) LastCustomerChannel(callID uuid.UUID) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.custChan[callID]
}

// LastAgentChannel returns the most recent agent channel ID for a call.
func (m *MockGateway) LastAgentChannel(callID uuid.UUID) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.agentCh[callID]
}

// Compile-time checks
var (
	_ domain.AsteriskGateway = (*Client)(nil)
	_ domain.AsteriskGateway = (*MockGateway)(nil)
)

// Unused-import guards
var _ = errors.New
