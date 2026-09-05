package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
	voicedb "github.com/hoainguyen222/DongDo_CS_V2/internal/repository/sqlcdb/voice"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog"
)

// VoiceCallRepo implements domain.VoiceCallRepository using sqlc-generated voice queries.
type VoiceCallRepo struct {
	db     *DB
	logger zerolog.Logger
}

// NewVoiceCallRepo constructs a VoiceCallRepo using the shared DB handle.
func NewVoiceCallRepo(db *DB) *VoiceCallRepo {
	return &VoiceCallRepo{
		db:     db,
		logger: logger.With().Str("repo", "voice").Logger(),
	}
}

func voiceCallFromRow(row voicedb.VoiceCall) *domain.VoiceCall {
	vc := &domain.VoiceCall{
		ID:              row.ID,
		SessionID:       row.SessionID,
		CallerType:      row.CallerType,
		CallerID:        row.CallerID,
		CalleeType:      row.CalleeType,
		CalleeID:        row.CalleeID,
		Status:          row.Status,
		DurationSeconds: int(row.DurationSeconds),
		CreatedAt:       row.CreatedAt,
	}
	if row.RecordingUrl.Valid {
		vc.RecordingURL = row.RecordingUrl.String
	}
	if row.Transcript.Valid {
		vc.Transcript = row.Transcript.String
	}
	if row.EndedAt.Valid {
		vc.EndedAt = &row.EndedAt.Time
	}
	return vc
}

func (r *VoiceCallRepo) Create(ctx context.Context, call *domain.VoiceCall) (*domain.VoiceCall, error) {
	row, err := r.db.Voice.CreateVoiceCall(ctx, voicedb.CreateVoiceCallParams{
		SessionID:  call.SessionID,
		CallerType: call.CallerType,
		CallerID:   call.CallerID,
		CalleeType: call.CalleeType,
		CalleeID:   call.CalleeID,
	})
	if err != nil {
		r.logger.Error().Err(err).Str("session_id", call.SessionID).Msg("CreateVoiceCall failed")
		return nil, err
	}
	return voiceCallFromRow(row), nil
}

func (r *VoiceCallRepo) UpdateStatus(ctx context.Context, id int64, status domain.CallStatus) error {
	if err := r.db.Voice.UpdateCallStatus(ctx, voicedb.UpdateCallStatusParams{
		Column1: status,
		ID:      id,
	}); err != nil {
		r.logger.Error().Err(err).Int64("call_id", id).Msg("UpdateCallStatus failed")
		return err
	}
	return nil
}

func (r *VoiceCallRepo) End(ctx context.Context, id int64, durationSeconds int, recordingURL string) error {
	if err := r.db.Voice.EndCall(ctx, voicedb.EndCallParams{
		DurationSeconds: int32(durationSeconds),
		RecordingUrl:    pgtype.Text{String: recordingURL, Valid: recordingURL != ""},
		ID:              id,
	}); err != nil {
		r.logger.Error().Err(err).Int64("call_id", id).Msg("EndCall failed")
		return err
	}
	return nil
}

func (r *VoiceCallRepo) MarkMissed(ctx context.Context, id int64) error {
	if err := r.db.Voice.MarkMissedCall(ctx, id); err != nil {
		r.logger.Error().Err(err).Int64("call_id", id).Msg("MarkMissedCall failed")
		return err
	}
	return nil
}

func (r *VoiceCallRepo) SetTranscript(ctx context.Context, id int64, transcript string) error {
	if err := r.db.Voice.SetCallTranscript(ctx, voicedb.SetCallTranscriptParams{
		Transcript: pgtype.Text{String: transcript, Valid: transcript != ""},
		ID:         id,
	}); err != nil {
		r.logger.Error().Err(err).Int64("call_id", id).Msg("SetCallTranscript failed")
		return err
	}
	return nil
}

func (r *VoiceCallRepo) ListAll(ctx context.Context) ([]*domain.VoiceCall, error) {
	rows, err := r.db.Voice.ListAllCalls(ctx)
	if err != nil {
		r.logger.Error().Err(err).Msg("ListAllCalls failed")
		return nil, err
	}
	list := make([]*domain.VoiceCall, 0, len(rows))
	for _, row := range rows {
		list = append(list, voiceCallFromRow(row))
	}
	return list, nil
}

