package domain_test

import (
	"testing"

	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
)

// TestCallStateMachine_HappyPath walks the entire lifecycle.
func TestCallStateMachine_HappyPath(t *testing.T) {
	c := &domain.Call{Status: domain.CallStatusCreated}
	steps := []domain.CallStatus{
		domain.CallStatusWaiting,
		domain.CallStatusWaitingAgent,
		domain.CallStatusConnecting,
		domain.CallStatusRinging,
		domain.CallStatusInProgress,
		domain.CallStatusEnded,
	}
	for _, next := range steps {
		if err := c.TransitionTo(next); err != nil {
			t.Fatalf("transition %s→%s failed: %v", c.Status, next, err)
		}
		if c.Status != next {
			t.Fatalf("status not updated: got %s want %s", c.Status, next)
		}
	}
	if !c.Status.IsTerminal() {
		t.Fatalf("ENDED must be terminal; got %s", c.Status)
	}
}

// TestCallStateMachine_InvalidTransitions covers the rules from the spec.
func TestCallStateMachine_InvalidTransitions(t *testing.T) {
	cases := []struct {
		from, to domain.CallStatus
	}{
		{domain.CallStatusWaiting, domain.CallStatusInProgress},   // skip steps
		{domain.CallStatusCreated, domain.CallStatusRinging},       // skip steps
		{domain.CallStatusInProgress, domain.CallStatusWaiting},    // back to queue
		{domain.CallStatusEnded, domain.CallStatusInProgress},      // terminal → active
		{domain.CallStatusCancelled, domain.CallStatusWaiting},     // terminal → active
		{domain.CallStatusRinging, domain.CallStatusWaiting},       // back to queue
		{domain.CallStatusConnecting, domain.CallStatusWaiting},    // back to queue
	}
	for _, tc := range cases {
		c := &domain.Call{Status: tc.from}
		if err := c.TransitionTo(tc.to); err == nil {
			t.Errorf("expected error for %s→%s, got nil", tc.from, tc.to)
		}
	}
}

// TestCallStateMachine_TerminalHasNoOutgoing ensures terminal states are sticky.
func TestCallStateMachine_TerminalHasNoOutgoing(t *testing.T) {
	terminals := []domain.CallStatus{
		domain.CallStatusEnded, domain.CallStatusRejected, domain.CallStatusCancelled,
		domain.CallStatusMissed, domain.CallStatusFailed, domain.CallStatusTimeout,
	}
	for _, term := range terminals {
		c := &domain.Call{Status: term}
		for _, next := range []domain.CallStatus{
			domain.CallStatusWaiting, domain.CallStatusInProgress, domain.CallStatusRinging,
		} {
			if err := c.TransitionTo(next); err == nil {
				t.Errorf("terminal %s should not transition to %s", term, next)
			}
		}
	}
}

// TestAgentStateMachine covers the canonical agent transitions.
func TestAgentStateMachine(t *testing.T) {
	cases := []struct {
		from, to domain.AgentState
		ok       bool
	}{
		{domain.AgentOffline, domain.AgentAvailable, true},
		{domain.AgentAvailable, domain.AgentReserved, true},
		{domain.AgentReserved, domain.AgentRinging, true},
		{domain.AgentRinging, domain.AgentBusy, true},
		{domain.AgentBusy, domain.AgentAvailable, true},

		// rejects / timeouts
		{domain.AgentReserved, domain.AgentAvailable, true},
		{domain.AgentRinging, domain.AgentAvailable, true},

		// away/offline
		{domain.AgentAvailable, domain.AgentAway, true},
		{domain.AgentAway, domain.AgentAvailable, true},
		{domain.AgentBusy, domain.AgentOffline, true},

		// invalid
		{domain.AgentOffline, domain.AgentBusy, false},
		{domain.AgentAvailable, domain.AgentBusy, false}, // must go through RESERVED/RINGING
		{domain.AgentBusy, domain.AgentReserved, false},
		{domain.AgentReserved, domain.AgentBusy, false}, // must go through RINGING
	}
	for _, tc := range cases {
		a := &domain.Agent{State: tc.from}
		err := a.TransitionAgentTo(tc.to)
		got := err == nil
		if got != tc.ok {
			t.Errorf("%s→%s: got ok=%v err=%v, want ok=%v", tc.from, tc.to, got, err, tc.ok)
		}
	}
}

// TestCallStatus_IsTerminal sanity-checks the terminal predicate.
func TestCallStatus_IsTerminal(t *testing.T) {
	terminals := []domain.CallStatus{
		domain.CallStatusEnded, domain.CallStatusRejected, domain.CallStatusCancelled,
		domain.CallStatusMissed, domain.CallStatusFailed, domain.CallStatusTimeout,
	}
	for _, s := range terminals {
		if !s.IsTerminal() {
			t.Errorf("%s must be terminal", s)
		}
	}
	nonTerminals := []domain.CallStatus{
		domain.CallStatusCreated, domain.CallStatusWaiting, domain.CallStatusWaitingAgent,
		domain.CallStatusConnecting, domain.CallStatusRinging, domain.CallStatusInProgress,
	}
	for _, s := range nonTerminals {
		if s.IsTerminal() {
			t.Errorf("%s must NOT be terminal", s)
		}
	}
}
