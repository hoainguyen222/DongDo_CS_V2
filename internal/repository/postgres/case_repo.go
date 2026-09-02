package postgres

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
)

type CaseRepo struct {
	db *DB
}

func NewCaseRepo(db *DB) *CaseRepo {
	return &CaseRepo{db: db}
}

func (r *CaseRepo) Upsert(ctx context.Context, sessionID string, guestID *uuid.UUID, customerName, customerPhone string, status domain.CaseStatus, assignedCS, lastMessage string) (*domain.ChatCase, error) {
	row := r.db.Pool.QueryRow(ctx, `
		INSERT INTO chat_cases (session_id, guest_id, customer_name, customer_phone, status, assigned_cs, last_message, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5::case_status, $6, $7, NOW(), NOW())
		ON CONFLICT (session_id) DO UPDATE SET
			customer_name = CASE WHEN EXCLUDED.customer_name <> '' AND EXCLUDED.customer_name <> 'Khách hàng' THEN EXCLUDED.customer_name ELSE chat_cases.customer_name END,
			customer_phone = CASE WHEN EXCLUDED.customer_phone <> '' THEN EXCLUDED.customer_phone ELSE chat_cases.customer_phone END,
			status = CASE
				WHEN chat_cases.status = 'HUMAN_CS_ACTIVE' AND EXCLUDED.status = 'NEEDS_HUMAN_CS' THEN chat_cases.status
				ELSE EXCLUDED.status
			END,
			last_message = COALESCE(NULLIF(EXCLUDED.last_message, ''), chat_cases.last_message),
			assigned_cs = COALESCE(NULLIF(EXCLUDED.assigned_cs, ''), chat_cases.assigned_cs),
			updated_at = NOW()
		RETURNING id, session_id, guest_id, customer_name, customer_phone, status, assigned_cs, last_message, resolution_note, created_at, updated_at
	`, sessionID, guestID, customerName, customerPhone, string(status), assignedCS, lastMessage)

	var c domain.ChatCase
	var statusStr string
	err := row.Scan(&c.ID, &c.SessionID, &c.GuestID, &c.CustomerName, &c.CustomerPhone, &statusStr, &c.AssignedCS, &c.LastMessage, &c.ResolutionNote, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	c.Status = domain.CaseStatus(statusStr)
	return &c, nil
}

func (r *CaseRepo) List(ctx context.Context, statusFilter domain.CaseStatus) ([]*domain.ChatCase, error) {
	var rows pgx.Rows
	var err error

	if statusFilter != "" {
		rows, err = r.db.Pool.Query(ctx, `
			SELECT id, session_id, guest_id, customer_name, customer_phone, status, assigned_cs, last_message, resolution_note, created_at, updated_at
			FROM chat_cases
			WHERE status = $1::case_status
			ORDER BY updated_at DESC
		`, string(statusFilter))
	} else {
		rows, err = r.db.Pool.Query(ctx, `
			SELECT id, session_id, guest_id, customer_name, customer_phone, status, assigned_cs, last_message, resolution_note, created_at, updated_at
			FROM chat_cases
			ORDER BY updated_at DESC
		`)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cases []*domain.ChatCase
	for rows.Next() {
		var c domain.ChatCase
		var statusStr string
		if err := rows.Scan(&c.ID, &c.SessionID, &c.GuestID, &c.CustomerName, &c.CustomerPhone, &statusStr, &c.AssignedCS, &c.LastMessage, &c.ResolutionNote, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		c.Status = domain.CaseStatus(statusStr)
		cases = append(cases, &c)
	}
	return cases, nil
}

func (r *CaseRepo) Get(ctx context.Context, sessionID string) (*domain.ChatCase, error) {
	row := r.db.Pool.QueryRow(ctx, `
		SELECT id, session_id, guest_id, customer_name, customer_phone, status, assigned_cs, last_message, resolution_note, created_at, updated_at
		FROM chat_cases
		WHERE session_id = $1
	`, sessionID)

	var c domain.ChatCase
	var statusStr string
	err := row.Scan(&c.ID, &c.SessionID, &c.GuestID, &c.CustomerName, &c.CustomerPhone, &statusStr, &c.AssignedCS, &c.LastMessage, &c.ResolutionNote, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	c.Status = domain.CaseStatus(statusStr)
	return &c, nil
}

func (r *CaseRepo) Assign(ctx context.Context, sessionID, csUsername string) error {
	_, err := r.db.Pool.Exec(ctx, `
		UPDATE chat_cases SET status = 'HUMAN_CS_ACTIVE', assigned_cs = $1, updated_at = NOW()
		WHERE session_id = $2
	`, csUsername, sessionID)
	return err
}

func (r *CaseRepo) Resolve(ctx context.Context, sessionID, csUsername, resolutionNote string) error {
	_, err := r.db.Pool.Exec(ctx, `
		UPDATE chat_cases SET status = 'RESOLVED', assigned_cs = $1, resolution_note = $2, updated_at = NOW()
		WHERE session_id = $3
	`, csUsername, resolutionNote, sessionID)
	return err
}

func (r *CaseRepo) Delete(ctx context.Context, sessionID string) error {
	_, err := r.db.Pool.Exec(ctx, `DELETE FROM chat_cases WHERE session_id = $1`, sessionID)
	return err
}

func (r *CaseRepo) DeleteAll(ctx context.Context) error {
	_, err := r.db.Pool.Exec(ctx, `DELETE FROM chat_cases`)
	return err
}
