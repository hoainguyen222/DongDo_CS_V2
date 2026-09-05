package domain

import "fmt"

// ============================================================
// Agent State Machine
// ============================================================
//
// Lifecycle:
//
//	OFFLINE
//	  ↓ login / WS connect
//	AVAILABLE
//	  ↓ reserve (atomic Redis Lua)
//	RESERVED
//	  ↓ accept
//	RINGING          (Asterisk dialing the agent leg)
//	  ↓ answer (ARI StasisStart)
//	BUSY             (bridge up)
//	  ↓ hangup / bridge destroy
//	AVAILABLE
//
// Side exits from RESERVED:
//	RESERVED → AVAILABLE  (timeout, agent reject, customer cancel)
//
// Side exits from RINGING:
//	RINGING → AVAILABLE   (no answer on agent leg, timeout)
//
// Side exits from BUSY:
//	BUSY    → OFFLINE     (WS disconnect)
//	BUSY    → AVAILABLE   (graceful hangup)
type AgentStatus string

const (
	AgentOffline   AgentStatus = "OFFLINE"
	AgentAvailable AgentStatus = "AVAILABLE"
	AgentReserved  AgentStatus = "RESERVED"
	AgentRinging   AgentStatus = "RINGING"
	AgentBusy      AgentStatus = "BUSY"
	AgentAway      AgentStatus = "AWAY" // explicit "do not route" — optional
)

// String returns the canonical string form.
func (s AgentStatus) String() string { return string(s) }

// IsTerminal reports whether the agent cannot be assigned new work
// (only OFFLINE / BUSY are blockers — RESERVED and RINGING are
// transient and will either succeed or roll back).
func (s AgentStatus) IsAssignable() bool {
	return s == AgentAvailable
}

var agentTransitions = map[AgentStatus]map[AgentStatus]bool{
	AgentOffline: {
		AgentAvailable: true,
	},
	AgentAvailable: {
		// AVAILABLE can only be RESERVED or AWAY/OFFLINE; cannot jump
		// straight to BUSY (must pass through RINGING).
		AgentReserved: true, AgentOffline: true, AgentAway: true,
	},
	AgentReserved: {
		AgentRinging: true, AgentAvailable: true, AgentOffline: true,
	},
	AgentRinging: {
		AgentBusy: true, AgentAvailable: true, AgentOffline: true,
	},
	AgentBusy: {
		AgentAvailable: true, AgentOffline: true,
	},
	AgentAway: {
		AgentAvailable: true, AgentOffline: true,
	},
}

// CanTransitionAgent reports whether moving an agent from → to is allowed.
func CanTransitionAgent(from, to AgentStatus) bool {
	if from == to {
		return true
	}
	if allowed, ok := agentTransitions[from]; ok {
		return allowed[to]
	}
	return false
}

// ErrIllegalAgentTransition is returned by AgentStateMachine.TransitionTo
// when the requested move is not allowed.
type ErrIllegalAgentTransition struct {
	From, To AgentStatus
}

func (e *ErrIllegalAgentTransition) Error() string {
	return fmt.Sprintf("agent: illegal transition %s → %s", e.From, e.To)
}

// AgentStateMachine validates agent state transitions.
type AgentStateMachine struct{}

// TransitionTo returns nil if the transition is allowed,
// ErrIllegalAgentTransition otherwise.
func (AgentStateMachine) TransitionTo(from, to AgentStatus) error {
	if CanTransitionAgent(from, to) {
		return nil
	}
	return &ErrIllegalAgentTransition{From: from, To: to}
}

// IsValidAgentStatus returns true for any known status name.
func IsValidAgentStatus(s string) bool {
	switch AgentStatus(s) {
	case AgentOffline, AgentAvailable, AgentReserved, AgentRinging, AgentBusy, AgentAway:
		return true
	}
	return false
}
