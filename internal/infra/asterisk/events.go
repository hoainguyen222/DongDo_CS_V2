package asterisk

import (
	"strings"
	"time"

	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
)

// parseEvent converts a raw AMI event map into a typed domain.CallEvent.
//
// AMI events come in several flavors. Asterisk emits many more event types
// than we care about (e.g. Newexten, VarSet, PeerStatus); we map the
// call-control relevant subset onto domain.CallEventType and ignore the
// rest by leaving Type = CallEventOther so they still flow through the
// event channel but don't trigger state transitions.
func parseEvent(raw map[string]string) domain.CallEvent {
	if raw == nil {
		return domain.CallEvent{Type: domain.CallEventOther, ReceivedAt: time.Now()}
	}

	ev := domain.CallEvent{
		Type:        mapEventType(raw["Event"]),
		Channel:     raw["Channel"],
		ChannelID:   raw["Channel"],
		UniqueID:    raw["Uniqueid"],
		LinkedID:    raw["Linkedid"],
		BridgeID:    raw["BridgeUniqueid"],
		CallerID:    raw["CallerID"],
		CallerIDNum: raw["CallerIDNum"],
		Exten:       raw["Exten"],
		Context:     raw["Context"],
		DestChannel: raw["DestChannel"],
		DestUniqueID: raw["DestUniqueid"],
		ConnectedLineNum:  raw["ConnectedLineNum"],
		ConnectedLineName: raw["ConnectedLineName"],
		State:       raw["ChannelState"],
		Cause:       raw["Cause"],
		CauseTxt:    raw["Cause-txt"],
		DialStatus:  raw["DialStatus"],
		ReceivedAt:  time.Now(),
		Raw:         raw,
	}
	if ev.Type == domain.CallEventDial {
		ev.DestChannel = raw["DestChannel"]
		ev.DestUniqueID = raw["DestUniqueid"]
		ev.DialStatus = raw["DialStatus"]
	}
	if ev.Type == domain.CallEventBridgeEnter || ev.Type == domain.CallEventBridgeLeave {
		ev.BridgeID = raw["BridgeUniqueid"]
		if v, ok := raw["BridgeNumChannels"]; ok {
			// Numeric hint kept in Raw only - not part of domain.CallEvent.
			ev.Raw["BridgeNumChannels"] = v
		}
	}
	return ev
}

// mapEventType maps an AMI Event: string onto our typed enum.
func mapEventType(name string) domain.CallEventType {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "newchannel":
		return domain.CallEventNewchannel
	case "dialbegin":
		return domain.CallEventDialBegin
	case "dialend":
		return domain.CallEventDialEnd
	case "dial":
		return domain.CallEventDial
	case "bridgeenter":
		return domain.CallEventBridgeEnter
	case "bridgeleave":
		return domain.CallEventBridgeLeave
	case "hold":
		return domain.CallEventHold
	case "unhold":
		return domain.CallEventUnhold
	case "transfer":
		return domain.CallEventTransfer
	case "hangup":
		return domain.CallEventHangup
	default:
		return domain.CallEventOther
	}
}

// ============================================================
// Higher-level event projections
// ============================================================
//
// VoiceUseCase consumes these to decide when a call has reached "Bridged",
// "Failed" or "Ended". Keeping the projection logic here keeps the usecase
// clean and testable.

// Projection describes what the voice flow should do in response to an
// event. Returning a zero Projection means "no state change, ignore".
type Projection struct {
	NewStatus    domain.CallStatus // status to write into the DB
	Connected    bool              // true if both parties are now bridged
	Failure      bool              // true if the call is permanently failed
	Hangup       bool              // true if a party hung up
	EndCall      bool              // true if the call should be marked ENDED
	DurationHint int               // suggested duration if we have it
	Reason       string            // human-readable reason
}

// ProjectEvent returns the state change implied by an AMI event.
func ProjectEvent(ev domain.CallEvent) Projection {
	switch ev.Type {
	case domain.CallEventNewchannel:
		// Channel is created but not bridged yet. Caller may want to
		// transition RINGING → DIALING.
		return Projection{NewStatus: domain.CallDialing}

	case domain.CallEventDial, domain.CallEventDialBegin:
		// Dial started between two channels.
		return Projection{NewStatus: domain.CallDialing}

	case domain.CallEventDialEnd:
		// One leg finished dialing. Inspect DialStatus to detect failure.
		switch strings.ToLower(ev.DialStatus) {
		case "answer":
			return Projection{NewStatus: domain.CallDialing, Connected: true}
		case "busy":
			return Projection{NewStatus: domain.CallFailed, Failure: true, Reason: "busy"}
		case "congestion":
			return Projection{NewStatus: domain.CallFailed, Failure: true, Reason: "congestion"}
		case "cancel":
			return Projection{NewStatus: domain.CallFailed, Failure: true, Reason: "cancelled"}
		case "noanswer":
			return Projection{NewStatus: domain.CallMissed, Failure: true, Reason: "no-answer"}
		case "chanunavail":
			return Projection{NewStatus: domain.CallFailed, Failure: true, Reason: "channel-unavailable"}
		default:
			return Projection{NewStatus: domain.CallDialing}
		}

	case domain.CallEventBridgeEnter:
		// A channel entered a bridge - first BridgeEnter means the call
		// has both parties connected.
		return Projection{NewStatus: domain.CallBridged, Connected: true}

	case domain.CallEventBridgeLeave:
		// A channel left a bridge; if this was the last one, the call
		// is about to end. The Hangup event confirms the final state.
		return Projection{NewStatus: domain.CallEnded, Hangup: true, EndCall: true}

	case domain.CallEventHangup:
		// Channel hung up. Asterisk sends Cause and Cause-txt; we capture
		// them for logs.
		reason := ev.CauseTxt
		if reason == "" {
			reason = ev.Cause
		}
		if reason == "" {
			reason = "hangup"
		}
		return Projection{NewStatus: domain.CallEnded, Hangup: true, EndCall: true, Reason: reason}

	case domain.CallEventTransfer:
		return Projection{NewStatus: domain.CallBridged, Connected: true}

	default:
		return Projection{}
	}
}
