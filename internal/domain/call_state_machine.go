package domain

// ============================================================
// CallStateMachine — single source of truth for valid call transitions.
// Every state mutation MUST call TransitionTo before hitting the repo.
// ============================================================

var allowedCallTransitions = map[CallStatus]map[CallStatus]bool{
	CallStatusCreated: {
		CallStatusWaiting:      true,
		CallStatusCancelled:    true,
		CallStatusFailed:       true,
		CallStatusTimeout:      true,
	},
	CallStatusWaiting: {
		CallStatusWaitingAgent: true,
		CallStatusCancelled:    true,
		CallStatusTimeout:      true,
		CallStatusFailed:       true,
	},
	CallStatusWaitingAgent: {
		CallStatusRinging:    true,
		CallStatusWaiting:    true,
		CallStatusCancelled:  true,
		CallStatusMissed:     true,
		CallStatusFailed:     true,
		CallStatusTimeout:    true,
		CallStatusRejected:   true,
		CallStatusConnecting: true, // agent accepts → ARI originate starts
	},
	CallStatusConnecting: {
		CallStatusRinging:    true,
		CallStatusInProgress: true,
		CallStatusFailed:     true,
	},
	CallStatusRinging: {
		CallStatusInProgress: true,
		CallStatusFailed:     true,
		CallStatusEnded:      true, // customer dropped before agent picked up
	},
	CallStatusInProgress: {
		CallStatusEnded:  true,
		CallStatusFailed: true,
	},
	// terminal states have no outgoing transitions
}

// CanTransition reports whether moving from → to is allowed.
func CanTransition(from, to CallStatus) bool {
	if from == to {
		return false
	}
	next, ok := allowedCallTransitions[from]
	if !ok {
		return false
	}
	return next[to]
}

// TransitionTo validates the transition and returns ErrInvalidTransition if not allowed.
func (c *Call) TransitionTo(next CallStatus) error {
	if c.Status.IsTerminal() {
		return ErrInvalidTransition
	}
	if !CanTransition(c.Status, next) {
		return ErrInvalidTransition
	}
	c.Status = next
	return nil
}

// ============================================================
// AgentStateMachine
// ============================================================

var allowedAgentTransitions = map[AgentState]map[AgentState]bool{
	AgentOffline: {
		AgentAvailable: true,
	},
	AgentAvailable: {
		AgentReserved: true,
		AgentAway:     true,
		AgentOffline:  true,
	},
	AgentReserved: {
		AgentRinging:  true,
		AgentAvailable: true, // reject / timeout
		AgentOffline:  true,
	},
	AgentRinging: {
		AgentBusy:      true,
		AgentAvailable: true, // reject late
		AgentOffline:   true,
	},
	AgentBusy: {
		AgentAvailable: true,
		AgentAway:      true,
		AgentOffline:   true,
	},
	AgentAway: {
		AgentAvailable: true,
		AgentOffline:   true,
	},
}

// CanTransitionAgent reports whether moving an agent from → to is allowed.
func CanTransitionAgent(from, to AgentState) bool {
	if from == to {
		return false
	}
	next, ok := allowedAgentTransitions[from]
	if !ok {
		return false
	}
	return next[to]
}

// TransitionAgentTo validates and updates the agent state in place.
func (a *Agent) TransitionAgentTo(next AgentState) error {
	if !CanTransitionAgent(a.State, next) {
		return ErrInvalidTransition
	}
	a.State = next
	return nil
}
