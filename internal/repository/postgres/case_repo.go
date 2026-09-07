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

// populateLastSenderTypes fills cs.LastSenderType for each row using a
// single round-trip with a window function. The previous implementation
// issued one `SELECT … LIMIT 1` per session, which scaled as O(N) queries
// and was the primary reason HandleListCases was slow on large pages.
//
// The new SQL walks the index on (session_id, created_at DESC, id DESC),
// picks the newest message per session, and joins back to chat_cases so
// rows with no messages simply keep an empty LastSenderType.
//
// Uses DISTINCT ON (Postgres extension) which is more efficient than
// window functions for "latest per group" in this shape, and matches
// the existing index ordering exactly.
const lastSenderBySessionSQL = `
SELECT DISTINCT ON (session_id)
       session_id,
       sender_type
FROM chat_messages
WHERE session_id = ANY($1::text[])
ORDER BY session_id, created_at DESC, id DESC
`

func (r *CaseRepo) populateLastSenderTypes(ctx context.Context, cases []*domain.ChatCase) {
	if len(cases) == 0 {
		return
	}

	sessionIDs := make([]string, len(cases))
	for i, cs := range cases {
		sessionIDs[i] = cs.SessionID
	}

	// Single round-trip; ignore errors (the column is best-effort enrichment
	// and the listing is still correct without it).
	rows, err := r.db.Pool.Query(ctx, lastSenderBySessionSQL, sessionIDs)
	if err != nil {
		r.logger.Warn().Err(err).Int("case_count", len(cases)).
			Msg("populateLastSenderTypes bulk query failed; LastSenderType left empty")
		return
	}
	defer rows.Close()

	bySession := make(map[string]string, len(cases))
	for rows.Next() {
		var sid, st string
		if err := rows.Scan(&sid, &st); err != nil {
			r.logger.Warn().Err(err).Msg("populateLastSenderTypes scan failed")
			continue
		}
		bySession[sid] = st
	}
	if err := rows.Err(); err != nil {
		r.logger.Warn().Err(err).Msg("populateLastSenderTypes rows iteration error")
		return
	}

	for _, cs := range cases {
		if st, ok := bySession[cs.SessionID]; ok {
			cs.LastSenderType = st
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

func listCasesPageRowToDomain(c *chatdb.ListCasesPageRow) *domain.ChatCase {
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

func searchCasesPageRowToDomain(c *chatdb.SearchCasesPageRow) *domain.ChatCase {
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

// ListPage returns a paginated slice of cases with optional status filter
// and an OR-of-LIKE search across customer_name, customer_phone, session_id,
// and last_message. Returns (rows, total, err) where total is the COUNT(*)
// matching the same filter so the caller can compute total_pages without
// loading the rest of the table.
func (r *CaseRepo) ListPage(ctx context.Context, p domain.ListCasesParams) ([]*domain.ChatCase, int64, error) {
	page := p.Page
	if page < 1 {
		page = 1
	}
	pageSize := p.PageSize
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	offset := (page - 1) * pageSize
	statusFilter := string(p.Status)
	hasSearch := p.Search != ""

	var total int64
	if hasSearch {
		t, err := r.db.Chat.CountCasesSearch(ctx, chatdb.CountCasesSearchParams{
			Column1: statusFilter,
			Lower:   p.Search,
		})
		if err != nil {
			r.logger.Error().Err(err).Str("search", p.Search).Msg("CountCasesSearch failed")
			return nil, 0, err
		}
		total = t
	} else {
		t, err := r.db.Chat.CountCases(ctx, statusFilter)
		if err != nil {
			r.logger.Error().Err(err).Msg("CountCases failed")
			return nil, 0, err
		}
		total = t
	}

	var out []*domain.ChatCase
	if hasSearch {
		rows, err := r.db.Chat.SearchCasesPage(ctx, chatdb.SearchCasesPageParams{
			Column1: statusFilter,
			Limit:   int32(pageSize),
			Offset:  int32(offset),
			Lower:   p.Search,
		})
		if err != nil {
			r.logger.Error().Err(err).Str("search", p.Search).Msg("SearchCasesPage failed")
			return nil, 0, err
		}
		out = make([]*domain.ChatCase, 0, len(rows))
		for i := range rows {
			out = append(out, searchCasesPageRowToDomain(&rows[i]))
		}
	} else {
		rows, err := r.db.Chat.ListCasesPage(ctx, chatdb.ListCasesPageParams{
			Column1: statusFilter,
			Limit:   int32(pageSize),
			Offset:  int32(offset),
		})
		if err != nil {
			r.logger.Error().Err(err).Msg("ListCasesPage failed")
			return nil, 0, err
		}
		out = make([]*domain.ChatCase, 0, len(rows))
		for i := range rows {
			out = append(out, listCasesPageRowToDomain(&rows[i]))
		}
	}

	r.populateLastSenderTypes(ctx, out)
	return out, total, nil
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
