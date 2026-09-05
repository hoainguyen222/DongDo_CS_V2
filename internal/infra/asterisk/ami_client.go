// Package asterisk provides an Asterisk Manager Interface (AMI) client and a
// thin call-control layer (Originate, Hangup, Redirect, MixMonitor, ...).
//
// AMI is a line-oriented TCP protocol where each request is a sequence of
// "Key: Value\r\n" lines terminated by an empty line ("\r\n\r\n"). Responses
// arrive as either:
//   - A single response message with "Response: Success/Failure" and an
//     ActionID we generated up front; or
//   - An asynchronous event block, which may come before, between or after
//     responses.
//
// All messages are parsed into a `map[string]string` and the high-level
// events.go code maps them to typed `domain.CallEvent` values.
package asterisk

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
	"github.com/rs/zerolog"
)

// Client is a single-connection Asterisk Manager Interface client. It is safe
// for concurrent use: Actions are serialized through a mutex and pending
// responses are matched against the caller-supplied ActionID.
//
// The connection is automatically re-established by the supervisor goroutine
// whenever it drops, using exponential backoff bounded by ReconnectMax.
type Client struct {
	cfg    Config
	logger zerolog.Logger

	// Connection state.
	mu          sync.Mutex
	conn        net.Conn
	reader      *bufio.Reader
	writer      *bufio.Writer
	writeCh     chan writeOp
	connected   atomic.Bool
	loginPassed atomic.Bool
	stopCh      chan struct{}
	stopOnce    sync.Once

	// Pending response registry: ActionID -> waiting channel.
	pending   sync.Map // map[string]chan amiResponse

	// Async event sink consumed by Events().
	eventCh chan domain.CallEvent

	// Stats.
	reconnects atomic.Int64
}

// NewClient constructs a Client with the supplied config. Connect must be
// called separately before any Actions can be sent. Returns an error if the
// config is incomplete.
func NewClient(cfg Config) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("asterisk client: %w", err)
	}

	c := &Client{
		cfg:     cfg,
		logger:  newLogger().With().Str("component", "asterisk_client").Logger(),
		stopCh:  make(chan struct{}),
		writeCh: make(chan writeOp, 64),
		eventCh: make(chan domain.CallEvent, cfg.EventBufferSize),
	}
	return c, nil
}

// Enabled reports whether AMI integration is on. The Asterisk integration
// can be constructed-but-disabled by leaving cfg.Password empty or by
// checking Enabled on the higher-level config. This helper lets us assert
// either case uniformly.
func (c *Client) Enabled() bool { return true }

// IsConnected returns true when the TCP socket is currently up and the
// Asterisk handshake (banner + Login) has completed successfully.
func (c *Client) IsConnected() bool {
	return c.connected.Load() && c.loginPassed.Load()
}

// Events exposes the read end of the async event channel. The channel is
// closed when Disconnect is called or the supervisor goroutine exits.
func (c *Client) Events() <-chan domain.CallEvent {
	return c.eventCh
}

// Connect opens the TCP socket, completes the AMI banner + Login handshake
// and spawns the supervisor goroutine that handles auto-reconnect. It is
// safe to call when already connected - it returns immediately.
//
// The context is used only for the initial connect; once connected, the
// supervisor keeps the client alive in the background.
func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	if c.connected.Load() {
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()

	if err := c.dialAndLogin(ctx); err != nil {
		return fmt.Errorf("asterisk: initial connect failed: %w", err)
	}

	go c.supervisor()

	return nil
}

// dialAndLogin performs the full connect+login sequence. It is retry-safe:
// any pending state from a previous run is cleared first.
func (c *Client) dialAndLogin(ctx context.Context) error {
	c.mu.Lock()
	// Reset state on retry.
	if c.conn != nil {
		_ = c.conn.Close()
		c.conn = nil
		c.reader = nil
		c.writer = nil
	}
	c.loginPassed.Store(false)

	conn, err := c.dialWithContext(ctx)
	if err != nil {
		c.mu.Unlock()
		return fmt.Errorf("dial: %w", err)
	}
	c.conn = conn
	c.reader = bufio.NewReader(conn)
	c.writer = bufio.NewWriter(conn)
	c.mu.Unlock()

	if err := c.readBanner(); err != nil {
		_ = conn.Close()
		return fmt.Errorf("read banner: %w", err)
	}

	loginResp, err := c.login(ctx)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("login: %w", err)
	}
	if !loginResp.success() {
		_ = conn.Close()
		return fmt.Errorf("login rejected: %s", loginResp.diagnose())
	}

	c.connected.Store(true)
	c.loginPassed.Store(true)
	c.logger.Info().
		Str("host", c.cfg.Host).
		Int("port", c.cfg.Port).
		Str("username", c.cfg.Username).
		Msg("AMI connected and authenticated")

	// Spawn reader/writer goroutines for the new connection.
	go c.readLoop()
	go c.writeLoop()

	return nil
}

