package postgres

import (
	"context"
	"time"

	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog"
)

// ChatTagRepo implements domain.ChatTagRepository using raw pgxpool queries.
type ChatTagRepo struct {
	db     *DB
	logger zerolog.Logger
}

// NewChatTagRepo constructs a ChatTagRepo.
func NewChatTagRepo(db *DB) *ChatTagRepo {
	return &ChatTagRepo{
		db:     db,
		logger: logger.With().Str("repo", "chat_tag").Logger(),
	}
}

// ============================================================
// Tag CRUD
// ============================================================

func (r *ChatTagRepo) ListTags(ctx context.Context) ([]*domain.ChatTag, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, name, description, color, created_by, is_active, created_at, updated_at
		 FROM chat_tags WHERE is_active = TRUE ORDER BY name ASC`)
	if err != nil {
		r.logger.Error().Err(err).Msg("failed to list chat tags")
		return nil, err
	}
	defer rows.Close()

	var tags []*domain.ChatTag
	for rows.Next() {
		t := &domain.ChatTag{}
		if err := rows.Scan(&t.ID, &t.Name, &t.Description, &t.Color, &t.CreatedBy, &t.IsActive, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}

func (r *ChatTagRepo) CreateTag(ctx context.Context, tag *domain.ChatTag) (*domain.ChatTag, error) {
	created := &domain.ChatTag{}
	err := r.db.Pool.QueryRow(ctx,
		`INSERT INTO chat_tags (name, description, color, created_by, is_active)
		 VALUES ($1, $2, $3, $4, TRUE)
		 RETURNING id, name, description, color, created_by, is_active, created_at, updated_at`,
		tag.Name, tag.Description, tag.Color, tag.CreatedBy,
	).Scan(&created.ID, &created.Name, &created.Description, &created.Color, &created.CreatedBy, &created.IsActive, &created.CreatedAt, &created.UpdatedAt)
	if err != nil {
		r.logger.Error().Err(err).Str("name", tag.Name).Msg("failed to create chat tag")
		return nil, err
	}
	return created, nil
}

func (r *ChatTagRepo) UpdateTag(ctx context.Context, id int64, name, description, color string) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE chat_tags SET name=$1, description=$2, color=$3, updated_at=NOW() WHERE id=$4 AND is_active=TRUE`,
		name, description, color, id,
	)
	if err != nil {
		r.logger.Error().Err(err).Int64("id", id).Msg("failed to update chat tag")
	}
	return err
}

func (r *ChatTagRepo) DeleteTag(ctx context.Context, id int64) error {
	// Soft delete
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE chat_tags SET is_active=FALSE, updated_at=NOW() WHERE id=$1`, id,
	)
	if err != nil {
		r.logger.Error().Err(err).Int64("id", id).Msg("failed to delete chat tag")
	}
	return err
}

// ============================================================
// Case Tag operations
// ============================================================

func (r *ChatTagRepo) GetCaseTags(ctx context.Context, sessionID string) ([]*domain.CaseTag, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT ct.id, ct.session_id, ct.tag_id, ct.assigned_by, ct.created_at,
		        t.name, t.color
		 FROM case_tags ct
		 JOIN chat_tags t ON t.id = ct.tag_id
		 WHERE ct.session_id = $1
		 ORDER BY ct.created_at ASC`,
		sessionID,
	)
	if err != nil {
		r.logger.Error().Err(err).Str("session_id", sessionID).Msg("failed to get case tags")
		return nil, err
	}
	defer rows.Close()

	var tags []*domain.CaseTag
	for rows.Next() {
		t := &domain.CaseTag{}
		if err := rows.Scan(&t.ID, &t.SessionID, &t.TagID, &t.AssignedBy, &t.CreatedAt, &t.TagName, &t.TagColor); err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}

