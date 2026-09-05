package domain

import (
	"context"
	"time"
)

// ============================================================
// Call Event (Audit Log)
// ============================================================
//
// CallEvent is an append-only audit entry for a voice call. It is
// written by:
//   - the HTTP layer when a user-driven action happens (source=API)
//   - the Asterisk ARI event pump when a media event happens (source=ARI)
//   - the use case for system-driven transitions (source=SYSTEM)
//
// Call events are append-only; updates and deletes are forbidden. A
// unique index on (call_id, event_type, source) prevents the ARI
// event stream from producing duplicate rows after a reconnect.
//
// The CallEventType enum is defined in asterisk.go (it shares the
// same string space as the AMI event types).

type CallEventSource string

const (
	CallEventSourceAPI    CallEventSource = "API"
	CallEventSourceARI    CallEventSource = "ARI"
	CallEventSourceSystem CallEventSource = "SYSTEM"
)

// CallEventRecord is the persisted audit row.
type CallEventRecord struct {
	ID        int64           `json:"id"`
	CallID    int64           `json:"call_id"`
	EventType CallEventType   `json:"event_type"`
	Source    CallEventSource `json:"source"`
	Payload   string          `json:"payload"` // JSON-encoded
	CreatedAt time.Time       `json:"created_at"`
}

// CallEventRepository persists CallEventRecords. Implementations must
// guarantee idempotency on (call_id, event_type, source) so that an
// ARI reconnect does not produce duplicates.
type CallEventRepository interface {
	Append(ctx context.Context, ev *CallEventRecord) error
	ListByCall(ctx context.Context, callID int64) ([]*CallEventRecord, error)
}
