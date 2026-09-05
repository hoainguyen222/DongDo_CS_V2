package domain

import (
	"context"
	"time"
)

// ============================================================
// Asterisk AMI (Asterisk Manager Interface) integration
// ============================================================
//
// This file defines the abstract interfaces that the VoiceUseCase depends on,
// so the production Asterisk TCP client (internal/infra/asterisk) and a
// NoOp fallback can be substituted without changing business logic.
//
// The interfaces are intentionally narrow: only call-control primitives the
// voice flow actually needs. Lower-level AMI mechanics live in the
// implementation package.

// OriginateRequest is the structured payload for AMI Action: Originate.
//
// Channel/Exten/Context/Priority map directly to AMI fields. Variables are
// sent as `Variable: name=value` headers, used by Asterisk dialplan for
// caller-id, CDR userfield, custom routing, etc.
type OriginateRequest struct {
	// Channel is the technology/resource to dial first (typically the caller
	// leg). Format examples: "SIP/1001", "PJSIP/alice", "Local/1001@from-internal".
	Channel string

	// Exten is the destination to dial when Channel answers (the callee).
	// Format examples: "1002", "84987654321", "queue_dongdo".
	Exten string

	// Context is the dialplan context used to route the Channel leg.
	// Defaults to "from-internal" if empty.
	Context string

	// Priority is the dialplan priority used for the Channel leg.
	// Defaults to 1 if zero.
	Priority int

	// CallerID is the string presented as the caller id on the destination
	// channel (e.g. "CSKH DongDo <1001>").
	CallerID string

	// Timeout in milliseconds. 0 means use Asterisk default.
	Timeout int

	// Variables are sent as `Variable:` AMI headers and become channel
	// variables available to the dialplan (CALLERID(name), CDR(userfield), ...).
	Variables map[string]string

	// Async reports only whether the Originate was *accepted* by Asterisk;
	// call state arrives asynchronously via AMI events.
	Async bool
}

// OriginateResult describes the channel ID Asterisk allocated for the
// outbound call (when known). Asterisk returns this in the OriginateResponse
// event; for Async originates the channel id is often empty until the
// Newchannel event fires.
type OriginateResult struct {
	ActionID   string
	ChannelID  string
	Channel    string
	Context    string
	Exten      string
	Reason     string
	Success    bool
	RawMessage map[string]string
}

// CallEvent is a typed representation of an AMI call-control event parsed by
// the events.go file in internal/infra/asterisk. Higher layers use these
// to update the database and publish WebSocket updates.
// CallEventType mirrors Asterisk AMI event categories AND the
// audit-level event types stored in `call_events`. Defined here
// (instead of in call_event.go) so both layers share a single enum.
type CallEventType string

const (
	// AMI event categories.
	CallEventNewchannel  CallEventType = "Newchannel"
	CallEventDialBegin   CallEventType = "DialBegin"
	CallEventDialEnd     CallEventType = "DialEnd"
	CallEventBridgeEnter CallEventType = "BridgeEnter"
	CallEventBridgeLeave CallEventType = "BridgeLeave"
	CallEventHold        CallEventType = "Hold"
	CallEventUnhold      CallEventType = "Unhold"
	CallEventTransfer    CallEventType = "Transfer"
	CallEventHangup      CallEventType = "Hangup"
	CallEventDial        CallEventType = "Dial"
	CallEventOther       CallEventType = "Other"

	// Audit-level event types (call_events.event_type column).
	CallEventCreated    CallEventType = "CREATED"
	CallEventQueued     CallEventType = "QUEUED"
	CallEventAssigned   CallEventType = "ASSIGNED"
	CallEventAccepted   CallEventType = "ACCEPTED"
	CallEventRejected   CallEventType = "REJECTED"
	CallEventConnecting CallEventType = "CONNECTING"
	CallEventRinging    CallEventType = "RINGING"
	CallEventBridged    CallEventType = "BRIDGED"
	CallEventEnded      CallEventType = "ENDED"
	CallEventFailed     CallEventType = "FAILED"
	CallEventMissed     CallEventType = "MISSED"
	CallEventCancelled  CallEventType = "CANCELLED"
	CallEventTimeout    CallEventType = "TIMEOUT"
	CallEventRecording  CallEventType = "RECORDING_STARTED"
)

// CallEvent is a normalized AMI event the voice flow consumes.
type CallEvent struct {
	Type        CallEventType
	Channel     string
	ChannelID   string
	UniqueID    string
	LinkedID    string
	BridgeID    string
	CallerID    string
	CallerIDNum string
	Exten       string
	Context     string
	DestChannel string
	DestUniqueID string
	ConnectedLineNum string
	ConnectedLineName string
	State       string
	Cause       string
	CauseTxt    string
	DialStatus  string
	ReceivedAt  time.Time
	Raw         map[string]string
}

// AsteriskClient is the abstract surface used by VoiceUseCase. The concrete
// implementation lives in internal/infra/asterisk and a NoOp stub lives
// next to it. Keeping this as an interface allows tests and disabled
// configurations to substitute behavior cleanly.
type AsteriskClient interface {
	// Enabled reports whether AMI integration is turned on.
	Enabled() bool

	// Connect opens the TCP socket to Asterisk, performs login and starts
	// the event-reading goroutine. It is safe to call repeatedly; a no-op
	// when already connected.
	Connect(ctx context.Context) error

	// Disconnect closes the AMI connection and stops background goroutines.
	Disconnect(ctx context.Context) error

	// IsConnected reports the live connection state.
	IsConnected() bool

	// Originate asks Asterisk to place an outbound call described by req.
	// For synchronous originates it returns once Asterisk acknowledges; for
	// async originates it returns immediately with a result that only carries
	// the ActionID, and the real channel id is delivered via the event stream.
	Originate(ctx context.Context, req OriginateRequest) (*OriginateResult, error)

	// Hangup terminates the call on the given channel id (Asterisk
	// `Action: Hangup`).
	Hangup(ctx context.Context, channel string) error

	// Redirect reroutes an in-progress channel to a new extension/context
	// (Asterisk `Action: Redirect`). Used by AcceptCall to route a ringing
	// channel to the agent's extension.
	Redirect(ctx context.Context, channel, exten, context, priority string) error

	// Transfer moves the bridge endpoint of the given channel to another
	// extension (Asterisk `Action: Redirect` with the atxfer-style target).
	Transfer(ctx context.Context, channel, targetExten string) error

	// Monitor starts MixMonitor on the channel (server-side recording).
	Monitor(ctx context.Context, channel, filename string) error

	// StopMonitor ends MixMonitor on the channel and returns the recording
	// filename if Asterisk reports it back.
	StopMonitor(ctx context.Context, channel string) (string, error)

	// Events returns a read-only channel that delivers AMI events. The
	// channel is closed when Disconnect is called.
	Events() <-chan CallEvent
}

// AsteriskClientFactory returns either a live client or a NoOp depending on
// configuration. The factory is exposed so callers do not need to know about
// concrete implementations.
type AsteriskClientFactory func(ctx context.Context, cfg AsteriskConfig) (AsteriskClient, error)

// AsteriskConfig is a minimal view of the connection settings the domain
// layer needs; the richer struct lives in internal/config. Defined here to
// keep the domain package import-clean.
type AsteriskConfig struct {
	Enabled  bool
	Host     string
	Port     int
	Username string
	Password string
	Context  string
	Trunk    string
	Queue    string
}