func (r *ChatTagRepo) AttachTag(ctx context.Context, sessionID string, tagID int64, assignedBy string) error {
	// Get tag info for history
	var tagName, tagColor string
	err := r.db.Pool.QueryRow(ctx, `SELECT name, color FROM chat_tags WHERE id=$1`, tagID).
		Scan(&tagName, &tagColor)
	if err != nil {
		r.logger.Error().Err(err).Int64("tag_id", tagID).Msg("tag not found for attach")
		return err
	}

	// Insert case_tag (ignore conflict = already attached)
	_, err = r.db.Pool.Exec(ctx,
		`INSERT INTO case_tags (session_id, tag_id, assigned_by) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
		sessionID, tagID, assignedBy,
	)
	if err != nil {
		r.logger.Error().Err(err).Str("session_id", sessionID).Int64("tag_id", tagID).Msg("failed to attach tag")
		return err
	}

	// Record history
	_, err = r.db.Pool.Exec(ctx,
		`INSERT INTO case_tag_history (session_id, tag_id, tag_name, tag_color, action, performed_by)
		 VALUES ($1, $2, $3, $4, 'attach', $5)`,
		sessionID, tagID, tagName, tagColor, assignedBy,
	)
	if err != nil {
		r.logger.Warn().Err(err).Msg("failed to record tag attach history")
	}
	return nil
}

func (r *ChatTagRepo) DetachTag(ctx context.Context, sessionID string, tagID int64, performedBy string) error {
	// Get tag info for history before deleting
	var tagName, tagColor string
	_ = r.db.Pool.QueryRow(ctx, `SELECT name, color FROM chat_tags WHERE id=$1`, tagID).
		Scan(&tagName, &tagColor)

	_, err := r.db.Pool.Exec(ctx,
		`DELETE FROM case_tags WHERE session_id=$1 AND tag_id=$2`,
		sessionID, tagID,
	)
	if err != nil {
		r.logger.Error().Err(err).Str("session_id", sessionID).Int64("tag_id", tagID).Msg("failed to detach tag")
		return err
	}

	// Record history
	_, _ = r.db.Pool.Exec(ctx,
		`INSERT INTO case_tag_history (session_id, tag_id, tag_name, tag_color, action, performed_by)
		 VALUES ($1, $2, $3, $4, 'detach', $5)`,
		sessionID, tagID, tagName, tagColor, performedBy,
	)
	return nil
}

// ============================================================
// Alert Config
// ============================================================

func (r *ChatTagRepo) GetAlertConfig(ctx context.Context) (*domain.AlertConfig, error) {
	cfg := &domain.AlertConfig{}
	err := r.db.Pool.QueryRow(ctx,
		`SELECT id, is_enabled, timeout_seconds, alert_content, updated_by, updated_at
		 FROM alert_config WHERE id=1`,
	).Scan(&cfg.ID, &cfg.IsEnabled, &cfg.TimeoutSeconds, &cfg.AlertContent, &cfg.UpdatedBy, &cfg.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			// Return safe defaults
			return &domain.AlertConfig{
				ID:             1,
				IsEnabled:      false,
				TimeoutSeconds: 60,
				AlertContent:   "⚠️ Có tin nhắn khách hàng chờ trả lời! Vui lòng xử lý ngay.",
				UpdatedAt:      time.Now(),
			}, nil
		}
		r.logger.Error().Err(err).Msg("failed to get alert config")
		return nil, err
	}
	return cfg, nil
}

func (r *ChatTagRepo) UpsertAlertConfig(ctx context.Context, cfg *domain.AlertConfig) error {
	_, err := r.db.Pool.Exec(ctx,
		`INSERT INTO alert_config (id, is_enabled, timeout_seconds, alert_content, updated_by, updated_at)
		 VALUES (1, $1, $2, $3, $4, NOW())
		 ON CONFLICT (id) DO UPDATE SET
		   is_enabled      = EXCLUDED.is_enabled,
		   timeout_seconds = EXCLUDED.timeout_seconds,
		   alert_content   = EXCLUDED.alert_content,
		   updated_by      = EXCLUDED.updated_by,
		   updated_at      = NOW()`,
		cfg.IsEnabled, cfg.TimeoutSeconds, cfg.AlertContent, cfg.UpdatedBy,
	)
	if err != nil {
		r.logger.Error().Err(err).Msg("failed to upsert alert config")
	}
	return err
}

// ============================================================
// Alert Events
// ============================================================

func (r *ChatTagRepo) CreateAlertEvent(ctx context.Context, sessionID string, timeoutSeconds int) (*domain.AlertEvent, error) {
	evt := &domain.AlertEvent{}
	err := r.db.Pool.QueryRow(ctx,
		`INSERT INTO alert_events (session_id, timeout_seconds)
		 VALUES ($1, $2)
		 RETURNING id, session_id, timeout_seconds, triggered_at, resolved_at, is_resolved`,
		sessionID, timeoutSeconds,
	).Scan(&evt.ID, &evt.SessionID, &evt.TimeoutSeconds, &evt.TriggeredAt, &evt.ResolvedAt, &evt.IsResolved)
	if err != nil {
		r.logger.Error().Err(err).Str("session_id", sessionID).Msg("failed to create alert event")
		return nil, err
	}
	return evt, nil
}

func (r *ChatTagRepo) ResolveAlertEvent(ctx context.Context, sessionID string) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE alert_events SET is_resolved=TRUE, resolved_at=NOW()
		 WHERE session_id=$1 AND is_resolved=FALSE`,
		sessionID,
	)
	if err != nil {
		r.logger.Error().Err(err).Str("session_id", sessionID).Msg("failed to resolve alert event")
	}
	return err
}

func (r *ChatTagRepo) ListUnresolvedAlertEvents(ctx context.Context) ([]*domain.AlertEvent, error) {
	rows, err := r.db.Pool.Query(ctx,
		`SELECT id, session_id, timeout_seconds, triggered_at, resolved_at, is_resolved
		 FROM alert_events WHERE is_resolved=FALSE ORDER BY triggered_at DESC`)
	if err != nil {
		r.logger.Error().Err(err).Msg("failed to list unresolved alert events")
		return nil, err
	}
	defer rows.Close()

	var events []*domain.AlertEvent
	for rows.Next() {
		e := &domain.AlertEvent{}
		if err := rows.Scan(&e.ID, &e.SessionID, &e.TimeoutSeconds, &e.TriggeredAt, &e.ResolvedAt, &e.IsResolved); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}
