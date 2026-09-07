package domain

import "errors"

// ============================================================
// Call subsystem errors (sentinel).
// Handlers map these to HTTP codes via errors.Is().
// ============================================================

var (
	ErrCallNotFound          = errors.New("call not found")
	ErrInvalidTransition     = errors.New("invalid state transition")
	ErrDuplicateAction       = errors.New("duplicate participant action")
	ErrAgentNotAvailable     = errors.New("agent not available")
	ErrAgentNotReserved      = errors.New("agent not reserved for this call")
	ErrAgentStaleReservation = errors.New("agent reservation belongs to another call")
	ErrCallAlreadyTerminal   = errors.New("call already in terminal state")
	ErrIdempotencyMismatch   = errors.New("idempotency key used for different request body")
	ErrQueueEmpty            = errors.New("queue empty")
	ErrRedisUnavailable      = errors.New("redis unavailable")
	ErrAsteriskUnavailable   = errors.New("asterisk unavailable")
)
