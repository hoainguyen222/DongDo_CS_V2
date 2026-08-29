package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
)

// ============================================================
// Setting Repository
// ============================================================

type SettingRepo struct {
	db *DB
}

func NewSettingRepo(db *DB) *SettingRepo {
	return &SettingRepo{db: db}
}

func (r *SettingRepo) Get(ctx context.Context, key, fallback string) (string, error) {
	row := r.db.Pool.QueryRow(ctx, `
		SELECT setting_value FROM system_settings WHERE setting_key = $1
	`, key)

	var val string
	err := row.Scan(&val)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fallback, nil
		}
		return fallback, err
	}
	return val, nil
}

func (r *SettingRepo) Set(ctx context.Context, key, value string) error {
	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO system_settings (setting_key, setting_value)
		VALUES ($1, $2)
		ON CONFLICT (setting_key) DO UPDATE SET setting_value = EXCLUDED.setting_value
	`, key, value)
	return err
}

// ============================================================
// Voice Call Repository
// ============================================================

type VoiceCallRepo struct {
	db *DB
}

func NewVoiceCallRepo(db *DB) *VoiceCallRepo {
	return &VoiceCallRepo{db: db}
}

func (r *VoiceCallRepo) Create(ctx context.Context, call *domain.VoiceCall) (*domain.VoiceCall, error) {
	row := r.db.Pool.QueryRow(ctx, `
		INSERT INTO voice_calls (session_id, caller_type, caller_id, callee_type, callee_id, status, created_at)
		VALUES ($1, $2::caller_type, $3, $4::caller_type, $5, 'RINGING'::call_status, NOW())
		RETURNING id, session_id, caller_type, caller_id, callee_type, callee_id, status, duration_seconds, recording_url, transcript, created_at, ended_at
	`, call.SessionID, string(call.CallerType), call.CallerID, string(call.CalleeType), call.CalleeID)

	var vc domain.VoiceCall
	var cType, ceType, sType string
	err := row.Scan(&vc.ID, &vc.SessionID, &cType, &vc.CallerID, &ceType, &vc.CalleeID, &sType, &vc.DurationSeconds, &vc.RecordingURL, &vc.Transcript, &vc.CreatedAt, &vc.EndedAt)
	if err != nil {
		return nil, err
	}
	vc.CallerType = domain.CallerType(cType)
	vc.CalleeType = domain.CallerType(ceType)
	vc.Status = domain.CallStatus(sType)
	return &vc, nil
}

func (r *VoiceCallRepo) UpdateStatus(ctx context.Context, id int64, status domain.CallStatus) error {
	_, err := r.db.Pool.Exec(ctx, `
		UPDATE voice_calls SET status = $1::call_status, ended_at = NOW() WHERE id = $2
	`, string(status), id)
	return err
}

func (r *VoiceCallRepo) End(ctx context.Context, id int64, durationSeconds int, recordingURL string) error {
	_, err := r.db.Pool.Exec(ctx, `
		UPDATE voice_calls SET status = 'ENDED'::call_status, duration_seconds = $1, recording_url = $2, ended_at = NOW() WHERE id = $3
	`, durationSeconds, recordingURL, id)
	return err
}

func (r *VoiceCallRepo) SetTranscript(ctx context.Context, id int64, transcript string) error {
	_, err := r.db.Pool.Exec(ctx, `
		UPDATE voice_calls SET transcript = $1 WHERE id = $2
	`, transcript, id)
	return err
}

func (r *VoiceCallRepo) ListAll(ctx context.Context) ([]*domain.VoiceCall, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, session_id, caller_type, caller_id, callee_type, callee_id, status, duration_seconds, recording_url, transcript, created_at, ended_at
		FROM voice_calls
		ORDER BY created_at DESC
		LIMIT 100
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var calls []*domain.VoiceCall
	for rows.Next() {
		var vc domain.VoiceCall
		var cType, ceType, sType string
		if err := rows.Scan(&vc.ID, &vc.SessionID, &cType, &vc.CallerID, &ceType, &vc.CalleeID, &sType, &vc.DurationSeconds, &vc.RecordingURL, &vc.Transcript, &vc.CreatedAt, &vc.EndedAt); err != nil {
			return nil, err
		}
		vc.CallerType = domain.CallerType(cType)
		vc.CalleeType = domain.CallerType(ceType)
		vc.Status = domain.CallStatus(sType)
		calls = append(calls, &vc)
	}
	return calls, nil
}

func (r *VoiceCallRepo) GetBySession(ctx context.Context, sessionID string) ([]*domain.VoiceCall, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, session_id, caller_type, caller_id, callee_type, callee_id, status, duration_seconds, recording_url, transcript, created_at, ended_at
		FROM voice_calls
		WHERE session_id = $1
		ORDER BY created_at DESC
	`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var calls []*domain.VoiceCall
	for rows.Next() {
		var vc domain.VoiceCall
		var cType, ceType, sType string
		if err := rows.Scan(&vc.ID, &vc.SessionID, &cType, &vc.CallerID, &ceType, &vc.CalleeID, &sType, &vc.DurationSeconds, &vc.RecordingURL, &vc.Transcript, &vc.CreatedAt, &vc.EndedAt); err != nil {
			return nil, err
		}
		vc.CallerType = domain.CallerType(cType)
		vc.CalleeType = domain.CallerType(ceType)
		vc.Status = domain.CallStatus(sType)
		calls = append(calls, &vc)
	}
	return calls, nil
}