// dialWithContext wraps net.Dialer.DialContext so we honor ctx cancellation
// while still applying the configured DialTimeout.
func (c *Client) dialWithContext(ctx context.Context) (net.Conn, error) {
	d := net.Dialer{Timeout: c.cfg.DialTimeout}
	return d.DialContext(ctx, "tcp", net.JoinHostPort(c.cfg.Host, strconv.Itoa(c.cfg.Port)))
}

// readBanner waits for the initial "Asterisk Call Manager/1.x" greeting.
func (c *Client) readBanner() error {
	c.mu.Lock()
	r := c.reader
	c.mu.Unlock()
	if r == nil {
		return errors.New("reader not initialized")
	}

	c.mu.Lock()
	_ = c.conn.SetReadDeadline(time.Now().Add(c.cfg.ReadTimeout))
	c.mu.Unlock()

	msg, err := readMessage(r)
	if err != nil {
		return err
	}
	if _, ok := msg["Response"]; !ok {
		return fmt.Errorf("unexpected banner: %v", msg)
	}
	c.logger.Debug().Str("banner", msg["Response"]).Msg("AMI banner received")
	return nil
}

// login sends Action: Login and waits for the synchronous response.
func (c *Client) login(ctx context.Context) (*amiResponse, error) {
	resp, err := c.Action(ctx, "Login", map[string]string{
		"Username": c.cfg.Username,
		"Secret":   c.cfg.Password,
		"Events":   "on",
	})
	if err != nil {
		return nil, err
	}
	if !resp.success() {
		return resp, fmt.Errorf("login failed: %s", resp.diagnose())
	}
	return resp, nil
}

// Disconnect signals the supervisor to stop, closes the connection and
// releases all pending callers. After Disconnect the client must be
// re-initialized via Connect to be usable again.
func (c *Client) Disconnect(ctx context.Context) error {
	var firstErr error
	c.stopOnce.Do(func() {
		close(c.stopCh)
	})

	c.mu.Lock()
	if c.conn != nil {
		if err := c.conn.Close(); err != nil {
			firstErr = err
		}
		c.conn = nil
		c.reader = nil
		c.writer = nil
	}
	c.mu.Unlock()
	c.connected.Store(false)
	c.loginPassed.Store(false)

	// Wake up any pending action callers so they don't hang forever.
	c.pending.Range(func(k, v interface{}) bool {
		ch, _ := v.(chan amiResponse)
		select {
		case ch <- amiResponse{values: map[string]string{"Response": "Disconnect"}, isResponse: true}:
		default:
		}
		return true
	})

	// Close event channel after a short grace period so consumers have a
	// chance to drain.
	go func() {
		// Allow time for readLoop/writeLoop to exit.
		time.Sleep(50 * time.Millisecond)
		close(c.eventCh)
	}()

	c.logger.Info().Msg("AMI client disconnected")
	return firstErr
}

// supervisor runs in the background and keeps the client connected across
// transient network failures. It exits when stopCh is closed.
func (c *Client) supervisor() {
	backoff := c.cfg.ReconnectMin
	for {
		select {
		case <-c.stopCh:
			return
		default:
		}

		if c.IsConnected() {
			// Idle until something goes wrong. We rely on readLoop errors
			// to detect disconnect; check periodically to recover.
			time.Sleep(1 * time.Second)
			continue
		}

		c.reconnects.Add(1)
		c.logger.Warn().
			Dur("backoff", backoff).
			Msg("AMI reconnecting")

		ctx, cancel := context.WithTimeout(context.Background(), c.cfg.DialTimeout+c.cfg.LoginTimeout+2*time.Second)
		err := c.dialAndLogin(ctx)
		cancel()

		if err != nil {
			c.logger.Error().Err(err).Dur("backoff", backoff).Msg("AMI reconnect failed")
			select {
			case <-c.stopCh:
				return
			case <-time.After(backoff):
			}
			backoff = nextBackoff(backoff, c.cfg.ReconnectMax)
			continue
		}

		backoff = c.cfg.ReconnectMin
	}
}

// nextBackoff doubles backoff up to max.
func nextBackoff(current, max time.Duration) time.Duration {
	next := current * 2
	if next > max {
		return max
	}
	return next
}

// ============================================================
// AMI write/read primitives
// ============================================================

