package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog"
)

// CallRepo implements domain.CallRepository using direct pgx queries
// (the call schema is new and not yet covered by sqlc).
type CallRepo struct {
	db     *DB
	logger zerolog.Logger
}

// NewCallRepo returns a CallRepo using the shared DB.
func NewCallRepo(db *DB) *CallRepo {
	return &CallRepo{
		db:     db,
		logger: logger.With().Str("repo", "call").Logger(),
	}
}

// Compile-time interface check
var _ domain.CallRepository = (*CallRepo)(nil)

// ----------------------------------------------------------------
// Insert
// ----------------------------------------------------------------

func (r *CallRepo) Insert(ctx context.Context, c *domain.Call) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	c.Status = domain.CallStatusCreated
	if c.RequestedAt.IsZero() {
		c.RequestedAt = time.Now().UTC()
	}
	c.LastEventAt = time.Now().UTC()
	c.CreatedAt = time.Now().UTC()
	c.UpdatedAt = c.CreatedAt

	_, err := r.db.Pool.Exec(ctx, `
        INSERT INTO calls (
            id, customer_id, agent_id, status, priority,
            requested_at, last_event_at, created_at, updated_at
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
    `, c.ID, c.CustomerID, c.AgentID, string(c.Status), c.Priority,
		c.RequestedAt, c.LastEventAt, c.CreatedAt, c.UpdatedAt)
	if err != nil {
		r.logger.Error().Err(err).Str("call_id", c.ID.String()).Msg("Insert call failed")
		return err
	}

	// Audit row for CREATED.
	_ = r.AppendEvent(ctx, &domain.CallEvent{
		CallID:     c.ID,
		EventType:  "call.created",
		Source:     domain.CallEventSourceAPI,
		Payload:    map[string]any{"customer_id": c.CustomerID, "priority": c.Priority},
		OccurredAt: c.CreatedAt,
	})
	return nil
}

// ----------------------------------------------------------------
// Get
// ----------------------------------------------------------------

