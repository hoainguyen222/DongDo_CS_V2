package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
	chatdb "github.com/hoainguyen222/DongDo_CS_V2/internal/repository/sqlcdb/chat"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog"
)

// CaseRepo implements domain.CaseRepository using sqlc-generated chatdb queries and direct pool queries.
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

func parseAssignedHistory(raw []byte) []string {
	var history []string
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &history)
	}
	if history == nil {
		history = []string{}
	}
	return history
}

// Get returns a single case by sessionID with full helper and assignment history fields.
func (r *CaseRepo) Get(ctx context.Context, sessionID string) (*domain.ChatCase, error) {
	query := `
		SELECT c.id, c.session_id, c.guest_id, c.customer_name, c.customer_phone,
		       c.status, COALESCE(c.assigned_cs, ''), COALESCE(c.active_assigned_cs, ''),
		       COALESCE(c.assigned_cs_history, '[]'::jsonb),
		       COALESCE(c.requires_help, false), COALESCE(c.help_content, ''),
		       COALESCE(c.help_requested_by, ''), c.help_requested_at,
		       COALESCE(c.last_message, ''), COALESCE(c.resolution_note, ''), c.created_at, c.updated_at,
		       COALESCE((SELECT sender_type FROM chat_messages WHERE session_id = c.session_id ORDER BY created_at DESC, id DESC LIMIT 1), 'guest') AS last_sender_type
		FROM chat_cases c
		WHERE c.session_id = $1
	`
	var c domain.ChatCase
	var guestUUID pgtype.UUID
	var historyBytes []byte
	var helpReqAt sql.NullTime

	err := r.db.Pool.QueryRow(ctx, query, sessionID).Scan(
		&c.ID, &c.SessionID, &guestUUID, &c.CustomerName, &c.CustomerPhone,
		&c.Status, &c.AssignedCS, &c.ActiveAssignedCS,
		&historyBytes,
		&c.RequiresHelp, &c.HelpContent,
		&c.HelpRequestedBy, &helpReqAt,
		&c.LastMessage, &c.ResolutionNote, &c.CreatedAt, &c.UpdatedAt,
		&c.LastSenderType,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		r.logger.Error().Err(err).Str("session_id", sessionID).Msg("GetCase failed")
		return nil, err
	}

	if guestUUID.Valid {
		id := uuid.UUID(guestUUID.Bytes)
		c.GuestID = &id
	}
	c.AssignedCSHistory = parseAssignedHistory(historyBytes)
	if helpReqAt.Valid {
		c.HelpRequestedAt = &helpReqAt.Time
	}

	return &c, nil
}

// Upsert inserts or updates a chatdb case.
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

	return r.Get(ctx, row.SessionID)
}

// List returns chatdb cases, optionally filtered by status with full fields.
func (r *CaseRepo) List(ctx context.Context, statusFilter domain.CaseStatus) ([]*domain.ChatCase, error) {
	query := `
		SELECT c.id, c.session_id, c.guest_id, c.customer_name, c.customer_phone,
		       c.status, COALESCE(c.assigned_cs, ''), COALESCE(c.active_assigned_cs, ''),
		       COALESCE(c.assigned_cs_history, '[]'::jsonb),
		       COALESCE(c.requires_help, false), COALESCE(c.help_content, ''),
		       COALESCE(c.help_requested_by, ''), c.help_requested_at,
		       COALESCE(c.last_message, ''), COALESCE(c.resolution_note, ''), c.created_at, c.updated_at,
		       COALESCE((SELECT sender_type FROM chat_messages WHERE session_id = c.session_id ORDER BY created_at DESC, id DESC LIMIT 1), 'guest') AS last_sender_type
		FROM chat_cases c
	`
	var rows pgx.Rows
	var err error
	if statusFilter != "" {
		query += ` WHERE c.status = $1 ORDER BY c.updated_at DESC`
		rows, err = r.db.Pool.Query(ctx, query, statusFilter)
	} else {
		query += ` ORDER BY c.updated_at DESC`
		rows, err = r.db.Pool.Query(ctx, query)
	}

	if err != nil {
		r.logger.Error().Err(err).Msg("ListCases failed")
		return nil, err
	}
	defer rows.Close()

	out := make([]*domain.ChatCase, 0)
	for rows.Next() {
		var c domain.ChatCase
		var guestUUID pgtype.UUID
		var historyBytes []byte
		var helpReqAt sql.NullTime

		if err := rows.Scan(
			&c.ID, &c.SessionID, &guestUUID, &c.CustomerName, &c.CustomerPhone,
			&c.Status, &c.AssignedCS, &c.ActiveAssignedCS,
			&historyBytes,
			&c.RequiresHelp, &c.HelpContent,
			&c.HelpRequestedBy, &helpReqAt,
			&c.LastMessage, &c.ResolutionNote, &c.CreatedAt, &c.UpdatedAt,
			&c.LastSenderType,
		); err != nil {
			continue
		}

		if guestUUID.Valid {
			id := uuid.UUID(guestUUID.Bytes)
			c.GuestID = &id
		}
		c.AssignedCSHistory = parseAssignedHistory(historyBytes)
		if helpReqAt.Valid {
			c.HelpRequestedAt = &helpReqAt.Time
		}
		out = append(out, &c)
	}
	return out, nil
}

