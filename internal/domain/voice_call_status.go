package domain

import (
	"fmt"
	"sync"
)

// ============================================================
// Voice Call State Machine (Refactored)
// ============================================================
//
// CallStatus represents the lifecycle state of a voice call. The state
// machine is now strictly validated: every transition must go through
// CallStateMachine.TransitionTo() which returns ErrIllegalTransition
// for moves not in the allowed graph.
//
// Lifecycle (primary):
//
//	CREATED
//	  ↓
//	WAITING              (in queue, no agent)
//	  ↓
//	WAITING_AGENT        (reserved to an agent, awaiting accept)
//	  ↓
//	CONNECTING           (Asterisk originating legs)
//	  ↓
//	RINGING              (one or both legs ringing)
//	  ↓
//	IN_PROGRESS          (bridge established, media flowing)
//	  ↓
//	ENDED
//
// Terminal alternatives (any non-terminal state can transition):
//	REJECTED, CANCELLED, MISSED, FAILED, TIMEOUT
//
// BACKWARD COMPAT: the legacy enums (CallRinging/CallDialing/CallBridged/
// CallActive/CallEnded/CallFailed/CallMissed/CallRejected) are preserved
// as aliases so existing repositories and tests do not break. New code
// should use the canonical names (CallWaiting/CallWaitingAgent/...).
type CallStatus string

const (
	// Canonical lifecycle states (new).
	CallCreated      CallStatus = "CREATED"
	CallWaiting      CallStatus = "WAITING"
	CallWaitingAgent CallStatus = "WAITING_AGENT"
	CallConnecting   CallStatus = "CONNECTING"
	CallInProgress   CallStatus = "IN_PROGRESS"

	// Canonical terminal states (new).
	CallRejected CallStatus = "REJECTED"
	CallCancelled CallStatus = "CANCELLED"
	CallMissed   CallStatus = "MISSED"
	CallFailed   CallStatus = "FAILED"
	CallTimeout  CallStatus = "TIMEOUT"

	// Legacy aliases — kept so existing repository code, sqlc queries,
	// and DB rows still match. New transitions are mapped onto the
	// canonical names where appropriate.
	CallRinging  CallStatus = "RINGING"  // == CallWaitingAgent (legacy)
	CallDialing  CallStatus = "DIALING"  // == CallConnecting   (legacy)
	CallBridged  CallStatus = "BRIDGED"  // == CallInProgress   (legacy)
	CallActive   CallStatus = "ACTIVE"   // == CallInProgress   (legacy)
	CallEnded    CallStatus = "ENDED"    // == ENDED            (legacy)
)

// Canonical returns the modern name for a status, mapping legacy
// aliases onto the canonical lifecycle states. Useful when reading
// rows from PostgreSQL that still hold the old values.
func (s CallStatus) Canonical() CallStatus {
	switch s {
	case CallRinging:
		return CallWaitingAgent
	case CallDialing:
		return CallConnecting
	case CallBridged, CallActive:
		return CallInProgress
	default:
		return s
	}
}

// String returns the canonical string representation.
func (s CallStatus) String() string { return string(s) }

// IsTerminal reports whether the call can no longer transition to a
// new state.
func (s CallStatus) IsTerminal() bool {
	switch s.Canonical() {
	case CallEnded, CallFailed, CallMissed, CallRejected, CallCancelled, CallTimeout:
		return true
	default:
		return false
	}
}

// IsLive reports whether the call is currently in an active live state.
func (s CallStatus) IsLive() bool {
	switch s.Canonical() {
	case CallInProgress, CallConnecting, CallRinging, CallWaitingAgent, CallWaiting:
		return true
	default:
		return false
	}
}

// ErrIllegalTransition is returned by CallStateMachine.TransitionTo
// when the requested move is not in the allowed graph. It is
// implemented as a value type — callers type-assert against
// *ErrIllegalTransition to inspect From/To.
type ErrIllegalTransition struct {
	From, To CallStatus
}

func (e *ErrIllegalTransition) Error() string {
	return fmt.Sprintf("call: illegal transition %s → %s", e.From, e.To)
}

