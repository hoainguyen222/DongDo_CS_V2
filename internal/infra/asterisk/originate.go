package asterisk

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
)

// Originate asks Asterisk to place an outbound call per req. The high-level
// flow is:
//
//  1. Translate the domain request into AMI action parameters (Channel,
//     Exten, Context, Priority, CallerID, Variables).
//  2. Submit Action: Originate.
//  3. For synchronous originates wait for the synchronous Response and
//     return; for async originates the response only confirms Asterisk has
//     accepted the request - the actual channel id arrives as a Newchannel
//     event later.
//
// We never refuse to send an Originate when the channel id is unknown up
// front: Asterisk will allocate one asynchronously and the voice usecase
// will correlate via the UniqueID field in subsequent events.
func (c *Client) Originate(ctx context.Context, req domain.OriginateRequest) (*domain.OriginateResult, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("asterisk: cannot originate while disconnected")
	}

	params := map[string]string{
		"Channel":  req.Channel,
		"Exten":    req.Exten,
		"Context":  req.Context,
		"Priority": fmt.Sprintf("%d", priorityOr(req.Priority, 1)),
	}
	if req.CallerID != "" {
		params["CallerID"] = req.CallerID
	}
	if req.Timeout > 0 {
		params["Timeout"] = fmt.Sprintf("%d", req.Timeout)
	}
	// Async: don't block the AMI session waiting for the originate to
	// complete; events will tell us what happened.
	if req.Async {
		params["Async"] = "yes"
	}

	// AMI accepts multiple `Variable:` headers to set channel variables.
	// We preserve them by setting them directly in the params map -
	// buildActionMessage will emit each as its own line.
	for k, v := range req.Variables {
		params["Variable: "+k] = v
	}

	resp, err := c.Action(ctx, "Originate", params)
	if err != nil {
		return nil, fmt.Errorf("originate: %w", err)
	}

	res := &domain.OriginateResult{
		ActionID:   actionIDFromResp(resp),
		Channel:    req.Channel,
		Context:    req.Context,
		Exten:      req.Exten,
		RawMessage: resp.values,
		Success:    resp.success(),
		Reason:     resp.diagnose(),
	}

	if v, ok := resp.values["Channel"]; ok && v != "" {
		res.Channel = v
	}
	if v, ok := resp.values["Exten"]; ok && v != "" {
		res.Exten = v
	}
	if v, ok := resp.values["Context"]; ok && v != "" {
		res.Context = v
	}

	return res, nil
}

// OriginateToAgent dials an outbound agent extension with sensible defaults
// pulled from the configured Context/Trunk. It is the entry point used by
// VoiceUseCase.InitiateCall for the agent leg of a two-party call.
func (c *Client) OriginateToAgent(ctx context.Context, agentExten, callerID string, vars map[string]string) (*domain.OriginateResult, error) {
	if agentExten == "" {
		return nil, fmt.Errorf("asterisk: agent extension is empty")
	}
	channel := "PJSIP/" + agentExten
	merged := mergeVars(vars, map[string]string{
		"DD_LEG":          "agent",
		"DD_AGENT_EXTEN":  agentExten,
	})
	return c.Originate(ctx, domain.OriginateRequest{
		Channel:  channel,
		Exten:    agentExten,
		Context:  c.cfg.Context,
		Priority: 1,
		CallerID: callerID,
		Async:    true,
		Variables: merged,
	})
}

// OriginateToGuest dials the guest's phone number via the configured outbound
// trunk. The guest phone number must already be in dialable form (no leading
// + or spaces).
func (c *Client) OriginateToGuest(ctx context.Context, guestPhone, callerID string, vars map[string]string) (*domain.OriginateResult, error) {
	if guestPhone == "" {
		return nil, fmt.Errorf("asterisk: guest phone is empty")
	}
	channel := c.cfg.Trunk + "/" + sanitizePhone(guestPhone)
	merged := mergeVars(vars, map[string]string{
		"DD_LEG":         "guest",
		"DD_GUEST_PHONE": guestPhone,
	})
	return c.Originate(ctx, domain.OriginateRequest{
		Channel:  channel,
		Exten:    sanitizePhone(guestPhone),
		Context:  c.cfg.Context,
		Priority: 1,
		CallerID: callerID,
		Async:    true,
		Variables: merged,
	})
}

