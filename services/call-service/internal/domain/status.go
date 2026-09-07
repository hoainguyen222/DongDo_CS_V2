package domain

import "errors"

type CallStatus string

const (
	StatusCreated      CallStatus = "CREATED"
	StatusWaiting      CallStatus = "WAITING"
	StatusWaitingAgent CallStatus = "WAITING_AGENT"
	StatusConnecting   CallStatus = "CONNECTING"
	StatusRinging      CallStatus = "RINGING"
	StatusInProgress   CallStatus = "IN_PROGRESS"
	StatusTerminating  CallStatus = "TERMINATING"
	StatusEnded        CallStatus = "ENDED"
	StatusCancelled    CallStatus = "CANCELLED"
	StatusFailed       CallStatus = "FAILED"
	StatusTimeout      CallStatus = "TIMEOUT"
	StatusMissed       CallStatus = "MISSED"
)

type AgentStatus string

const (
	AgentOffline   AgentStatus = "OFFLINE"
	AgentAvailable AgentStatus = "AVAILABLE"
	AgentReserved  AgentStatus = "RESERVED"
	AgentRinging   AgentStatus = "RINGING"
	AgentBusy      AgentStatus = "BUSY"
	AgentAway      AgentStatus = "AWAY"
)

var (
	ErrInvalidStateTransition = errors.New("invalid call state transition")
	ErrCallNotFound           = errors.New("call not found")
	ErrAgentNotAvailable      = errors.New("agent is not available")
)

// ValidateTransition checks strict State Machine transition rules
func ValidateCallTransition(current, next CallStatus) error {
	if current == next {
		return nil
	}

	validTransitions := map[CallStatus][]CallStatus{
		StatusCreated:      {StatusWaiting, StatusCancelled, StatusFailed},
		StatusWaiting:      {StatusWaitingAgent, StatusCancelled, StatusTimeout, StatusFailed},
		StatusWaitingAgent: {StatusConnecting, StatusWaiting, StatusTimeout, StatusCancelled, StatusFailed},
		StatusConnecting:   {StatusRinging, StatusTerminating, StatusFailed, StatusCancelled},
		StatusRinging:      {StatusInProgress, StatusTerminating, StatusFailed, StatusMissed},
		StatusInProgress:   {StatusTerminating, StatusFailed},
		StatusTerminating:  {StatusEnded, StatusFailed},
		// Terminal states cannot transition further
		StatusEnded:     {},
		StatusCancelled: {},
		StatusFailed:    {},
		StatusTimeout:   {},
		StatusMissed:    {},
	}

	allowed, exists := validTransitions[current]
	if !exists {
		return ErrInvalidStateTransition
	}

	for _, state := range allowed {
		if state == next {
			return nil
		}
	}

	return ErrInvalidStateTransition
}