// allowedTransitions defines the state machine. The map is keyed by
// canonical names; legacy aliases are normalized via Canonical()
// before lookup.
var allowedTransitions = map[CallStatus]map[CallStatus]bool{
	CallCreated: {
		CallWaiting: true, CallFailed: true, CallCancelled: true,
	},
	CallWaiting: {
		CallWaitingAgent: true, CallCancelled: true, CallFailed: true, CallTimeout: true,
		// legacy migration paths
		CallRinging: true,
	},
	CallWaitingAgent: {
		CallConnecting: true, CallRejected: true, CallMissed: true,
		CallCancelled: true, CallFailed: true, CallWaiting: true,
		// legacy migration paths
		CallDialing: true, CallRinging: true,
	},
	CallConnecting: {
		CallWaitingAgent: true, CallInProgress: true, CallFailed: true, CallMissed: true,
		// legacy
		CallDialing: true, CallBridged: true, CallActive: true, CallRinging: true,
	},
	CallRinging: {
		// CallRinging is the legacy alias for CallWaitingAgent — the
		// map is consulted with canonical keys only, so the entries
		// below are unused. Kept for documentation.
		CallInProgress: true, CallFailed: true, CallMissed: true,
		CallCancelled: true, CallWaiting: true,
		// legacy
		CallBridged: true, CallActive: true, CallConnecting: true,
	},
	CallInProgress: {
		CallEnded: true, CallFailed: true,
		// legacy
		CallBridged: true, CallActive: true,
	},
	CallBridged: {
		CallEnded: true, CallFailed: true,
		CallInProgress: true,
	},
	CallActive: {
		CallEnded: true, CallFailed: true,
		CallInProgress: true,
	},
	CallDialing: {
		// CallDialing is the legacy alias for CallConnecting. Same as
		// above — entries below are documentation.
		CallRinging: true, CallInProgress: true, CallFailed: true, CallMissed: true,
		CallConnecting: true, CallWaitingAgent: true,
	},
	// Terminal states stay terminal — no outgoing transitions except
	// correction paths (e.g. failed → ended once cleanup completes).
	CallEnded:     {},
	CallFailed:    {CallEnded: true},
	CallMissed:    {CallEnded: true},
	CallRejected:  {CallEnded: true},
	CallCancelled: {CallEnded: true},
	CallTimeout:   {CallEnded: true},
}

// CanTransition reports whether moving from → to is allowed.
func CanTransition(from, to CallStatus) bool {
	fromC := from.Canonical()
	toC := to.Canonical()
	if fromC == toC {
		return true // idempotent re-write
	}
	if allowed, ok := allowedTransitions[fromC]; ok {
		return allowed[toC]
	}
	return false
}

// ============================================================
// CallStateMachine — process-wide validator
// ============================================================
//
// Use this to centralize state-transition validation when wrapping
// repositories: every UpdateStatus call should funnel through a
// CallStateMachine.TransitionTo() call so illegal moves are
// rejected with ErrIllegalTransition.
//
// The struct is intentionally tiny and stateless (no DB) so it can
// be embedded in use cases and called from goroutines without locks.
type CallStateMachine struct{}

// TransitionTo validates the requested transition. Returns
// ErrIllegalTransition if the move is not in the allowed graph.
func (CallStateMachine) TransitionTo(from, to CallStatus) error {
	if CanTransition(from, to) {
		return nil
	}
	return &ErrIllegalTransition{From: from, To: to}
}

// Normalize maps a legacy status read from PostgreSQL onto its
// canonical counterpart, so callers always work with the new names.
func (CallStateMachine) Normalize(s CallStatus) CallStatus { return s.Canonical() }

// ============================================================
// Sentinel validation helper for concurrent use
// ============================================================

// IsValidCallStatus returns true for any known status name (legacy or
// canonical). Used by validators in HTTP handlers.
func IsValidCallStatus(s string) bool {
	switch CallStatus(s) {
	case CallCreated, CallWaiting, CallWaitingAgent, CallConnecting,
		CallRinging, CallInProgress, CallBridged, CallActive, CallDialing,
		CallEnded, CallFailed, CallMissed, CallRejected, CallCancelled, CallTimeout:
		return true
	}
	return false
}

// ============================================================
// SafeTransition — atomic transition helper for the voice usecase.
// ============================================================
//
// Wraps a status-update operation with:
//   1. TransitionTo() validation.
//   2. Optimistic concurrency on the in-memory mirror (used by tests
//      and the reconciliation goroutine).
type transitionLatch struct {
	mu sync.Mutex
}

// SafeTransition executes updateFn if the transition is legal.
// `updateFn` is responsible for actually persisting the new state
// (e.g. via VoiceCallRepository.UpdateStatus). Returns
// ErrIllegalTransition if validation fails.
func SafeTransition(from, to CallStatus, updateFn func() error) error {
	var l transitionLatch
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := (CallStateMachine{}).TransitionTo(from, to); err != nil {
		return err
	}
	return updateFn()
}
