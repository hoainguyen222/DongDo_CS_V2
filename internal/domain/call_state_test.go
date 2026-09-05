package domain

import "testing"

// ============================================================
// CallStateMachine
// ============================================================

func TestCallStateMachine_HappyPath(t *testing.T) {
	sm := CallStateMachine{}
	chain := []CallStatus{
		CallCreated, CallWaiting, CallWaitingAgent, CallConnecting, CallInProgress, CallEnded,
	}
	for i := 0; i < len(chain)-1; i++ {
		if err := sm.TransitionTo(chain[i], chain[i+1]); err != nil {
			t.Fatalf("expected %s→%s to be legal, got %v", chain[i], chain[i+1], err)
		}
	}
}

func TestCallStateMachine_IllegalTransitions(t *testing.T) {
	sm := CallStateMachine{}
	cases := []struct {
		from, to CallStatus
	}{
		{CallWaiting, CallInProgress}, // must go through WAITING_AGENT → CONNECTING
		{CallInProgress, CallWaiting},
		{CallEnded, CallInProgress}, // terminal must stay terminal
		{CallRejected, CallInProgress},
		{CallCreated, CallInProgress},
	}
	for _, c := range cases {
		if err := sm.TransitionTo(c.from, c.to); err == nil {
			t.Errorf("expected %s→%s to be illegal, got nil", c.from, c.to)
		}
	}
}

func TestCallStateMachine_LegacyAliases(t *testing.T) {
	sm := CallStateMachine{}
	// Legacy RINGING (canonical WAITING_AGENT) → legacy BRIDGED (canonical
	// IN_PROGRESS) is illegal — must go via CONNECTING first.
	if err := sm.TransitionTo(CallRinging, CallBridged); err == nil {
		t.Errorf("legacy RINGING→BRIDGED should be illegal (must go via CONNECTING)")
	}
	// Legacy ACTIVE → ENDED is legal.
	if err := sm.TransitionTo(CallActive, CallEnded); err != nil {
		t.Fatalf("legacy ACTIVE→ENDED should be legal, got %v", err)
	}
	// Legacy CONNECTING (DIALING) → IN_PROGRESS (BRIDGED) is legal.
	if err := sm.TransitionTo(CallDialing, CallBridged); err != nil {
		t.Fatalf("legacy DIALING→BRIDGED should be legal, got %v", err)
	}
}

func TestCallStatus_Canonicalization(t *testing.T) {
	cases := map[CallStatus]CallStatus{
		CallRinging:  CallWaitingAgent,
		CallDialing:  CallConnecting,
		CallBridged:  CallInProgress,
		CallActive:   CallInProgress,
		CallWaiting:  CallWaiting,
		CallEnded:    CallEnded,
	}
	for input, want := range cases {
		if got := input.Canonical(); got != want {
			t.Errorf("Canonical(%s)=%s, want %s", input, got, want)
		}
	}
}

func TestCallStatus_IsTerminal(t *testing.T) {
	terminals := []CallStatus{CallEnded, CallFailed, CallMissed, CallRejected, CallCancelled, CallTimeout}
	for _, s := range terminals {
		if !s.IsTerminal() {
			t.Errorf("%s should be terminal", s)
		}
	}
	nonTerminals := []CallStatus{CallWaiting, CallWaitingAgent, CallConnecting, CallInProgress}
	for _, s := range nonTerminals {
		if s.IsTerminal() {
			t.Errorf("%s should NOT be terminal", s)
		}
	}
}

// ============================================================
// AgentStateMachine
// ============================================================

func TestAgentStateMachine_HappyPath(t *testing.T) {
	sm := AgentStateMachine{}
	chain := []AgentStatus{
		AgentOffline, AgentAvailable, AgentReserved, AgentRinging, AgentBusy, AgentAvailable,
	}
	for i := 0; i < len(chain)-1; i++ {
		if err := sm.TransitionTo(chain[i], chain[i+1]); err != nil {
			t.Fatalf("expected %s→%s to be legal, got %v", chain[i], chain[i+1], err)
		}
	}
}

func TestAgentStateMachine_RejectionRecovery(t *testing.T) {
	sm := AgentStateMachine{}
	// Agent rejects: RESERVED → AVAILABLE
	if err := sm.TransitionTo(AgentReserved, AgentAvailable); err != nil {
		t.Errorf("RESERVED→AVAILABLE should be legal, got %v", err)
	}
	// Agent goes offline mid-call: BUSY → OFFLINE
	if err := sm.TransitionTo(AgentBusy, AgentOffline); err != nil {
		t.Errorf("BUSY→OFFLINE should be legal, got %v", err)
	}
}

func TestAgentStateMachine_IllegalTransitions(t *testing.T) {
	sm := AgentStateMachine{}
	cases := []struct {
		from, to AgentStatus
	}{
		{AgentOffline, AgentBusy},     // must pass through AVAILABLE
		{AgentAvailable, AgentBusy},   // BUSY requires RINGING → ANSWERED; direct AVAILABLE→BUSY is not allowed
		{AgentBusy, AgentReserved},    // cannot go back to RESERVED without passing through AVAILABLE
		{AgentReserved, AgentBusy},    // must go via RINGING
	}
	for _, c := range cases {
		if err := sm.TransitionTo(c.from, c.to); err == nil {
			t.Errorf("expected %s→%s to be illegal, got nil", c.from, c.to)
		}
	}
}
