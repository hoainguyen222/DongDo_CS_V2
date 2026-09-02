package sqlite

import (
	"context"
	"time"

	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
)

// ChatSessionRepo implements domain.ChatSessionRepository for SQLite.
type ChatSessionRepo struct {
	db *DB
}

func NewChatSessionRepo(db *DB) *ChatSessionRepo {
	return &ChatSessionRepo{db: db}
}

// Get retrieves a session by ID.
func (r *ChatSessionRepo) Get(ctx context.Context, sessionID string) (*domain.ChatSession, error) {
	row := r.db.SQLDB.QueryRowContext(ctx, `
		SELECT session_id, guest_id, display_name, created_at, last_active_at,
		       expires_at, ip_address, user_agent, is_active
		FROM chat_sessions
		WHERE session_id = ?
	`, sessionID)

	var s domain.ChatSession
	var guestID *string
	var isActiveInt int
	var createdAt, lastActive, expiresAt string

	err := row.Scan(
		&s.SessionID, &guestID, &s.DisplayName,
		&createdAt, &lastActive, &expiresAt,
		&s.IPAddress, &s.UserAgent, &isActiveInt,
	)
	if err != nil {
		return nil, err
	}

	if t, perr := time.Parse(time.RFC3339, createdAt); perr == nil {
		s.CreatedAt = t
	}
	if t, perr := time.Parse(time.RFC3339, lastActive); perr == nil {
		s.LastActiveAt = t
	}
	if t, perr := time.Parse(time.RFC3339, expiresAt); perr == nil {
		s.ExpiresAt = t
	}

	s.GuestID = guestID
	s.IsActive = isActiveInt == 1
	return &s, nil
}

// Upsert creates a new session or resumes existing one.
func (r *ChatSessionRepo) Upsert(ctx context.Context, session *domain.ChatSession) (*domain.ChatSession, error) {
	now := time.Now().Format(time.RFC3339)
	createdAt := session.CreatedAt.Format(time.RFC3339)
	lastActive := session.LastActiveAt.Format(time.RFC3339)
	expiresAt := session.ExpiresAt.Format(time.RFC3339)

	_, err := r.db.SQLDB.ExecContext(ctx, `
		INSERT INTO chat_sessions
		    (session_id, guest_id, display_name, created_at, last_active_at,
		     expires_at, ip_address, user_agent, is_active)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 1)
		ON CONFLICT(session_id) DO UPDATE SET
			last_active_at = ?,
			expires_at = MAX(expires_at, excluded.expires_at),
			ip_address = excluded.ip_address,
			user_agent = excluded.user_agent,
			is_active = 1
	`, session.SessionID, session.GuestID, session.DisplayName,
		createdAt, lastActive, expiresAt,
		session.IPAddress, session.UserAgent,
		now,
	)
	if err != nil {
		return nil, err
	}
	return r.Get(ctx, session.SessionID)
}

// Touch updates last_active_at and extends expires_at.
func (r *ChatSessionRepo) Touch(ctx context.Context, sessionID string, newExpiry time.Time) error {
	now := time.Now().Format(time.RFC3339)
	newExp := newExpiry.Format(time.RFC3339)

	_, err := r.db.SQLDB.ExecContext(ctx, `
		UPDATE chat_sessions
		SET last_active_at = ?,
		    expires_at = MAX(expires_at, ?)
		WHERE session_id = ? AND is_active = 1
	`, now, newExp, sessionID)
	return err
}

// Deactivate marks the session as inactive.
func (r *ChatSessionRepo) Deactivate(ctx context.Context, sessionID string) error {
	_, err := r.db.SQLDB.ExecContext(ctx, `
		UPDATE chat_sessions SET is_active = 0 WHERE session_id = ?
	`, sessionID)
	return err
}

// UpdateGuestID links a guest_id to the session.
func (r *ChatSessionRepo) UpdateGuestID(ctx context.Context, sessionID, guestID string) error {
	_, err := r.db.SQLDB.ExecContext(ctx, `
		UPDATE chat_sessions SET guest_id = ? WHERE session_id = ?
	`, guestID, sessionID)
	return err
}

// UpdateDisplayName updates the session display name.
func (r *ChatSessionRepo) UpdateDisplayName(ctx context.Context, sessionID, displayName string) error {
	_, err := r.db.SQLDB.ExecContext(ctx, `
		UPDATE chat_sessions SET display_name = ? WHERE session_id = ?
	`, displayName, sessionID)
	return err
}

// CleanupExpired removes expired sessions older than 1 day.
func (r *ChatSessionRepo) CleanupExpired(ctx context.Context) (int64, error) {
	cutoff := time.Now().Add(-24 * time.Hour).Format(time.RFC3339)
	result, err := r.db.SQLDB.ExecContext(ctx, `
		DELETE FROM chat_sessions WHERE expires_at < ?
	`, cutoff)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}