// ListPaged returns paginated chat cases with optional status filter and search.
// Returns (items, totalCount, error).
func (r *CaseRepo) ListPaged(ctx context.Context, statusFilter domain.CaseStatus, search string, page, limit int) ([]*domain.ChatCase, int64, error) {
	offset := (page - 1) * limit

	// Convert statusFilter to string for SQL query (empty string = no filter)
	statusStr := ""
	if statusFilter != "" {
		statusStr = string(statusFilter)
	}

	// Empty search is treated as a no-filter (the SQL handles '' via LIKE).
	// Get total count
	total, err := r.db.Chat.CountCases(ctx, chatdb.CountCasesParams{
		Column1: statusStr,
		Column2: search,
	})
	if err != nil {
		r.logger.Error().Err(err).Msg("CountCases failed")
		return nil, 0, err
	}

	// Get paginated results with full fields including last_sender_type
	query := `
		SELECT c.id, c.session_id, c.guest_id, c.customer_name, c.customer_phone,
		       c.status, COALESCE(c.assigned_cs, ''), COALESCE(c.active_assigned_cs, ''),
		       COALESCE(c.assigned_cs_history, '[]'::jsonb),
		       COALESCE(c.requires_help, false), COALESCE(c.help_content, ''),
		       COALESCE(c.help_requested_by, ''), c.help_requested_at,
		       COALESCE(c.last_message, ''), COALESCE(c.resolution_note, ''), c.created_at, c.updated_at,
		       COALESCE((SELECT sender_type FROM chat_messages WHERE session_id = c.session_id ORDER BY created_at DESC, id DESC LIMIT 1), 'guest') AS last_sender_type
		FROM chat_cases c
		WHERE (
			($1::text IS NULL OR $1 = '' OR c.status::text = $1::text)
			AND (
				$2::text IS NULL OR $2 = ''
				OR LOWER(c.customer_name) LIKE '%' || LOWER($2::text) || '%'
				OR LOWER(c.customer_phone) LIKE '%' || LOWER($2::text) || '%'
				OR LOWER(c.session_id) LIKE '%' || LOWER($2::text) || '%'
				OR LOWER(c.last_message) LIKE '%' || LOWER($2::text) || '%'
			)
		)
		ORDER BY c.updated_at DESC, c.id DESC
		LIMIT $3 OFFSET $4
	`

	rows, err := r.db.Pool.Query(ctx, query, statusStr, search, limit, offset)
	if err != nil {
		r.logger.Error().Err(err).Msg("ListPaged query failed")
		return nil, 0, err
	}
	defer rows.Close()

	out := make([]*domain.ChatCase, 0, limit)
	for rows.Next() {
		var c domain.ChatCase
		var guestUUID pgtype.UUID
		var historyBytes []byte
		var helpReqAt sql.NullTime

		err := rows.Scan(
			&c.ID, &c.SessionID, &guestUUID, &c.CustomerName, &c.CustomerPhone,
			&c.Status, &c.AssignedCS, &c.ActiveAssignedCS,
			&historyBytes,
			&c.RequiresHelp, &c.HelpContent,
			&c.HelpRequestedBy, &helpReqAt,
			&c.LastMessage, &c.ResolutionNote, &c.CreatedAt, &c.UpdatedAt,
			&c.LastSenderType,
		)
		if err != nil {
			r.logger.Error().Err(err).Msg("ListPaged scan failed")
			return nil, 0, err
		}

		if guestUUID.Valid {
			id := uuid.UUID(guestUUID.Bytes)
			c.GuestID = &id
		}
		c.AssignedCSHistory = parseAssignedHistory(historyBytes)
		if helpReqAt.Valid {
			c.HelpRequestedAt = &helpReqAt.Time
		}

		out = append(out, &c)
	}

	if err := rows.Err(); err != nil {
		r.logger.Error().Err(err).Msg("ListPaged rows error")
		return nil, 0, err
	}

	return out, total, nil
}

