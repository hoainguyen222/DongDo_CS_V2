package sqlite

import (
	"context"
	"time"
)

// RevokedTokenRepo implements domain.RevokedTokenRepository for SQLite.
type RevokedTokenRepo struct {
	db *DB
}

func NewRevokedTokenRepo(db *DB) *RevokedTokenRepo {
	return &RevokedTokenRepo{db: db}
}

// IsRevoked checks if a jti exists in the revoked_tokens table.
func (r *RevokedTokenRepo) IsRevoked(ctx context.Context, jti string) (bool, error) {
	var exists int
	err := r.db.SQLDB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM revoked_tokens WHERE jti = ?",
		jti,
	).Scan(&exists)
	return exists > 0, err
}

// Revoke inserts a jti into the revocation list.
func (r *RevokedTokenRepo) Revoke(ctx context.Context, jti, userID string, expiresAt time.Time, reason string) error {
	_, err := r.db.SQLDB.ExecContext(ctx, `
		INSERT OR IGNORE INTO revoked_tokens (jti, user_id, revoked_at, expires_at, reason)
		VALUES (?, ?, ?, ?, ?)
	`, jti, userID, time.Now().Format(time.RFC3339), expiresAt.Format(time.RFC3339), reason)
	return err
}

// Cleanup removes entries whose expires_at is in the past.
func (r *RevokedTokenRepo) Cleanup(ctx context.Context) (int64, error) {
	result, err := r.db.SQLDB.ExecContext(ctx,
		"DELETE FROM revoked_tokens WHERE expires_at < datetime('now')",
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// EnsureTable creates the revoked_tokens table if it doesn't exist.
func (r *RevokedTokenRepo) EnsureTable(ctx context.Context) error {
	_, err := r.db.SQLDB.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS revoked_tokens (
			jti        TEXT PRIMARY KEY,
			user_id    TEXT NOT NULL,
			revoked_at TEXT NOT NULL,
			expires_at TEXT NOT NULL,
			reason     TEXT DEFAULT ''
		)
	`)
	return err
}