// writeOp is a unit of work submitted to the writeLoop. It serializes access
// to the underlying connection so concurrent Action() callers cannot interleave
// their key/value lines.
type writeOp struct {
	payload []byte
	done    chan struct{}
}

// writeLoop owns the only writer reference and flushes complete AMI
// messages in the order they are queued. It exits when stopCh closes or the
// underlying writer returns an error.
func (c *Client) writeLoop() {
	for {
		select {
		case <-c.stopCh:
			return
		case op, ok := <-c.writeCh:
			if !ok {
				return
			}
			c.mu.Lock()
			w := c.writer
			conn := c.conn
			c.mu.Unlock()
			if w == nil || conn == nil {
				close(op.done)
				continue
			}
			if err := conn.SetWriteDeadline(time.Now().Add(c.cfg.WriteTimeout)); err != nil {
				c.logger.Error().Err(err).Msg("AMI SetWriteDeadline failed")
				close(op.done)
				continue
			}
			if _, err := w.Write(op.payload); err != nil {
				c.logger.Error().Err(err).Msg("AMI write failed")
				c.markDisconnected()
				close(op.done)
				continue
			}
			if err := w.Flush(); err != nil {
				c.logger.Error().Err(err).Msg("AMI flush failed")
				c.markDisconnected()
				close(op.done)
				continue
			}
			close(op.done)
		}
	}
}

func (c *Client) markDisconnected() {
	if c.conn != nil {
		_ = c.conn.Close()
	}
	c.connected.Store(false)
	c.loginPassed.Store(false)
}

// readLoop is the only goroutine reading from the socket. It dispatches
// each parsed message to either a pending Action caller (matched by
// ActionID) or to the event channel.
func (c *Client) readLoop() {
	defer c.markDisconnected()

	for {
		c.mu.Lock()
		r := c.reader
		c.mu.Unlock()
		if r == nil {
			return
		}

		_ = c.conn.SetReadDeadline(time.Now().Add(c.cfg.ReadTimeout))
		msg, err := readMessage(r)
		if err != nil {
			if errors.Is(err, io.EOF) || isClosedConnErr(err) {
				c.logger.Warn().Err(err).Msg("AMI socket closed")
				return
			}
			if isTimeoutErr(err) {
				// Refresh the deadline so a slow Asterisk doesn't kick us out.
				continue
			}
			c.logger.Error().Err(err).Msg("AMI read error")
			return
		}

		c.dispatch(msg)
	}
}

// dispatch routes a parsed message to either a waiting Action caller or to
// the async event stream.
func (c *Client) dispatch(msg map[string]string) {
	if msg == nil {
		return
	}

	// Responses have a Response: field; events have an Event: field.
	if _, isResp := msg["Response"]; isResp {
		actionID := msg["ActionID"]
		if actionID == "" {
			actionID = msg["ActionID"]
		}
		if actionID != "" {
			if v, ok := c.pending.LoadAndDelete(actionID); ok {
				ch := v.(chan amiResponse)
				ch <- amiResponse{values: msg, isResponse: true}
			}
			return
		}
		// No ActionID - drop but log, we can't route it back.
		c.logger.Warn().Interface("msg", msg).Msg("AMI response without ActionID")
		return
	}

	// Otherwise it's an event. Decode it into a typed CallEvent and ship
	// it on the event channel. If the buffer is full, drop oldest to keep
	// the system responsive.
	ev := parseEvent(msg)
	select {
	case c.eventCh <- ev:
	default:
		// Drain one to make room, then send.
		select {
		case <-c.eventCh:
		default:
		}
		select {
		case c.eventCh <- ev:
		default:
			c.logger.Warn().Str("event", string(ev.Type)).Msg("AMI event channel saturated; dropping")
		}
	}
}

// amiResponse is the result returned by Action(). It carries the raw
// headers plus helpers for the common Success/Failure interpretation.
type amiResponse struct {
	values     map[string]string
	isResponse bool
}

func (r amiResponse) success() bool {
	return strings.EqualFold(r.values["Response"], "Success")
}

func (r amiResponse) diagnose() string {
	if msg := r.values["Message"]; msg != "" {
		return msg
	}
	return r.values["Response"]
}

// newActionID generates a random ActionID used to correlate responses with
// the requests that produced them.
func newActionID() string {
	var b [12]byte
	_, _ = rand.Read(b[:])
	return "act-" + hex.EncodeToString(b[:])
}

// Action sends an Action to Asterisk and waits for the matching response.
// The response is identified by the ActionID we embed in the request.
//
// params maps directly to "Key: Value\r\n" AMI headers.
func (c *Client) Action(ctx context.Context, action string, params map[string]string) (*amiResponse, error) {
	return c.ActionWithResponse(ctx, action, params)
}

