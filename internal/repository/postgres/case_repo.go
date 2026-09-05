package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
	chatdb "github.com/hoainguyen222/DongDo_CS_V2/internal/repository/sqlcdb/chat"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog"
)

// CaseRepo implements domain.CaseRepository using sqlc-generated chatdb queries.
type CaseRepo struct {
	db     *DB
	logger zerolog.Logger
}

func NewCaseRepo(db *DB) *CaseRepo {
	return &CaseRepo{
		db:     db,
		logger: logger.With().Str("repo", "case").Logger(),
	}
}

// upsertCaseRowToDomain converts an UpsertCaseRow (returned by UpsertCase) to domain entity.
func upsertCaseRowToDomain(c *chatdb.UpsertCaseRow) *domain.ChatCase {
	out := &domain.ChatCase{
		ID:            c.ID,
		SessionID:     c.SessionID,
		CustomerName:  c.CustomerName,
		CustomerPhone: c.CustomerPhone,
		Status:        c.Status,
		CreatedAt:     c.CreatedAt,
		UpdatedAt:     c.UpdatedAt,
	}
	if c.GuestID.Valid {
		id := uuid.UUID(c.GuestID.Bytes)
		out.GuestID = &id
	}
	if c.AssignedCs.Valid {
		out.AssignedCS = c.AssignedCs.String
	}
	if c.LastMessage.Valid {
		out.LastMessage = c.LastMessage.String
	}
	if c.ResolutionNote.Valid {
		out.ResolutionNote = c.ResolutionNote.String
	}
	return out
}

// listCasesRowToDomain converts a ListCasesRow to domain entity.
func listCasesRowToDomain(c *chatdb.ListCasesRow) *domain.ChatCase {
	out := &domain.ChatCase{
		ID:            c.ID,
		SessionID:     c.SessionID,
		CustomerName:  c.CustomerName,
		CustomerPhone: c.CustomerPhone,
		Status:        c.Status,
		CreatedAt:     c.CreatedAt,
		UpdatedAt:     c.UpdatedAt,
	}
	if c.GuestID.Valid {
		id := uuid.UUID(c.GuestID.Bytes)
		out.GuestID = &id
	}
	if c.AssignedCs.Valid {
		out.AssignedCS = c.AssignedCs.String
	}
	if c.LastMessage.Valid {
		out.LastMessage = c.LastMessage.String
	}
	if c.ResolutionNote.Valid {
		out.ResolutionNote = c.ResolutionNote.String
	}
	return out
}

// listCasesByStatusRowToDomain converts a ListCasesByStatusRow to domain entity.
func listCasesByStatusRowToDomain(c *chatdb.ListCasesByStatusRow) *domain.ChatCase {
	out := &domain.ChatCase{
		ID:            c.ID,
		SessionID:     c.SessionID,
		CustomerName:  c.CustomerName,
		CustomerPhone: c.CustomerPhone,
		Status:        c.Status,
		CreatedAt:     c.CreatedAt,
		UpdatedAt:     c.UpdatedAt,
	}
	if c.GuestID.Valid {
		id := uuid.UUID(c.GuestID.Bytes)
		out.GuestID = &id
	}
	if c.AssignedCs.Valid {
		out.AssignedCS = c.AssignedCs.String
	}
	if c.LastMessage.Valid {
		out.LastMessage = c.LastMessage.String
	}
	if c.ResolutionNote.Valid {
		out.ResolutionNote = c.ResolutionNote.String
	}
	return out
}

// getCaseRowToDomain converts a GetCaseRow to domain entity.
func getCaseRowToDomain(c *chatdb.GetCaseRow) *domain.ChatCase {
	out := &domain.ChatCase{
		ID:            c.ID,
		SessionID:     c.SessionID,
		CustomerName:  c.CustomerName,
		CustomerPhone: c.CustomerPhone,
		Status:        c.Status,
		CreatedAt:     c.CreatedAt,
		UpdatedAt:     c.UpdatedAt,
	}
	if c.GuestID.Valid {
		id := uuid.UUID(c.GuestID.Bytes)
		out.GuestID = &id
	}
	if c.AssignedCs.Valid {
		out.AssignedCS = c.AssignedCs.String
	}
	if c.LastMessage.Valid {
		out.LastMessage = c.LastMessage.String
	}
	if c.ResolutionNote.Valid {
		out.ResolutionNote = c.ResolutionNote.String
	}
	return out
}

