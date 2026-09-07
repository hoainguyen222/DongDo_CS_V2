package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/hoainguyen222/DongDo_CS_V2/services/call-service/internal/domain"
	_ "github.com/lib/pq"
	"github.com/rs/zerolog/log"
)

type CallRepository struct {
	db *sql.DB
}

func NewCallRepository(db *sql.DB) *CallRepository {
	return &CallRepository{db: db}
}

func (r *CallRepository) Create(ctx context.Context, call *domain.Call) error {
	query := `
		INSERT INTO calls_v2 (id, session_id, customer_id, agent_id, status, requested_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
	`
	_, err := r.db.ExecContext(ctx, query,
		call.ID, call.SessionID, call.CustomerID, call.AgentID, string(call.Status), call.RequestedAt,
	)
	if err != nil {
		log.Error().Err(err).Str("call_id", call.ID).Msg("Failed to insert call record into Postgres")
		return err
	}
	return nil
}

func (r *CallRepository) GetByID(ctx context.Context, callID string) (*domain.Call, error) {
	query := `
		SELECT id, session_id, customer_id, COALESCE(agent_id, ''), status, requested_at, assigned_at, started_at, ended_at, duration_seconds, created_at, updated_at
		FROM calls_v2 WHERE id = $1
	`
	row := r.db.QueryRowContext(ctx, query, callID)

	var c domain.Call
	var agentID string
	var assignedAt, startedAt, endedAt sql.NullTime

	err := row.Scan(
		&c.ID, &c.SessionID, &c.CustomerID, &agentID, &c.Status,
		&c.RequestedAt, &assignedAt, &startedAt, &endedAt,
		&c.DurationSeconds, &c.CreatedAt, &c.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, domain.ErrCallNotFound
	}
	if err != nil {
		return nil, err
	}

	c.AgentID = agentID
	if assignedAt.Valid {
		c.AssignedAt = &assignedAt.Time
	}
	if startedAt.Valid {
		c.StartedAt = &startedAt.Time
	}
	if endedAt.Valid {
		c.EndedAt = &endedAt.Time
	}

	return &c, nil
}

func (r *CallRepository) UpdateStatus(ctx context.Context, callID string, status domain.CallStatus) error {
	query := `
		UPDATE calls_v2 
		SET status = $1, updated_at = NOW()
		WHERE id = $2
	`
	_, err := r.db.ExecContext(ctx, query, string(status), callID)
	return err
}

func (r *CallRepository) AssignAgent(ctx context.Context, callID, agentID string) error {
	query := `
		UPDATE calls_v2 
		SET agent_id = $1, status = $2, assigned_at = NOW(), updated_at = NOW()
		WHERE id = $3
	`
	_, err := r.db.ExecContext(ctx, query, agentID, string(domain.StatusWaitingAgent), callID)
	return err
}

func (r *CallRepository) SetStarted(ctx context.Context, callID string) error {
	query := `
		UPDATE calls_v2 
		SET status = $1, started_at = NOW(), updated_at = NOW()
		WHERE id = $2
	`
	_, err := r.db.ExecContext(ctx, query, string(domain.StatusInProgress), callID)
	return err
}

func (r *CallRepository) EndCall(ctx context.Context, callID string, durationSec int, finalStatus domain.CallStatus) error {
	query := `
		UPDATE calls_v2 
		SET status = $1, duration_seconds = $2, ended_at = NOW(), updated_at = NOW()
		WHERE id = $3
	`
	_, err := r.db.ExecContext(ctx, query, string(finalStatus), durationSec, callID)
	return err
}

func (r *CallRepository) SaveEvent(ctx context.Context, event *domain.CallEvent) error {
	payloadBytes, _ := json.Marshal(event.Payload)
	query := `
		INSERT INTO call_events (call_id, event_type, source, payload, created_at)
		VALUES ($1, $2, $3, $4, NOW())
	`
	_, err := r.db.ExecContext(ctx, query, event.CallID, event.EventType, event.Source, string(payloadBytes))
	return err
}

func (r *CallRepository) UpsertSession(ctx context.Context, session *domain.CallSession) error {
	query := `
		INSERT INTO call_sessions (call_id, customer_channel_id, agent_channel_id, bridge_id, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
		ON CONFLICT (call_id) DO UPDATE SET
			customer_channel_id = EXCLUDED.customer_channel_id,
			agent_channel_id = EXCLUDED.agent_channel_id,
			bridge_id = EXCLUDED.bridge_id,
			updated_at = NOW()
	`
	_, err := r.db.ExecContext(ctx, query, session.CallID, session.CustomerChannelID, session.AgentChannelID, session.BridgeID)
	return err
}

func (r *CallRepository) SaveRecording(ctx context.Context, recording *domain.CallRecording) error {
	query := `
		INSERT INTO call_recordings (call_id, recording_url, file_size_bytes, transcript, created_at)
		VALUES ($1, $2, $3, $4, NOW())
	`
	_, err := r.db.ExecContext(ctx, query, recording.CallID, recording.RecordingURL, recording.FileSizeBytes, recording.Transcript)
	return err
}

func (r *CallRepository) ListAll(ctx context.Context, limit int) ([]*domain.Call, error) {
	if limit <= 0 {
		limit = 100
	}
	query := `
		SELECT c.id, c.session_id, c.customer_id, COALESCE(c.agent_id, ''), c.status, 
		       c.requested_at, c.assigned_at, c.started_at, c.ended_at, c.duration_seconds, c.created_at, c.updated_at,
		       COALESCE(r.recording_url, ''), COALESCE(r.transcript, '')
		FROM calls_v2 c
		LEFT JOIN call_recordings r ON c.id = r.call_id
		ORDER BY c.created_at DESC
		LIMIT $1
	`
	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("list calls query failed: %w", err)
	}
	defer rows.Close()

	var calls []*domain.Call
	for rows.Next() {
		var c domain.Call
		var agentID, recURL, transcript string
		var assignedAt, startedAt, endedAt sql.NullTime

		err := rows.Scan(
			&c.ID, &c.SessionID, &c.CustomerID, &agentID, &c.Status,
			&c.RequestedAt, &assignedAt, &startedAt, &endedAt,
			&c.DurationSeconds, &c.CreatedAt, &c.UpdatedAt,
			&recURL, &transcript,
		)
		if err != nil {
			return nil, err
		}
		c.AgentID = agentID
		if assignedAt.Valid {
			c.AssignedAt = &assignedAt.Time
		}
		if startedAt.Valid {
			c.StartedAt = &startedAt.Time
		}
		if endedAt.Valid {
			c.EndedAt = &endedAt.Time
		}
		if recURL != "" || transcript != "" {
			c.Recording = &domain.CallRecording{
				CallID:       c.ID,
				RecordingURL: recURL,
				Transcript:   transcript,
			}
		}

		calls = append(calls, &c)
	}

	return calls, nil
}