// Hangup terminates the call on the given channel.
func (c *Client) Hangup(ctx context.Context, channel string) error {
	if channel == "" {
		return fmt.Errorf("asterisk: hangup called with empty channel")
	}
	resp, err := c.Action(ctx, "Hangup", map[string]string{
		"Channel": channel,
	})
	if err != nil {
		return fmt.Errorf("hangup: %w", err)
	}
	if !resp.success() {
		return fmt.Errorf("hangup rejected: %s", resp.diagnose())
	}
	return nil
}

// Redirect reroutes an in-progress channel to (exten, context, priority).
// priority is passed as a string because Asterisk accepts labels too.
func (c *Client) Redirect(ctx context.Context, channel, exten, contextName, priority string) error {
	if channel == "" {
		return fmt.Errorf("asterisk: redirect called with empty channel")
	}
	if exten == "" {
		return fmt.Errorf("asterisk: redirect requires a target extension")
	}
	if contextName == "" {
		contextName = c.cfg.Context
	}
	if priority == "" {
		priority = "1"
	}
	resp, err := c.Action(ctx, "Redirect", map[string]string{
		"Channel":   channel,
		"Exten":     exten,
		"Context":   contextName,
		"Priority":  priority,
	})
	if err != nil {
		return fmt.Errorf("redirect: %w", err)
	}
	if !resp.success() {
		return fmt.Errorf("redirect rejected: %s", resp.diagnose())
	}
	return nil
}

// Transfer is a convenience wrapper around Redirect for attended/blind
// transfer flows. It currently performs a blind transfer (Redirect only).
func (c *Client) Transfer(ctx context.Context, channel, targetExten string) error {
	if targetExten == "" {
		return fmt.Errorf("asterisk: transfer target extension is empty")
	}
	// Blind transfer: drop the channel into the target extension's context.
	return c.Redirect(ctx, channel, targetExten, c.cfg.Context, "1")
}

// Monitor starts MixMonitor on the channel, writing the recording to the
// path/filename supplied. Asterisk will save to filename relative to its
// configured monitoring directory unless an absolute path is given.
func (c *Client) Monitor(ctx context.Context, channel, filename string) error {
	if channel == "" {
		return fmt.Errorf("asterisk: monitor called with empty channel")
	}
	if filename == "" {
		filename = fmt.Sprintf("dongdo-call-%d", time.Now().UnixMilli())
	}
	resp, err := c.Action(ctx, "MixMonitor", map[string]string{
		"Channel": channel,
		"File":    filename,
		"Format":  "wav",
	})
	if err != nil {
		return fmt.Errorf("mixmonitor: %w", err)
	}
	if !resp.success() {
		return fmt.Errorf("mixmonitor rejected: %s", resp.diagnose())
	}
	return nil
}

// StopMonitor ends MixMonitor and returns the filename Asterisk reported
// back in the StopMixMonitor event. Because MixMonitor does not return a
// filename in its action response, callers should rely on the recording
// filename they passed to Monitor.
func (c *Client) StopMonitor(ctx context.Context, channel string) (string, error) {
	if channel == "" {
		return "", fmt.Errorf("asterisk: stopmonitor called with empty channel")
	}
	resp, err := c.Action(ctx, "StopMixMonitor", map[string]string{
		"Channel": channel,
	})
	if err != nil {
		return "", fmt.Errorf("stopmixmonitor: %w", err)
	}
	if !resp.success() {
		return "", fmt.Errorf("stopmixmonitor rejected: %s", resp.diagnose())
	}
	return "", nil
}

// ============================================================
// helpers
// ============================================================

func priorityOr(p, fallback int) int {
	if p <= 0 {
		return fallback
	}
	return p
}

// actionIDFromResp extracts our ActionID out of the AMI response map.
// We embed it as a normal header and Asterisk echoes it back verbatim.
func actionIDFromResp(r *amiResponse) string {
	if r == nil {
		return ""
	}
	return r.values["ActionID"]
}

// mergeVars returns a new map containing all keys from both maps; values
// from override win on conflict.
func mergeVars(base, override map[string]string) map[string]string {
	out := make(map[string]string, len(base)+len(override))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range override {
		out[k] = v
	}
	return out
}

// sanitizePhone strips characters Asterisk can't dial out of the trunk
// (spaces, dashes, parentheses, leading +). We keep digits and leading * / #.
func sanitizePhone(p string) string {
	var b strings.Builder
	for _, r := range p {
		switch {
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '*' || r == '#':
			b.WriteRune(r)
		}
	}
	return b.String()
}