// ActionWithResponse is an alias of Action kept explicit for callers that
// want to emphasize they expect a response.
func (c *Client) ActionWithResponse(ctx context.Context, action string, params map[string]string) (*amiResponse, error) {
	if !c.IsConnected() {
		return nil, errors.New("asterisk: client not connected")
	}

	actionID := newActionID()
	respCh := make(chan amiResponse, 1)
	c.pending.Store(actionID, respCh)
	defer c.pending.Delete(actionID)

	payload := buildActionMessage(action, actionID, params)
	op := writeOp{payload: payload, done: make(chan struct{})}

	// Submit the write; if the writeLoop is jammed we fail fast.
	select {
	case c.writeCh <- op:
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.stopCh:
		return nil, errors.New("asterisk: client stopped")
	}

	// Wait for the write to flush before reading the response.
	select {
	case <-op.done:
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.stopCh:
		return nil, errors.New("asterisk: client stopped")
	}

	timeout := c.cfg.OriginateTimeout
	if deadline, ok := ctx.Deadline(); ok {
		if d := time.Until(deadline); d > 0 && d < timeout {
			timeout = d
		}
	}

	select {
	case resp := <-respCh:
		return &resp, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("asterisk: action %q timed out after %s", action, timeout)
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.stopCh:
		return nil, errors.New("asterisk: client stopped")
	}
}

// buildActionMessage serializes an AMI action into the wire format:
//   Action: <action>\r\n
//   ActionID: <id>\r\n
//   Key: Value\r\n
//   ...\r\n
//   \r\n
func buildActionMessage(action, actionID string, params map[string]string) []byte {
	var b strings.Builder
	b.Grow(256)
	b.WriteString("Action: ")
	b.WriteString(action)
	b.WriteString("\r\n")
	b.WriteString("ActionID: ")
	b.WriteString(actionID)
	b.WriteString("\r\n")
	for k, v := range params {
		b.WriteString(k)
		b.WriteString(": ")
		b.WriteString(v)
		b.WriteString("\r\n")
	}
	b.WriteString("\r\n")
	return []byte(b.String())
}

// readMessage parses a single AMI message block (terminated by a blank line).
// Headers are returned as a map[string]string with surrounding whitespace
// stripped and case preserved.
func readMessage(r *bufio.Reader) (map[string]string, error) {
	msg := make(map[string]string, 16)
	for {
		line, err := readLine(r)
		if err != nil {
			return nil, err
		}
		if line == "" {
			if len(msg) == 0 {
				// Empty block - keep reading; some servers emit a stray
				// CRLF between events.
				continue
			}
			return msg, nil
		}
		idx := strings.IndexByte(line, ':')
		if idx < 0 {
			// Asterisk sometimes emits lines without a colon (rare); skip.
			continue
		}
		key := line[:idx]
		val := strings.TrimSpace(line[idx+1:])
		// When the same key appears twice (e.g. Variable:) we keep the
		// last one - this matches Asterisk semantics for things like
		// OriginateResponse messages.
		msg[key] = val
	}
}

// readLine returns the next \r\n or \n terminated line with trailing
// newlines stripped. The empty line that delimits an AMI message is
// reported as "".
func readLine(r *bufio.Reader) (string, error) {
	var b strings.Builder
	for {
		c, err := r.ReadByte()
		if err != nil {
			return "", err
		}
		if c == '\r' {
			// Peek the next byte; if it's '\n' consume it; otherwise keep.
			next, err := r.ReadByte()
			if err != nil {
				return "", err
			}
			if next != '\n' {
				_ = r.UnreadByte()
			}
			return b.String(), nil
		}
		if c == '\n' {
			return b.String(), nil
		}
		b.WriteByte(c)
		if b.Len() > 4096 {
			// Defensive: bail out before we allocate unbounded memory.
			return "", fmt.Errorf("asterisk: line too long (max 4096)")
		}
	}
}

func isClosedConnErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, net.ErrClosed) {
		return true
	}
	s := err.Error()
	return strings.Contains(s, "use of closed network connection") ||
		strings.Contains(s, "broken pipe") ||
		strings.Contains(s, "connection reset by peer")
}

func isTimeoutErr(err error) bool {
	if err == nil {
		return false
	}
	var nerr net.Error
	if errors.As(err, &nerr) {
		return nerr.Timeout()
	}
	return strings.Contains(err.Error(), "i/o timeout")
}

// Stats returns counters useful for /health endpoints and tests.
func (c *Client) Stats() (connected bool, reconnects int64) {
	return c.IsConnected(), c.reconnects.Load()
}