func (r *VoiceCallRepo) GetBySession(ctx context.Context, sessionID string) ([]*domain.VoiceCall, error) {
	rows, err := r.db.Voice.GetCallsBySession(ctx, sessionID)
	if err != nil {
		r.logger.Error().Err(err).Str("session_id", sessionID).Msg("GetCallsBySession failed")
		return nil, err
	}
	list := make([]*domain.VoiceCall, 0, len(rows))
	for _, row := range rows {
		list = append(list, voiceCallFromRow(row))
	}
	return list, nil
}

func (r *VoiceCallRepo) GetByID(ctx context.Context, id int64) (*domain.VoiceCall, error) {
	row, err := r.db.Voice.GetCallByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		r.logger.Error().Err(err).Int64("call_id", id).Msg("GetCallByID failed")
		return nil, err
	}
	return voiceCallFromRow(row), nil
}

func (r *VoiceCallRepo) Delete(ctx context.Context, id int64) error {
	if err := r.db.Voice.DeleteCall(ctx, id); err != nil {
		r.logger.Error().Err(err).Int64("call_id", id).Msg("DeleteCall failed")
		return err
	}
	return nil
}

// SetAsteriskChannels stores Asterisk identifiers on the call record. The
// query bypasses sqlc because the AMI tracking columns were added by a later
// migration that has not been re-generated.
func (r *VoiceCallRepo) SetAsteriskChannels(ctx context.Context, id int64, channelID, uniqueID, linkedID, targetExten string) error {
	const q = `
		UPDATE voice_calls
		SET channel_id = $2,
		    unique_id = $3,
		    linked_id = $4,
		    target_exten = $5
		WHERE id = $1
	`
	if _, err := r.db.Pool.Exec(ctx, q, id, channelID, uniqueID, linkedID, targetExten); err != nil {
		r.logger.Error().Err(err).Int64("call_id", id).
			Str("channel_id", channelID).
			Msg("SetAsteriskChannels failed")
		return fmt.Errorf("set asterisk channels: %w", err)
	}
	return nil
}

// GetByChannelID looks up a call by its AMI channel id.
func (r *VoiceCallRepo) GetByChannelID(ctx context.Context, channelID string) (*domain.VoiceCall, error) {
	const q = `
		SELECT id, session_id, caller_type, caller_id, callee_type, callee_id,
		       status, duration_seconds, recording_url, transcript, created_at, ended_at
		FROM voice_calls
		WHERE channel_id = $1
		LIMIT 1
	`
	row := r.db.Pool.QueryRow(ctx, q, channelID)
	vc, err := scanVoiceCall(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get by channel id: %w", err)
	}
	return vc, nil
}

// GetByLinkedID looks up a call by its AMI linked id (Asterisk groups both
// legs of a call under a single linked id once bridged).
func (r *VoiceCallRepo) GetByLinkedID(ctx context.Context, linkedID string) (*domain.VoiceCall, error) {
	const q = `
		SELECT id, session_id, caller_type, caller_id, callee_type, callee_id,
		       status, duration_seconds, recording_url, transcript, created_at, ended_at
		FROM voice_calls
		WHERE linked_id = $1
		LIMIT 1
	`
	row := r.db.Pool.QueryRow(ctx, q, linkedID)
	vc, err := scanVoiceCall(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get by linked id: %w", err)
	}
	return vc, nil
}

func scanVoiceCall(row pgx.Row) (*domain.VoiceCall, error) {
	var (
		vc           domain.VoiceCall
		duration     int32
		recordingURL pgtype.Text
		transcript   pgtype.Text
		endedAt      pgtype.Timestamptz
	)
	if err := row.Scan(
		&vc.ID,
		&vc.SessionID,
		&vc.CallerType,
		&vc.CallerID,
		&vc.CalleeType,
		&vc.CalleeID,
		&vc.Status,
		&duration,
		&recordingURL,
		&transcript,
		&vc.CreatedAt,
		&endedAt,
	); err != nil {
		return nil, err
	}
	vc.DurationSeconds = int(duration)
	if recordingURL.Valid {
		vc.RecordingURL = recordingURL.String
	}
	if transcript.Valid {
		vc.Transcript = transcript.String
	}
	if endedAt.Valid {
		vc.EndedAt = &endedAt.Time
	}
	return &vc, nil
}

// Suppress unused imports
var _ = errors.Is