// Count returns the total count of cases matching the given filters.
func (r *CaseRepo) Count(ctx context.Context, statusFilter domain.CaseStatus, search string) (int64, error) {
	// Convert statusFilter to string for SQL query (empty string = no filter)
	statusStr := ""
	if statusFilter != "" {
		statusStr = string(statusFilter)
	}

	countRow, err := r.db.Chat.CountCases(ctx, chatdb.CountCasesParams{
		Column1: statusStr,
		Column2: search,
	})
	if err != nil {
		r.logger.Error().Err(err).Msg("CountCases failed")
		return 0, err
	}
	return countRow, nil
}

// listCasesPagedRowToDomain converts a ListCasesPagedRow to domain entity.
func listCasesPagedRowToDomain(c *chatdb.ListCasesPagedRow) *domain.ChatCase {
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

// Assign moves the case to HUMAN_CS_ACTIVE and records the assigned CS username.
func (r *CaseRepo) Assign(ctx context.Context, sessionID, csUsername string) error {
	query := `
		UPDATE chat_cases
		SET status = 'HUMAN_CS_ACTIVE'::case_status,
		    assigned_cs = $1,
		    active_assigned_cs = $1,
		    assigned_cs_history = CASE 
		        WHEN assigned_cs_history @> to_jsonb($1::text) THEN assigned_cs_history 
		        ELSE assigned_cs_history || to_jsonb($1::text) 
		    END,
		    updated_at = NOW()
		WHERE session_id = $2
	`
	_, err := r.db.Pool.Exec(ctx, query, csUsername, sessionID)
	if err != nil {
		r.logger.Error().Err(err).Str("session_id", sessionID).Msg("AssignCase failed")
		return err
	}
	return nil
}

// Resolve marks the case as RESOLVED with a resolution note and automatically updates helper_status to 'done'.
func (r *CaseRepo) Resolve(ctx context.Context, sessionID, csUsername, resolutionNote string) error {
	query := `
		UPDATE chat_cases
		SET status = 'RESOLVED'::case_status,
		    helper_status = 'done',
		    assigned_cs = CASE WHEN $1 <> '' THEN $1 ELSE assigned_cs END,
		    resolution_note = CASE WHEN $2 <> '' THEN $2 ELSE resolution_note END,
		    updated_at = NOW()
		WHERE session_id = $3
	`
	_, err := r.db.Pool.Exec(ctx, query, csUsername, resolutionNote, sessionID)
	if err != nil {
		r.logger.Error().Err(err).Str("session_id", sessionID).Msg("ResolveCase failed")
		return err
	}
	return nil
}

// SubmitHelper flags a case as requires_help = true with detailed content and staff username.
func (r *CaseRepo) SubmitHelper(ctx context.Context, sessionID, helpContent, requestedBy string) error {
	query := `
		UPDATE chat_cases
		SET requires_help = true,
		    helper_status = 'new',
		    help_content = $2,
		    help_requested_by = $3,
		    help_requested_at = NOW(),
		    updated_at = NOW()
		WHERE session_id = $1
	`
	_, err := r.db.Pool.Exec(ctx, query, sessionID, helpContent, requestedBy)
	if err != nil {
		r.logger.Error().Err(err).Str("session_id", sessionID).Msg("SubmitHelper failed")
		return err
	}
	return nil
}

// ListHelperCases returns cases based on helper_status filter (new, processing, done, open, all).
// Normal cases without requires_help = true or tag 'Cần Hỗ Trợ' will NEVER be returned.
func (r *CaseRepo) ListHelperCases(ctx context.Context, helperStatusFilter string) ([]*domain.ChatCase, error) {
	baseHelperCond := `(c.requires_help = true OR c.session_id IN (
		SELECT ct.session_id FROM case_tags ct JOIN chat_tags t ON ct.tag_id = t.id WHERE t.name = 'Cần Hỗ Trợ'
	))`

	var whereClause string
	switch strings.ToLower(helperStatusFilter) {
	case "new":
		whereClause = fmt.Sprintf(`%s AND COALESCE(c.helper_status, 'new') = 'new' AND c.status <> 'RESOLVED'`, baseHelperCond)
	case "processing":
		whereClause = fmt.Sprintf(`%s AND COALESCE(c.helper_status, 'new') = 'processing' AND c.status <> 'RESOLVED'`, baseHelperCond)
	case "done":
		whereClause = fmt.Sprintf(`%s AND (COALESCE(c.helper_status, 'new') = 'done' OR c.status = 'RESOLVED')`, baseHelperCond)
	case "all":
		whereClause = baseHelperCond
	default: // "open" / ""
		whereClause = fmt.Sprintf(`%s AND COALESCE(c.helper_status, 'new') IN ('new', 'processing') AND c.status <> 'RESOLVED'`, baseHelperCond)
	}

	query := fmt.Sprintf(`
		SELECT c.id, c.session_id, c.guest_id, c.customer_name, c.customer_phone,
		       c.status, COALESCE(c.assigned_cs, ''), COALESCE(c.active_assigned_cs, ''),
		       COALESCE(c.assigned_cs_history, '[]'::jsonb),
		       COALESCE(c.requires_help, false), COALESCE(c.help_content, ''),
		       COALESCE(c.help_requested_by, ''), c.help_requested_at,
		       COALESCE(c.helper_status, 'new'),
		       COALESCE(c.last_message, ''), COALESCE(c.resolution_note, ''), c.created_at, c.updated_at,
		       COALESCE((SELECT sender_type FROM chat_messages WHERE session_id = c.session_id ORDER BY created_at DESC, id DESC LIMIT 1), 'guest') AS last_sender_type
		FROM chat_cases c
		WHERE %s
		ORDER BY COALESCE(c.help_requested_at, c.updated_at) DESC
	`, whereClause)

	rows, err := r.db.Pool.Query(ctx, query)
	if err != nil {
		r.logger.Error().Err(err).Msg("ListHelperCases failed")
		return nil, err
	}
	defer rows.Close()

	out := make([]*domain.ChatCase, 0)
	for rows.Next() {
		var c domain.ChatCase
		var guestUUID pgtype.UUID
		var historyBytes []byte
		var helpReqAt sql.NullTime

		if err := rows.Scan(
			&c.ID, &c.SessionID, &guestUUID, &c.CustomerName, &c.CustomerPhone,
			&c.Status, &c.AssignedCS, &c.ActiveAssignedCS,
			&historyBytes,
			&c.RequiresHelp, &c.HelpContent,
			&c.HelpRequestedBy, &helpReqAt,
			&c.HelperStatus,
			&c.LastMessage, &c.ResolutionNote, &c.CreatedAt, &c.UpdatedAt,
			&c.LastSenderType,
		); err != nil {
			continue
		}

		if guestUUID.Valid {
			id := uuid.UUID(guestUUID.Bytes)
			c.GuestID = &id
		}
		c.AssignedCSHistory = parseAssignedHistory(historyBytes)
		if helpReqAt.Valid {
			c.HelpRequestedAt = &helpReqAt.Time
		}
		out = append(out, &c)
	}
	return out, nil
}

// ProcessHelper handles 'take_over' or 'transfer' actions from Leaders/Admins.
func (r *CaseRepo) ProcessHelper(ctx context.Context, sessionID, action, targetUsername, currentLeader string) error {
	finalTarget := targetUsername
	if action == "take_over" || finalTarget == "" {
		finalTarget = currentLeader
	}

	query := `
		UPDATE chat_cases
		SET helper_status = 'processing',
		    status = 'HUMAN_CS_ACTIVE'::case_status,
		    active_assigned_cs = $2,
		    assigned_cs = $2,
		    assigned_cs_history = CASE 
		        WHEN assigned_cs_history @> to_jsonb($2::text) THEN assigned_cs_history 
		        ELSE assigned_cs_history || to_jsonb($2::text) 
		    END,
		    updated_at = NOW()
		WHERE session_id = $1
	`
	_, err := r.db.Pool.Exec(ctx, query, sessionID, finalTarget)
	if err != nil {
		r.logger.Error().Err(err).Str("session_id", sessionID).Msg("ProcessHelper failed")
		return err
	}
	return nil
}

// UpdateHelperStatus updates the helper_status (new, processing, done).
// When status is 'done', it automatically sets requires_help = false and detaches tag "Cần Hỗ Trợ".
func (r *CaseRepo) UpdateHelperStatus(ctx context.Context, sessionID string, status string) error {
	cleanStatus := strings.ToLower(strings.TrimSpace(status))
	if cleanStatus == "done" {
		query := `
			UPDATE chat_cases
			SET helper_status = 'done',
			    requires_help = false,
			    updated_at = NOW()
			WHERE session_id = $1
		`
		if _, err := r.db.Pool.Exec(ctx, query, sessionID); err != nil {
			r.logger.Error().Err(err).Str("session_id", sessionID).Msg("UpdateHelperStatus done failed")
			return err
		}

		// Detach tag "Cần Hỗ Trợ"
		detachQuery := `
			DELETE FROM case_tags
			WHERE session_id = $1 AND tag_id IN (SELECT id FROM chat_tags WHERE name = 'Cần Hỗ Trợ')
		`
		_, _ = r.db.Pool.Exec(ctx, detachQuery, sessionID)
		return nil
	}

	query := `
		UPDATE chat_cases
		SET helper_status = $2,
		    updated_at = NOW()
		WHERE session_id = $1
	`
	_, err := r.db.Pool.Exec(ctx, query, sessionID, cleanStatus)
	if err != nil {
		r.logger.Error().Err(err).Str("session_id", sessionID).Msg("UpdateHelperStatus failed")
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