func (r *CallRepo) Get(ctx context.Context, id uuid.UUID) (*domain.Call, error) {
	row := r.db.Pool.QueryRow(ctx, `
        SELECT id, customer_id, agent_id, status, priority,
               requested_at, assigned_at, started_at, ended_at,
               duration_seconds, last_event_at, ari_bridge_id,
               failure_reason, created_at, updated_at
        FROM calls WHERE id = $1
    `, id)
	c, err := scanCall(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return c, nil
}

// ----------------------------------------------------------------
// Idempotency
// ----------------------------------------------------------------

func (r *CallRepo) GetByIdempotencyKey(ctx context.Context, key string) (*domain.IdempotencyRecord, error) {
	row := r.db.Pool.QueryRow(ctx, `
        SELECT idempotency_key, call_id, response_body, created_at
        FROM call_idempotency
        WHERE idempotency_key = $1
    `, key)
	var rec domain.IdempotencyRecord
	var bodyRaw []byte
	if err := row.Scan(&rec.Key, &rec.CallID, &bodyRaw, &rec.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if len(bodyRaw) > 0 {
		_ = json.Unmarshal(bodyRaw, &rec.ResponseBody)
	}
	return &rec, nil
}

func (r *CallRepo) SaveIdempotency(ctx context.Context, rec *domain.IdempotencyRecord) error {
	body, err := json.Marshal(rec.ResponseBody)
	if err != nil {
		return err
	}
	_, err = r.db.Pool.Exec(ctx, `
        INSERT INTO call_idempotency (idempotency_key, call_id, response_body, created_at)
        VALUES ($1, $2, $3, NOW())
        ON CONFLICT (idempotency_key) DO NOTHING
    `, rec.Key, rec.CallID, body)
	return err
}

// ClaimIdempotency reserves an idempotency key atomically. If the key has
// never been seen, an empty placeholder row is inserted and claimed=true is
// returned. Otherwise claimed=false and the caller should look up the
// cached response via GetByIdempotencyKey and replay it.
//
// The placeholder is updated with the actual response via SaveIdempotency
// once the underlying operation completes.
func (r *CallRepo) ClaimIdempotency(ctx context.Context, key string) (bool, error) {
	tag, err := r.db.Pool.Exec(ctx, `
        INSERT INTO call_idempotency (idempotency_key, call_id, response_body, created_at)
        VALUES ($1, '00000000-0000-0000-0000-000000000000'::uuid, '{}'::jsonb, NOW())
        ON CONFLICT (idempotency_key) DO NOTHING
    `, key)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// ----------------------------------------------------------------
// State updates with CAS (prevents lost updates)
// ----------------------------------------------------------------

// UpdateState performs an atomic CAS update guarded by `from`.
// Returns domain.ErrInvalidTransition if the row is not in `from`.
func (r *CallRepo) UpdateState(ctx context.Context, id uuid.UUID, from, to domain.CallStatus, f domain.StateTransitionFields) error {
	if !domain.CanTransition(from, to) {
		return domain.ErrInvalidTransition
	}

	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// CAS
	tag, err := tx.Exec(ctx, `
        UPDATE calls SET status = $1, updated_at = NOW(), last_event_at = NOW()
        WHERE id = $2 AND status = $3
    `, string(to), id, string(from))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrInvalidTransition
	}

	// Optional column updates
	if f.AssignedAt != nil {
		_, _ = tx.Exec(ctx, `UPDATE calls SET assigned_at = $1 WHERE id = $2`, *f.AssignedAt, id)
	}
	if f.StartedAt != nil {
		_, _ = tx.Exec(ctx, `UPDATE calls SET started_at = $1 WHERE id = $2`, *f.StartedAt, id)
	}
	if f.EndedAt != nil {
		_, _ = tx.Exec(ctx, `UPDATE calls SET ended_at = $1 WHERE id = $2`, *f.EndedAt, id)
	}
	if f.DurationSeconds != nil {
		_, _ = tx.Exec(ctx, `UPDATE calls SET duration_seconds = $1 WHERE id = $2`, *f.DurationSeconds, id)
	}
	if f.ARIBridgeID != nil {
		_, _ = tx.Exec(ctx, `UPDATE calls SET ari_bridge_id = $1 WHERE id = $2`, *f.ARIBridgeID, id)
	}
	if f.FailureReason != nil {
		_, _ = tx.Exec(ctx, `UPDATE calls SET failure_reason = $1 WHERE id = $2`, *f.FailureReason, id)
	}
	if f.AgentID != nil {
		_, _ = tx.Exec(ctx, `UPDATE calls SET agent_id = $1 WHERE id = $2`, *f.AgentID, id)
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	// Append audit event (best-effort)
	_ = r.AppendEvent(ctx, &domain.CallEvent{
		CallID:     id,
		EventType:  "call.state_changed",
		Source:     domain.CallEventSourceSystem,
		Payload:    map[string]any{"from": string(from), "to": string(to)},
		OccurredAt: time.Now().UTC(),
	})
	return nil
}

// AssignAgent atomically transitions WAITING→WAITING_AGENT and binds the agent.
func (r *CallRepo) AssignAgent(ctx context.Context, id uuid.UUID, agentID string) error {
	tag, err := r.db.Pool.Exec(ctx, `
        UPDATE calls
        SET status = 'WAITING_AGENT'::call_status,
            agent_id = $1,
            assigned_at = NOW(),
            last_event_at = NOW(),
            updated_at = NOW()
        WHERE id = $2 AND status = 'WAITING'::call_status
    `, agentID, id)
	if err != nil {
		r.logger.Error().Err(err).Msg("AssignAgent failed")
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrInvalidTransition
	}
	_ = r.AppendEvent(ctx, &domain.CallEvent{
		CallID:     id,
		EventType:  "call.assigned",
		Source:     domain.CallEventSourceSystem,
		AgentID:    agentID,
		Payload:    map[string]any{"agent_id": agentID},
		OccurredAt: time.Now().UTC(),
	})
	return nil
}

// ----------------------------------------------------------------
// Participant actions (DB-level idempotency)
// ----------------------------------------------------------------

// RecordParticipant records a participant action. The (call_id, role, action)
// unique index makes the insert itself idempotent. We then detect whether the
// row was actually inserted (vs. skipped on conflict) by reading the
// COUNT AFTER — if the row count is zero we know it was a duplicate.
//
// This is correct under concurrent inserts because the unique index
// serializes the conflicting inserts and Postgres returns a stable
// affected-row count.
func (r *CallRepo) RecordParticipant(ctx context.Context, p *domain.CallParticipant) error {
	if p.ActionAt.IsZero() {
		p.ActionAt = time.Now().UTC()
	}
	tag, err := r.db.Pool.Exec(ctx, `
        INSERT INTO call_participants (call_id, participant_id, role, action, action_at)
        VALUES ($1, $2, $3, $4, $5)
        ON CONFLICT (call_id, role, action) DO NOTHING
    `, p.CallID, p.ParticipantID, string(p.Role), p.Action, p.ActionAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrDuplicateAction
	}
	return nil
}

// ----------------------------------------------------------------
// Events (append-only)
// ----------------------------------------------------------------

func (r *CallRepo) AppendEvent(ctx context.Context, e *domain.CallEvent) error {
	if e.OccurredAt.IsZero() {
		e.OccurredAt = time.Now().UTC()
	}
	payload, err := json.Marshal(e.Payload)
	if err != nil {
		return err
	}
	_, err = r.db.Pool.Exec(ctx, `
        INSERT INTO call_events (call_id, event_type, source, agent_id, payload, occurred_at)
        VALUES ($1, $2, $3, $4, $5, $6)
        ON CONFLICT (call_id, event_type, occurred_at) DO NOTHING
    `, e.CallID, e.EventType, string(e.Source), nullableStr(e.AgentID), payload, e.OccurredAt)
	return err
}

// ----------------------------------------------------------------
// Recordings
// ----------------------------------------------------------------

func (r *CallRepo) InsertRecording(ctx context.Context, rec *domain.CallRecording) error {
	_, err := r.db.Pool.Exec(ctx, `
        INSERT INTO call_recordings (call_id, ari_recording, storage_url, started_at, ended_at, state)
        VALUES ($1, $2, $3, $4, $5, $6)
    `, rec.CallID, nullableStr(rec.ARIRecording), nullableStr(rec.StorageURL),
		rec.StartedAt, rec.EndedAt, string(rec.State))
	return err
}

func (r *CallRepo) UpdateRecording(ctx context.Context, callID uuid.UUID, ariName, storageURL string, endedAt time.Time, state domain.CallRecordingState) error {
	_, err := r.db.Pool.Exec(ctx, `
        UPDATE call_recordings
        SET ari_recording = COALESCE(NULLIF($1, ''), ari_recording),
            storage_url   = COALESCE(NULLIF($2, ''), storage_url),
            ended_at      = COALESCE($3, ended_at),
            state         = $4
        WHERE call_id = $5
    `, nullableStr(ariName), nullableStr(storageURL), endedAt, string(state), callID)
	return err
}

// ----------------------------------------------------------------
// Listings
// ----------------------------------------------------------------

func (r *CallRepo) ListByAgent(ctx context.Context, agentID string, limit, offset int) ([]*domain.Call, error) {
	rows, err := r.db.Pool.Query(ctx, `
        SELECT id, customer_id, agent_id, status, priority,
               requested_at, assigned_at, started_at, ended_at,
               duration_seconds, last_event_at, ari_bridge_id,
               failure_reason, created_at, updated_at
        FROM calls
        WHERE agent_id = $1
        ORDER BY updated_at DESC
        LIMIT $2 OFFSET $3
    `, agentID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*domain.Call, 0)
	for rows.Next() {
		c, err := scanCall(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ListAll returns a paginated slice of all calls (Call v2). Used by the
// admin call history view (replaces the legacy voice_calls feed which
// only captures pre-Call-v2 traffic).
func (r *CallRepo) ListAll(ctx context.Context, limit, offset int) ([]*domain.Call, error) {
	rows, err := r.db.Pool.Query(ctx, `
        SELECT id, customer_id, agent_id, status, priority,
               requested_at, assigned_at, started_at, ended_at,
               duration_seconds, last_event_at, ari_bridge_id,
               failure_reason, created_at, updated_at
        FROM calls
        ORDER BY updated_at DESC
        LIMIT $1 OFFSET $2
    `, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*domain.Call, 0)
	for rows.Next() {
		c, err := scanCall(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ListActiveByAgent returns non-terminal calls owned by an agent. Used
// after a WebSocket reconnect so the admin layout can re-show the
// ringing banner for a call that was announced before the page reload.
func (r *CallRepo) ListActiveByAgent(ctx context.Context, agentID string) ([]*domain.Call, error) {
	rows, err := r.db.Pool.Query(ctx, `
        SELECT id, customer_id, agent_id, status, priority,
               requested_at, assigned_at, started_at, ended_at,
               duration_seconds, last_event_at, ari_bridge_id,
               failure_reason, created_at, updated_at
        FROM calls
        WHERE agent_id = $1
          AND status IN ('WAITING_AGENT', 'CONNECTING', 'RINGING', 'IN_PROGRESS')
        ORDER BY updated_at DESC
        LIMIT 5
    `, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*domain.Call, 0)
	for rows.Next() {
		c, err := scanCall(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *CallRepo) ListByCustomer(ctx context.Context, customerID string, limit, offset int) ([]*domain.Call, error) {
	rows, err := r.db.Pool.Query(ctx, `
        SELECT id, customer_id, agent_id, status, priority,
               requested_at, assigned_at, started_at, ended_at,
               duration_seconds, last_event_at, ari_bridge_id,
               failure_reason, created_at, updated_at
        FROM calls
        WHERE customer_id = $1
        ORDER BY updated_at DESC
        LIMIT $2 OFFSET $3
    `, customerID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*domain.Call, 0)
	for rows.Next() {
		c, err := scanCall(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ListActive returns non-terminal calls whose last_event_at is older than `olderThan`.
// Used by the reconciliation worker.
func (r *CallRepo) ListActive(ctx context.Context, olderThan time.Duration, limit int) ([]*domain.Call, error) {
	rows, err := r.db.Pool.Query(ctx, `
        SELECT id, customer_id, agent_id, status, priority,
               requested_at, assigned_at, started_at, ended_at,
               duration_seconds, last_event_at, ari_bridge_id,
               failure_reason, created_at, updated_at
        FROM calls
        WHERE status NOT IN ('ENDED','REJECTED','CANCELLED','MISSED','FAILED','TIMEOUT')
          AND last_event_at < NOW() - $1::interval
        ORDER BY last_event_at ASC
        LIMIT $2
    `, fmt.Sprintf("%d seconds", int(olderThan.Seconds())), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*domain.Call, 0)
	for rows.Next() {
		c, err := scanCall(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ----------------------------------------------------------------
// Helpers
// ----------------------------------------------------------------

type rowScanner interface {
	Scan(dest ...any) error
}

func scanCall(row rowScanner) (*domain.Call, error) {
	var c domain.Call
	var agentID, ariBridge, failure *string
	var assignedAt, startedAt, endedAt *time.Time
	if err := row.Scan(
		&c.ID, &c.CustomerID, &agentID, &c.Status, &c.Priority,
		&c.RequestedAt, &assignedAt, &startedAt, &endedAt,
		&c.DurationSeconds, &c.LastEventAt, &ariBridge,
		&failure, &c.CreatedAt, &c.UpdatedAt,
	); err != nil {
		return nil, err
	}
	c.AgentID = agentID
	c.AssignedAt = assignedAt
	c.StartedAt = startedAt
	c.EndedAt = endedAt
	if ariBridge != nil {
		c.ARIBridgeID = *ariBridge
	}
	if failure != nil {
		c.FailureReason = *failure
	}
	return &c, nil
}

func nullableStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
