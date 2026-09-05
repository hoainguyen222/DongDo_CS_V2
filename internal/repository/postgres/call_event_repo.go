package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// ============================================================
// CallEventRepository — append-only audit log
// ============================================================
//
// Backed by the `call_events` table created in migration 00005. The
// table has a unique index on (call_id, event_type, source) so an
// ARI reconnect cannot insert duplicate rows.
type CallEventRepo struct {
	db *DB
}

// NewCallEventRepo constructs the repository.
func NewCallEventRepo(db *DB) *CallEventRepo {
	return &CallEventRepo{db: db}
}

// Append inserts a new audit row. Idempotent on (call_id, event_type,
// source): duplicate inserts are silently ignored so an ARI reconnect
// or a retry of an HTTP call does not pollute the log.
func (r *CallEventRepo) Append(ctx context.Context, ev *domain.CallEventRecord) error {
	if ev == nil {
		return errors.New("nil call event")
	}
	const q = `
		INSERT INTO call_events (call_id, event_type, source, payload, created_at)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (call_id, event_type, source) DO NOTHING
	`
	_, err := r.db.Pool.Exec(ctx, q,
		ev.CallID,
		string(ev.EventType),
		string(ev.Source),
		ev.Payload,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			// FK violation: call_id doesn't exist. Skip silently — the
			// call was likely deleted while the event was in flight.
			return nil
		}
		return fmt.Errorf("call_events append: %w", err)
	}
	return nil
}

// ListByCall returns all events for a call, ordered by creation time.
func (r *CallEventRepo) ListByCall(ctx context.Context, callID int64) ([]*domain.CallEventRecord, error) {
	const q = `
		SELECT id, call_id, event_type, source, payload, created_at
		FROM call_events
		WHERE call_id = $1
		ORDER BY created_at ASC, id ASC
	`
	rows, err := r.db.Pool.Query(ctx, q, callID)
	if err != nil {
		return nil, fmt.Errorf("call_events list: %w", err)
	}
	defer rows.Close()

	out := make([]*domain.CallEventRecord, 0)
	for rows.Next() {
		var (
			ev         domain.CallEventRecord
			eventType  string
			source     string
			payload    pgtype.Text
		)
		if err := rows.Scan(&ev.ID, &ev.CallID, &eventType, &source, &payload, &ev.CreatedAt); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				break
			}
			return nil, fmt.Errorf("call_events scan: %w", err)
		}
		ev.EventType = domain.CallEventType(eventType)
		ev.Source = domain.CallEventSource(source)
		if payload.Valid {
			ev.Payload = payload.String
		}
		out = append(out, &ev)
	}
	return out, nil
}
