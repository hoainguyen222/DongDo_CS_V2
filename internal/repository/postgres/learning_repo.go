package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
)

type LearningRepo struct {
	db *DB
}

func NewLearningRepo(db *DB) *LearningRepo {
	return &LearningRepo{db: db}
}

func (r *LearningRepo) Add(ctx context.Context, sessionID, question, answer string, status domain.LearnStatus, createdBy string) (*domain.LearningItem, error) {
	row := r.db.Pool.QueryRow(ctx, `
		INSERT INTO learning_queue (session_id, question, answer, status, created_by, created_at)
		VALUES ($1, $2, $3, $4::learn_status, $5, NOW())
		RETURNING id, session_id, question, answer, status, created_by, approved_by, created_at, approved_at
	`, sessionID, question, answer, string(status), createdBy)

	var item domain.LearningItem
	var statusStr string
	err := row.Scan(&item.ID, &item.SessionID, &item.Question, &item.Answer, &statusStr, &item.CreatedBy, &item.ApprovedBy, &item.CreatedAt, &item.ApprovedAt)
	if err != nil {
		return nil, err
	}
	item.Status = domain.LearnStatus(statusStr)
	return &item, nil
}

func (r *LearningRepo) ListByStatus(ctx context.Context, status domain.LearnStatus) ([]*domain.LearningItem, error) {
	var rows pgx.Rows
	var err error

	if status != "" {
		rows, err = r.db.Pool.Query(ctx, `
			SELECT id, session_id, question, answer, status, created_by, approved_by, created_at, approved_at
			FROM learning_queue
			WHERE status = $1::learn_status
			ORDER BY id DESC
		`, string(status))
	} else {
		rows, err = r.db.Pool.Query(ctx, `
			SELECT id, session_id, question, answer, status, created_by, approved_by, created_at, approved_at
			FROM learning_queue
			ORDER BY id DESC
		`)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*domain.LearningItem
	for rows.Next() {
		var item domain.LearningItem
		var statusStr string
		if err := rows.Scan(&item.ID, &item.SessionID, &item.Question, &item.Answer, &statusStr, &item.CreatedBy, &item.ApprovedBy, &item.CreatedAt, &item.ApprovedAt); err != nil {
			return nil, err
		}
		item.Status = domain.LearnStatus(statusStr)
		items = append(items, &item)
	}
	return items, nil
}

func (r *LearningRepo) Get(ctx context.Context, id int64) (*domain.LearningItem, error) {
	row := r.db.Pool.QueryRow(ctx, `
		SELECT id, session_id, question, answer, status, created_by, approved_by, created_at, approved_at
		FROM learning_queue
		WHERE id = $1
	`, id)

	var item domain.LearningItem
	var statusStr string
	err := row.Scan(&item.ID, &item.SessionID, &item.Question, &item.Answer, &statusStr, &item.CreatedBy, &item.ApprovedBy, &item.CreatedAt, &item.ApprovedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	item.Status = domain.LearnStatus(statusStr)
	return &item, nil
}

func (r *LearningRepo) UpdateContent(ctx context.Context, id int64, question, answer string) error {
	_, err := r.db.Pool.Exec(ctx, `
		UPDATE learning_queue SET question = $1, answer = $2 WHERE id = $3
	`, question, answer, id)
	return err
}

func (r *LearningRepo) MarkStatus(ctx context.Context, id int64, status domain.LearnStatus, approvedBy string) error {
	_, err := r.db.Pool.Exec(ctx, `
		UPDATE learning_queue SET status = $1::learn_status, approved_by = $2, approved_at = NOW() WHERE id = $3
	`, string(status), approvedBy, id)
	return err
}

func (r *LearningRepo) DeleteBySession(ctx context.Context, sessionID string) error {
	_, err := r.db.Pool.Exec(ctx, `
		DELETE FROM learning_queue WHERE session_id = $1 AND status = 'PENDING'
	`, sessionID)
	return err
}

func (r *LearningRepo) ClearAll(ctx context.Context) error {
	_, err := r.db.Pool.Exec(ctx, `DELETE FROM learning_queue`)
	return err
}
