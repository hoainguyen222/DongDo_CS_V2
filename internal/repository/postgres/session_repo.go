// Package postgres provides PostgreSQL-backed implementations of the domain
// repositories using sqlc-generated queries.
package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
	authdb "github.com/hoainguyen222/DongDo_CS_V2/internal/repository/sqlcdb/auth"
	"github.com/jackc/pgx/v5"
)

// ============================================================
// Session Repository
// ============================================================

// SessionRepo persists auth sessions via sqlc-generated auth queries.
type SessionRepo struct {
	db *DB
}

// NewSessionRepo constructs a SessionRepo using the shared DB handle.
func NewSessionRepo(db *DB) *SessionRepo {
	return &SessionRepo{db: db}
}

// Create persists a new session and returns the materialized row.
func (r *SessionRepo) Create(ctx context.Context, token, username string, expiresAt time.Time) (*domain.Session, error) {
	s, err := r.db.Auth.CreateSession(ctx, authdb.CreateSessionParams{
		Token:     token,
		Username:  username,
		ExpiresAt: expiresAt,
	})
	if err != nil {
		return nil, err
	}
	return &domain.Session{
		ID:        s.ID,
		Token:     s.Token,
		Username:  s.Username,
		CreatedAt: s.CreatedAt,
		ExpiresAt: s.ExpiresAt,
	}, nil
}

// Verify looks up a session by token and returns the associated user info.
// Returns (nil, nil) when the token is missing, expired, or the user is
// inactive. The sqlc VerifySession query performs the JOIN with users and
// already enforces expires_at > NOW() and is_active = TRUE.
func (r *SessionRepo) Verify(ctx context.Context, token string) (*domain.SessionUser, error) {
	row, err := r.db.Auth.VerifySession(ctx, token)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &domain.SessionUser{
		Username: row.Username,
		FullName: row.FullName,
		Role:     row.Role,
		Token:    token,
	}, nil
}

// Delete removes a single session by token.
func (r *SessionRepo) Delete(ctx context.Context, token string) error {
	return r.db.Auth.DeleteSession(ctx, token)
}

// DeleteExpired removes all sessions whose expires_at is in the past.
func (r *SessionRepo) DeleteExpired(ctx context.Context) error {
	return r.db.Auth.DeleteExpiredSessions(ctx)
}

// DeleteByUsername removes every session belonging to a username.
func (r *SessionRepo) DeleteByUsername(ctx context.Context, username string) error {
	return r.db.Auth.DeleteUserSessions(ctx, username)
}
