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
	"github.com/rs/zerolog"
)

// ============================================================
// User Repository
// ============================================================

// UserRepo persists users via sqlc-generated auth queries.
type UserRepo struct {
	db     *DB
	logger zerolog.Logger
}

// NewUserRepo constructs a UserRepo using the shared DB handle.
func NewUserRepo(db *DB) *UserRepo {
	return &UserRepo{
		db:     db,
		logger: logger.With().Str("repo", "user").Logger(),
	}
}

// Create inserts a new user record. The sqlc CreateUser query does not
// return the password hash/salt, but the caller already has those values, so
// we build the domain User from the returned row plus the inputs.
func (r *UserRepo) Create(ctx context.Context, username, passwordHash, salt, fullName string, role domain.UserRole) (*domain.User, error) {
	row, err := r.db.Auth.CreateUser(ctx, authdb.CreateUserParams{
		Username:     username,
		PasswordHash: passwordHash,
		Salt:         salt,
		FullName:     fullName,
		Role:         role,
	})
	if err != nil {
		r.logger.Error().Err(err).Str("username", username).Msg("CreateUser failed")
		return nil, err
	}

	return &domain.User{
		ID:           row.ID,
		Username:     row.Username,
		PasswordHash: passwordHash,
		Salt:         salt,
		FullName:     row.FullName,
		Role:         row.Role,
		IsActive:     row.IsActive,
		CreatedAt:    row.CreatedAt,
	}, nil
}

// GetByUsername returns a user by username (case-insensitive, preserving
// original behavior). Returns (nil, nil) when not found.
func (r *UserRepo) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	u, err := r.db.Auth.GetUserByUsernameInsensitive(ctx, username)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		r.logger.Error().Err(err).Str("username", username).Msg("GetUserByUsernameInsensitive failed")
		return nil, err
	}

	return userFromAuth(u), nil
}

// List returns all users ordered by id ASC.
func (r *UserRepo) List(ctx context.Context) ([]*domain.User, error) {
	rows, err := r.db.Auth.ListUsers(ctx)
	if err != nil {
		r.logger.Error().Err(err).Msg("ListUsers failed")
		return nil, err
	}

	users := make([]*domain.User, 0, len(rows))
	for _, row := range rows {
		users = append(users, &domain.User{
			ID:        row.ID,
			Username:  row.Username,
			FullName:  row.FullName,
			Role:      row.Role,
			IsActive:  row.IsActive,
			CreatedAt: row.CreatedAt,
		})
	}

	return users, nil
}

// Delete removes a user by exact username.
func (r *UserRepo) Delete(ctx context.Context, username string) error {
	if err := r.db.Auth.DeleteUser(ctx, username); err != nil {
		r.logger.Error().Err(err).Str("username", username).Msg("DeleteUser failed")
		return err
	}
	return nil
}

// UpdatePassword updates the password hash and salt for a user.
func (r *UserRepo) UpdatePassword(ctx context.Context, username, passwordHash, salt string) error {
	if err := r.db.Auth.UpdatePassword(ctx, authdb.UpdatePasswordParams{
		PasswordHash: passwordHash,
		Salt:         salt,
		Username:     username,
	}); err != nil {
		r.logger.Error().Err(err).Str("username", username).Msg("UpdatePassword failed")
		return err
	}
	return nil
}

// UpdateUser updates a user's profile fields. When passwordHash is non-empty
// the credentials are also updated; otherwise only profile fields change.
func (r *UserRepo) UpdateUser(ctx context.Context, username, fullName string, role domain.UserRole, isActive bool, passwordHash, salt string) (*domain.User, error) {
	if passwordHash != "" {
		row, err := r.db.Auth.UpdateUserWithPassword(ctx, authdb.UpdateUserWithPasswordParams{
			FullName:     fullName,
			Role:         role,
			IsActive:     isActive,
			PasswordHash: passwordHash,
			Salt:         salt,
			Lower:        username,
		})
		if err != nil {
			r.logger.Error().Err(err).Str("username", username).Msg("UpdateUserWithPassword failed")
			return nil, err
		}

		return userFromUpdateRow(row.ID, row.Username, row.FullName, row.Role, row.IsActive, row.CreatedAt), nil
	}

	row, err := r.db.Auth.UpdateUserWithoutPassword(ctx, authdb.UpdateUserWithoutPasswordParams{
		FullName: fullName,
		Role:     role,
		IsActive: isActive,
		Lower:    username,
	})
	if err != nil {
		r.logger.Error().Err(err).Str("username", username).Msg("UpdateUserWithoutPassword failed")
		return nil, err
	}

	return userFromUpdateRow(row.ID, row.Username, row.FullName, row.Role, row.IsActive, row.CreatedAt), nil
}

// userFromAuth converts a sqlc auth User to a domain User.
func userFromAuth(u authdb.User) *domain.User {
	return &domain.User{
		ID:           u.ID,
		Username:     u.Username,
		PasswordHash: u.PasswordHash,
		Salt:         u.Salt,
		FullName:     u.FullName,
		Role:         u.Role,
		IsActive:     u.IsActive,
		CreatedAt:    u.CreatedAt,
	}
}

// userFromUpdateRow builds a domain User from an update row that does not
// include password fields.
func userFromUpdateRow(id int64, username, fullName string, role domain.UserRole, isActive bool, createdAt time.Time) *domain.User {
	return &domain.User{
		ID:        id,
		Username:  username,
		FullName:  fullName,
		Role:      role,
		IsActive:  isActive,
		CreatedAt: createdAt,
	}
}