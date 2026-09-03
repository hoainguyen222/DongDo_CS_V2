package postgres

import (
	"context"
	"errors"

	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
	voicedb "github.com/hoainguyen222/DongDo_CS_V2/internal/repository/sqlcdb/voice"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// VoiceCallRepo implements domain.VoiceCallRepository using sqlc-generated voice queries.
type VoiceCallRepo struct {
	db *DB
}

// NewVoiceCallRepo constructs a VoiceCallRepo using the shared DB handle.
func NewVoiceCallRepo(db *DB) *VoiceCallRepo {
	return &VoiceCallRepo{db: db}
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
		return nil, err
	}
	return voiceCallFromRow(row), nil
}

func (r *VoiceCallRepo) UpdateStatus(ctx context.Context, id int64, status domain.CallStatus) error {
	return r.db.Voice.UpdateCallStatus(ctx, voicedb.UpdateCallStatusParams{
		Column1: status,
		ID:      id,
	})
}

func (r *VoiceCallRepo) End(ctx context.Context, id int64, durationSeconds int, recordingURL string) error {
	return r.db.Voice.EndCall(ctx, voicedb.EndCallParams{
		DurationSeconds: int32(durationSeconds),
		RecordingUrl:    pgtype.Text{String: recordingURL, Valid: recordingURL != ""},
		ID:              id,
	})
}

func (r *VoiceCallRepo) SetTranscript(ctx context.Context, id int64, transcript string) error {
	return r.db.Voice.SetCallTranscript(ctx, voicedb.SetCallTranscriptParams{
		Transcript: pgtype.Text{String: transcript, Valid: transcript != ""},
		ID:         id,
	})
}

func (r *VoiceCallRepo) ListAll(ctx context.Context) ([]*domain.VoiceCall, error) {
	rows, err := r.db.Voice.ListAllCalls(ctx)
	if err != nil {
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
		return nil, err
	}
	return voiceCallFromRow(row), nil
}

func (r *VoiceCallRepo) Delete(ctx context.Context, id int64) error {
	return r.db.Voice.DeleteCall(ctx, id)
}

// Suppress unused imports
var _ = errors.Is