func (r *VoiceCallRepo) GetByID(ctx context.Context, id int64) (*domain.VoiceCall, error) {
	row := r.db.Pool.QueryRow(ctx, `
		SELECT id, session_id, caller_type, caller_id, callee_type, callee_id, status, duration_seconds, recording_url, transcript, created_at, ended_at
		FROM voice_calls
		WHERE id = $1
	`, id)

	var vc domain.VoiceCall
	var cType, ceType, sType string
	err := row.Scan(&vc.ID, &vc.SessionID, &cType, &vc.CallerID, &ceType, &vc.CalleeID, &sType, &vc.DurationSeconds, &vc.RecordingURL, &vc.Transcript, &vc.CreatedAt, &vc.EndedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	vc.CallerType = domain.CallerType(cType)
	vc.CalleeType = domain.CallerType(ceType)
	vc.Status = domain.CallStatus(sType)
	return &vc, nil
}

func (r *VoiceCallRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.Pool.Exec(ctx, `DELETE FROM voice_calls WHERE id = $1`, id)
	return err
}

// ============================================================
// Analytics Repository
// ============================================================

type AnalyticsRepo struct {
	db *DB
}

func NewAnalyticsRepo(db *DB) *AnalyticsRepo {
	return &AnalyticsRepo{db: db}
}

func (r *AnalyticsRepo) GetStats(ctx context.Context) (*domain.AnalyticsStats, error) {
	row := r.db.Pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM chat_cases)::int AS total_cases,
			(SELECT COUNT(DISTINCT session_id) FROM chat_messages)::int AS total_sessions,
			(SELECT COUNT(*) FROM chat_cases WHERE status = 'AI_ACTIVE')::int AS ai_active_cases,
			(SELECT COUNT(*) FROM chat_cases WHERE status = 'NEEDS_HUMAN_CS')::int AS needs_human_cases,
			(SELECT COUNT(*) FROM chat_cases WHERE status = 'HUMAN_CS_ACTIVE')::int AS active_human_cases,
			(SELECT COUNT(*) FROM chat_cases WHERE status = 'RESOLVED')::int AS resolved_cases,
			(SELECT COUNT(*) FROM learning_queue WHERE status = 'APPROVED')::int AS total_learned_qa,
			(SELECT COUNT(*) FROM learning_queue WHERE status = 'PENDING')::int AS pending_learn_count
	`)

	var s domain.AnalyticsStats
	err := row.Scan(
		&s.TotalCases,
		&s.TotalSessions,
		&s.AIActiveCases,
		&s.NeedsHumanCases,
		&s.ActiveHumanCases,
		&s.ResolvedCases,
		&s.TotalLearnedQA,
		&s.PendingLearnCount,
	)
	if err != nil {
		return nil, err
	}

	if s.TotalCases > 0 {
		s.AIServiceRate = float64(int((float64(s.AIActiveCases)/float64(s.TotalCases))*1000)) / 10.0
	}

	return &s, nil
}
