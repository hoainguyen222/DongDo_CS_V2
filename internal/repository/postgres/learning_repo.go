package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
	learningdb "github.com/hoainguyen222/DongDo_CS_V2/internal/repository/sqlcdb/learning"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// LearningRepo implements domain.LearningRepository using sqlc-generated learning queries.
type LearningRepo struct {
	db *DB
}

func NewLearningRepo(db *DB) *LearningRepo {
	return &LearningRepo{db: db}
}

// learningQueueToDomain maps a sqlc LearningQueue to the domain entity.
func learningQueueToDomain(q *learningdb.LearningQueue) *domain.LearningItem {
	out := &domain.LearningItem{
		ID:        q.ID,
		Question:  q.Question,
		Answer:    q.Answer,
		Status:    q.Status,
		CreatedAt: q.CreatedAt,
	}
	if q.SessionID.Valid {
		out.SessionID = q.SessionID.String
	}
	if q.CreatedBy.Valid {
		out.CreatedBy = q.CreatedBy.String
	}
	if q.ApprovedBy.Valid {
		out.ApprovedBy = q.ApprovedBy.String
	}
	if q.ApprovedAt.Valid {
		t := time.Time(q.ApprovedAt.Time)
		out.ApprovedAt = &t
	}
	return out
}

// Add inserts a new learning queue item.
func (r *LearningRepo) Add(ctx context.Context, sessionID, question, answer string, status domain.LearnStatus, createdBy string) (*domain.LearningItem, error) {
	row, err := r.db.Learning.AddToLearningQueue(ctx, learningdb.AddToLearningQueueParams{
		SessionID: pgtype.Text{String: sessionID, Valid: sessionID != ""},
		Question:  question,
		Answer:    answer,
		Status:    status,
		CreatedBy: pgtype.Text{String: createdBy, Valid: createdBy != ""},
	})
	if err != nil {
		return nil, err
	}
	return learningQueueToDomain(&row), nil
}

// ListByStatus returns learning items filtered by status. If status is empty,
// returns all items.
func (r *LearningRepo) ListByStatus(ctx context.Context, status domain.LearnStatus) ([]*domain.LearningItem, error) {
	var rows []learningdb.LearningQueue
	var err error

	if status != "" {
		rows, err = r.db.Learning.ListLearningByStatus(ctx, status)
	} else {
		rows, err = r.db.Learning.ListAllLearning(ctx)
	}
	if err != nil {
		return nil, err
	}

	out := make([]*domain.LearningItem, 0, len(rows))
	for i := range rows {
		out = append(out, learningQueueToDomain(&rows[i]))
	}
	return out, nil
}

// Get returns a single learning item by ID. Returns (nil, nil) when not found.
func (r *LearningRepo) Get(ctx context.Context, id int64) (*domain.LearningItem, error) {
	row, err := r.db.Learning.GetLearningItem(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return learningQueueToDomain(&row), nil
}

// UpdateContent updates the question and answer of a learning item.
func (r *LearningRepo) UpdateContent(ctx context.Context, id int64, question, answer string) error {
	return r.db.Learning.UpdateLearningContent(ctx, learningdb.UpdateLearningContentParams{
		Question: question,
		Answer:   answer,
		ID:       id,
	})
}

// MarkStatus updates the status and approver of a learning item.
func (r *LearningRepo) MarkStatus(ctx context.Context, id int64, status domain.LearnStatus, approvedBy string) error {
	return r.db.Learning.MarkLearningStatus(ctx, learningdb.MarkLearningStatusParams{
		Column1:    status,
		ApprovedBy: pgtype.Text{String: approvedBy, Valid: approvedBy != ""},
		ID:         id,
	})
}

// DeleteBySession removes PENDING learning items for the given session.
func (r *LearningRepo) DeleteBySession(ctx context.Context, sessionID string) error {
	return r.db.Learning.DeleteSessionLearning(ctx, pgtype.Text{String: sessionID, Valid: sessionID != ""})
}

// ClearAll removes all learning queue items.
func (r *LearningRepo) ClearAll(ctx context.Context) error {
	return r.db.Learning.ClearAllLearning(ctx)
}
