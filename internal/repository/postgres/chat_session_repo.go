package postgres

import (
	"context"
	"time"

	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
)

// ChatSessionRepo implements domain.ChatSessionRepository for PostgreSQL.
type ChatSessionRepo struct {
	db *DB
}

func NewChatSessionRepo(db *DB) *ChatSessionRepo {
	return &ChatSessionRepo{db: db}
}

// Get retrieves an active session by ID. Returns ErrSessionNotFound if not exists.
func (r *ChatSessionRepo) Get(ctx context.Context, sessionID string) (*domain.ChatSession, error) {
	row := r.db.Pool.QueryRow(ctx, `
		SELECT session_id, guest_id, display_name, created_at, last_active_at,
		       expires_at, ip_address, user_agent, is_active
		FROM chat_sessions
		WHERE session_id = $1
	`, sessionID)

	var s domain.ChatSession
	var guestID *string
	var isActive bool
	err := row.Scan(
		&s.SessionID, &guestID, &s.DisplayName,
		&s.CreatedAt, &s.LastActiveAt, &s.ExpiresAt,
		&s.IPAddress, &s.UserAgent, &isActive,
	)
	if err != nil {
		return nil, err
	}
	s.GuestID = guestID
	s.IsActive = isActive
	return &s, nil
}

// Upsert creates a new session or resumes existing one.
func (r *ChatSessionRepo) Upsert(ctx context.Context, session *domain.ChatSession) (*domain.ChatSession, error) {
	row := r.db.Pool.QueryRow(ctx, `
		INSERT INTO chat_sessions (session_id, guest_id, display_name, created_at,
		                           last_active_at, expires_at, ip_address, user_agent, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, TRUE)
		ON CONFLICT (session_id) DO UPDATE SET
			last_active_at = NOW(),
			expires_at = GREATEST(chat_sessions.expires_at, EXCLUDED.expires_at),
			ip_address = EXCLUDED.ip_address,
			user_agent = EXCLUDED.user_agent,
			is_active = TRUE
		RETURNING session_id, guest_id, display_name, created_at, last_active_at,
		          expires_at, ip_address, user_agent, is_active
	`, session.SessionID, session.GuestID, session.DisplayName,
		session.CreatedAt, session.LastActiveAt, session.ExpiresAt,
		session.IPAddress, session.UserAgent)

	var s domain.ChatSession
	var guestID *string
	var isActive bool
	err := row.Scan(
		&s.SessionID, &guestID, &s.DisplayName,
		&s.CreatedAt, &s.LastActiveAt, &s.ExpiresAt,
		&s.IPAddress, &s.UserAgent, &isActive,
	)
	if err != nil {
		return nil, err
	}
	s.GuestID = guestID
	s.IsActive = isActive
	return &s, nil
}

// Touch updates last_active_at and extends expires_at if not yet extended.
func (r *ChatSessionRepo) Touch(ctx context.Context, sessionID string, newExpiry time.Time) error {
	_, err := r.db.Pool.Exec(ctx, `
		UPDATE chat_sessions
		SET expires_at = GREATEST(expires_at, $2),
		    last_active_at = NOW()
		WHERE session_id = $1 AND is_active = TRUE
	`, sessionID, newExpiry)
	return err
}

// Deactivate marks the session as inactive.
func (r *ChatSessionRepo) Deactivate(ctx context.Context, sessionID string) error {
	_, err := r.db.Pool.Exec(ctx, `
		UPDATE chat_sessions SET is_active = FALSE WHERE session_id = $1
	`, sessionID)
	return err
}

// UpdateGuestID links a guest_id to the session.
func (r *ChatSessionRepo) UpdateGuestID(ctx context.Context, sessionID, guestID string) error {
	_, err := r.db.Pool.Exec(ctx, `
		UPDATE chat_sessions SET guest_id = $1 WHERE session_id = $2
	`, guestID, sessionID)
	return err
}

// UpdateDisplayName updates the session display name.
func (r *ChatSessionRepo) UpdateDisplayName(ctx context.Context, sessionID, displayName string) error {
	_, err := r.db.Pool.Exec(ctx, `
		UPDATE chat_sessions SET display_name = $1 WHERE session_id = $2
	`, displayName, sessionID)
	return err
}

// CleanupExpired removes expired sessions older than 1 day.
func (r *ChatSessionRepo) CleanupExpired(ctx context.Context) (int64, error) {
	result, err := r.db.Pool.Exec(ctx, `
		DELETE FROM chat_sessions
		WHERE expires_at < NOW() - INTERVAL '1 day'
	`)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}