// Upsert inserts or updates a chatdb case via sqlc.UpsertCase.
func (r *CaseRepo) Upsert(
	ctx context.Context,
	sessionID string,
	guestID *uuid.UUID,
	customerName, customerPhone string,
	status domain.CaseStatus,
	assignedCS, lastMessage string,
) (*domain.ChatCase, error) {
	guestUUID := pgtype.UUID{}
	if guestID != nil {
		guestUUID = pgtype.UUID{Bytes: *guestID, Valid: true}
	}

	row, err := r.db.Chat.UpsertCase(ctx, chatdb.UpsertCaseParams{
		SessionID:     sessionID,
		GuestID:       guestUUID,
		CustomerName:  customerName,
		CustomerPhone: customerPhone,
		Status:        status,
		AssignedCs:    pgtype.Text{String: assignedCS, Valid: assignedCS != ""},
		LastMessage:   pgtype.Text{String: lastMessage, Valid: lastMessage != ""},
	})
	if err != nil {
		r.logger.Error().Err(err).Str("session_id", sessionID).Msg("UpsertCase failed")
		return nil, err
	}

	return upsertCaseRowToDomain(&row), nil
}

func (r *CaseRepo) populateLastSenderTypes(ctx context.Context, cases []*domain.ChatCase) {
	for i := range cases {
		var senderType string
		err := r.db.Pool.QueryRow(ctx, `
			SELECT sender_type 
			FROM chat_messages 
			WHERE session_id = $1 
			ORDER BY created_at DESC, id DESC 
			LIMIT 1
		`, cases[i].SessionID).Scan(&senderType)
		if err == nil {
			cases[i].LastSenderType = senderType
		}
	}
}

// List returns chatdb cases, optionally filtered by status.
func (r *CaseRepo) List(ctx context.Context, statusFilter domain.CaseStatus) ([]*domain.ChatCase, error) {
	var out []*domain.ChatCase
	if statusFilter != "" {
		rows, err := r.db.Chat.ListCasesByStatus(ctx, statusFilter)
		if err != nil {
			r.logger.Error().Err(err).Str("status_filter", string(statusFilter)).Msg("ListCasesByStatus failed")
			return nil, err
		}
		out = make([]*domain.ChatCase, 0, len(rows))
		for i := range rows {
			out = append(out, listCasesByStatusRowToDomain(&rows[i]))
		}
	} else {
		rows, err := r.db.Chat.ListCases(ctx)
		if err != nil {
			r.logger.Error().Err(err).Msg("ListCases failed")
			return nil, err
		}
		out = make([]*domain.ChatCase, 0, len(rows))
		for i := range rows {
			out = append(out, listCasesRowToDomain(&rows[i]))
		}
	}

	r.populateLastSenderTypes(ctx, out)
	return out, nil
}

// Get returns a single case by sessionID. Returns (nil, nil) when not found.
func (r *CaseRepo) Get(ctx context.Context, sessionID string) (*domain.ChatCase, error) {
	row, err := r.db.Chat.GetCase(ctx, sessionID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		r.logger.Error().Err(err).Str("session_id", sessionID).Msg("GetCase failed")
		return nil, err
	}

	return getCaseRowToDomain(&row), nil
}

// Assign moves the case to HUMAN_CS_ACTIVE and records the assigned CS username.
func (r *CaseRepo) Assign(ctx context.Context, sessionID, csUsername string) error {
	if err := r.db.Chat.AssignCase(ctx, chatdb.AssignCaseParams{
		AssignedCs: pgtype.Text{String: csUsername, Valid: csUsername != ""},
		SessionID:  sessionID,
	}); err != nil {
		r.logger.Error().Err(err).Str("session_id", sessionID).Msg("AssignCase failed")
		return err
	}
	return nil
}

// Resolve marks the case as RESOLVED with a resolution note.
func (r *CaseRepo) Resolve(ctx context.Context, sessionID, csUsername, resolutionNote string) error {
	if err := r.db.Chat.ResolveCase(ctx, chatdb.ResolveCaseParams{
		AssignedCs:     pgtype.Text{String: csUsername, Valid: csUsername != ""},
		ResolutionNote: pgtype.Text{String: resolutionNote, Valid: resolutionNote != ""},
		SessionID:      sessionID,
	}); err != nil {
		r.logger.Error().Err(err).Str("session_id", sessionID).Msg("ResolveCase failed")
		return err
	}
	return nil
}

// Delete removes a single case by sessionID.
func (r *CaseRepo) Delete(ctx context.Context, sessionID string) error {
	if err := r.db.Chat.DeleteCase(ctx, sessionID); err != nil {
		r.logger.Error().Err(err).Str("session_id", sessionID).Msg("DeleteCase failed")
		return err
	}
	return nil
}

// DeleteAll removes all cases.
func (r *CaseRepo) DeleteAll(ctx context.Context) error {
	if err := r.db.Chat.DeleteAllCases(ctx); err != nil {
		r.logger.Error().Err(err).Msg("DeleteAllCases failed")
		return err
	}
	return nil
}