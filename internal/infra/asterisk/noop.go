package asterisk

import (
	"context"
	"errors"

	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
)

// NoOpClient is a domain.AsteriskClient implementation that accepts every
// call but does nothing. It is used when Asterisk is disabled, misconfigured
// or unavailable, so the rest of the application keeps working without a
// real PBX.
type NoOpClient struct {
	enabled bool
}

// NewNoOpClient returns a NoOpClient. If enabled is false, Events() returns
// a channel that never produces anything (so the consumer goroutine can
// range over it without leaking).
func NewNoOpClient(enabled bool) *NoOpClient {
	return &NoOpClient{enabled: enabled}
}

// Enabled reports whether the NoOp is enabled. We return the configured
// value so callers can distinguish "Asterisk on but not connected" (false)
// from "Asterisk deliberately off" (true here).
func (n *NoOpClient) Enabled() bool { return n.enabled }

// Connect is a no-op for the NoOp client.
func (n *NoOpClient) Connect(_ context.Context) error { return nil }

// Disconnect is a no-op for the NoOp client.
func (n *NoOpClient) Disconnect(_ context.Context) error { return nil }

// IsConnected always returns false for the NoOp client.
func (n *NoOpClient) IsConnected() bool { return false }

// Originate returns an error so callers can detect "Asterisk off" in tests
// or metrics. In normal operation the voice usecase guards on Enabled()
// before calling.
func (n *NoOpClient) Originate(_ context.Context, _ domain.OriginateRequest) (*domain.OriginateResult, error) {
	return nil, errors.New("asterisk: integration disabled")
}

// Hangup returns nil so cleanup paths stay happy when AMI is off.
func (n *NoOpClient) Hangup(_ context.Context, _ string) error { return nil }

// Redirect returns nil; the NoOp never had a channel to redirect.
func (n *NoOpClient) Redirect(_ context.Context, _, _, _, _ string) error { return nil }

// Transfer returns nil; see Redirect.
func (n *NoOpClient) Transfer(_ context.Context, _, _ string) error { return nil }

// Monitor returns nil.
func (n *NoOpClient) Monitor(_ context.Context, _, _ string) error { return nil }

// StopMonitor returns an empty filename.
func (n *NoOpClient) StopMonitor(_ context.Context, _ string) (string, error) {
	return "", nil
}

// Events returns a channel that is closed immediately so consumers' `for
// range` loops exit cleanly. This keeps the wiring in main.go identical to
// the live client.
func (n *NoOpClient) Events() <-chan domain.CallEvent {
	ch := make(chan domain.CallEvent)
	close(ch)
	return ch
}

// Factory returns the appropriate AsteriskClient for a given config. The
// returned error indicates the live client could not be built (most often
// because of a bad configuration); in that case callers should fall back
// to NewNoOpClient.
//
// The factory does not call Connect - the caller decides when to connect.
func Factory(ctx context.Context, d domain.AsteriskConfig) (domain.AsteriskClient, error) {
	if !d.Enabled {
		return NewNoOpClient(false), nil
	}
	cfg := FromDomainConfig(d)
	client, err := NewClient(cfg)
	if err != nil {
		// Config-level failure: fall back to NoOp so we don't crash the
		// process. The error is returned for the caller to log.
		return NewNoOpClient(false), err
	}
	// Best-effort connect; if Asterisk is unreachable the supervisor
	// goroutine will retry.
	if cerr := client.Connect(ctx); cerr != nil {
		// Don't propagate the error - we keep a non-nil client so the
		// supervisor goroutine can reconnect in the background.
		_ = cerr
	}
	return client, nil
}
