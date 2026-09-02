package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

// RevokedTokenRepo implements domain.RevokedTokenRepository for PostgreSQL.
type RevokedTokenRepo struct {
	db *DB
}

func NewRevokedTokenRepo(db *DB) *RevokedTokenRepo {
	return &RevokedTokenRepo{db: db}
}

// IsRevoked checks if a jti exists in the revoked_tokens table.
func (r *RevokedTokenRepo) IsRevoked(ctx context.Context, jti string) (bool, error) {
	var exists bool
	err := r.db.Pool.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM revoked_tokens WHERE jti = $1)",
		jti,
	).Scan(&exists)
	return exists, err
}

// Revoke inserts a jti into the revocation list.
func (r *RevokedTokenRepo) Revoke(ctx context.Context, jti, userID string, expiresAt time.Time, reason string) error {
	_, err := r.db.Pool.Exec(ctx, `
		INSERT INTO revoked_tokens (jti, user_id, expires_at, reason)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (jti) DO NOTHING
	`, jti, userID, expiresAt, reason)
	return err
}

// Cleanup removes entries whose expires_at is in the past.
func (r *RevokedTokenRepo) Cleanup(ctx context.Context) (int64, error) {
	result, err := r.db.Pool.Exec(ctx,
		"DELETE FROM revoked_tokens WHERE expires_at < NOW()",
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}

// RevokeWithTx inserts a jti using an existing transaction.
func (r *RevokedTokenRepo) RevokeWithTx(ctx context.Context, tx pgx.Tx, jti, userID string, expiresAt time.Time, reason string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO revoked_tokens (jti, user_id, expires_at, reason)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (jti) DO NOTHING
	`, jti, userID, expiresAt, reason)
	return err
}
