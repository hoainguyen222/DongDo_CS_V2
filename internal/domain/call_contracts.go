package domain

import (
	"context"
	"errors"
	"time"
)

// ============================================================
// Asterisk Gateway (Abstraction)
// ============================================================
//
// AsteriskGateway is the single interface the application layer uses
// to talk to Asterisk. The HTTP layer must NOT import
// internal/infra/asterisk directly — it must depend on this interface
// only.
//
// The gateway is implemented by:
//   - infra/asterisk.ARIService  (production, Stasis app / WebRTC)
//   - infra/asterisk.AMIService   (legacy, AMI Originate)
//
// All call-control mutations MUST go through this interface so the
// application layer is testable without a real PBX.
type AsteriskGateway interface {
	// Enabled reports whether the gateway is configured.
	Enabled() bool

	// Connected reports whether the underlying transport (ARI/AMI)
	// is currently usable.
	Connected() bool

	// OriginateGuestCall places a call to the guest endpoint (typically
	// the customer's SIP extension or PSTN trunk) and brings it into
	// our Stasis app.
	OriginateGuestCall(ctx context.Context, callID int64, sessionID, endpoint string) error

	// OriginateAgentCall places a call to the agent's SIP extension and
	// brings it into our Stasis app, then bridges with the existing
	// guest leg.
	OriginateAgentCall(ctx context.Context, callID int64, sessionID, agentExt string) error

	// HangupCall terminates both legs (if any) of the call.
	HangupCall(ctx context.Context, callID int64) error

	// StartRecording starts MixMonitor on the bridged channel.
	StartRecording(ctx context.Context, callID int64, filename string) error
}

// ============================================================
// Queue Manager (Abstraction)
// ============================================================
//
// QueueManager owns the realtime queue and agent state in Redis.
// The application layer depends on this interface so tests can
// substitute an in-memory implementation.
type QueueManager interface {
	// EnqueueCall appends callID to the FIFO queue. Returns the new
	// queue position (1-based).
	EnqueueCall(ctx context.Context, callID int64) (int, error)

	// DequeueCall pops the head of the queue.
	DequeueCall(ctx context.Context) (int64, error)

	// QueuePosition returns the 1-based position of callID, or 0 if
	// not in queue.
	QueuePosition(ctx context.Context, callID int64) (int, error)

	// AtomicReserveAgent atomically:
	//   1. Sets agent:{ext}:state = RESERVED with a TTL (so a stale
	//      reservation is auto-released).
	//   2. Stores call:{callID}:agent = ext.
	//   3. Pops the queue head (the callID passed in must match).
	// Returns (true, nil) on success. Returns (false, nil) when the
	// agent was not AVAILABLE or the queue head did not match.
	AtomicReserveAgent(ctx context.Context, callID int64, agentExt string, ttl time.Duration) (bool, error)

	// ReleaseReservation moves an agent from RESERVED back to AVAILABLE
	// (idempotent). Used by accept success / reject / timeout.
	ReleaseReservation(ctx context.Context, agentExt string) error

	// SetAgentState writes the agent's status into Redis (with optional
	// TTL on transient states like RINGING).
	SetAgentState(ctx context.Context, agentExt string, status AgentStatus, ttl time.Duration) error

	// GetAgentState returns the agent's status, or "" if no entry.
	GetAgentState(ctx context.Context, agentExt string) (AgentStatus, error)

	// SetAgentCurrentCall records the call the agent is currently
	// serving (or "" to clear).
	SetAgentCurrentCall(ctx context.Context, agentExt string, callID int64) error

	// ReserveIdempotency atomically checks-and-sets a key like
	// "call:{id}:idem:{op}". Returns (cachedResponseJSON, true) if
	// the key already existed; otherwise stores `payload` with the
	// supplied TTL and returns ("", false).
	ReserveIdempotency(ctx context.Context, key, payload string, ttl time.Duration) (existing string, hit bool, err error)
}

// ============================================================
// Common errors
// ============================================================

var (
	ErrNotFound         = errors.New("not found")
	ErrAlreadyAssigned  = errors.New("agent already assigned")
	ErrAgentUnavailable = errors.New("agent not available")
	ErrQueueEmpty       = errors.New("queue empty")
	ErrIdempotencyReplay = errors.New("idempotent replay")
)
