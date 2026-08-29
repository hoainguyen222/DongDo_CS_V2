package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
)

// ============================================================
// User Repository
// ============================================================

type UserRepo struct {
	db *DB
}

func NewUserRepo(db *DB) *UserRepo {
	return &UserRepo{db: db}
}

func (r *UserRepo) Create(ctx context.Context, username, passwordHash, salt, fullName string, role domain.UserRole) (*domain.User, error) {
	row := r.db.Pool.QueryRow(ctx, `
		INSERT INTO users (username, password_hash, salt, full_name, role, is_active, created_at)
		VALUES ($1, $2, $3, $4, $5::user_role, TRUE, NOW())
		RETURNING id, username, full_name, role, is_active, created_at
	`, username, passwordHash, salt, fullName, string(role))

	var u domain.User
	var roleStr string
	err := row.Scan(&u.ID, &u.Username, &u.FullName, &roleStr, &u.IsActive, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	u.Role = domain.UserRole(roleStr)
	return &u, nil
}

func (r *UserRepo) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	row := r.db.Pool.QueryRow(ctx, `
		SELECT id, username, password_hash, salt, full_name, role, is_active, created_at
		FROM users WHERE username = $1
	`, username)

	var u domain.User
	var roleStr string
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Salt, &u.FullName, &roleStr, &u.IsActive, &u.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	u.Role = domain.UserRole(roleStr)
	return &u, nil
}

func (r *UserRepo) List(ctx context.Context) ([]*domain.User, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, username, full_name, role, is_active, created_at
		FROM users ORDER BY id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*domain.User
	for rows.Next() {
		var u domain.User
		var roleStr string
		if err := rows.Scan(&u.ID, &u.Username, &u.FullName, &roleStr, &u.IsActive, &u.CreatedAt); err != nil {
			return nil, err
		}
		u.Role = domain.UserRole(roleStr)
		users = append(users, &u)
	}
	return users, nil
}

func (r *UserRepo) Delete(ctx context.Context, username string) error {
	_, err := r.db.Pool.Exec(ctx, `DELETE FROM users WHERE username = $1`, username)
	return err
}

func (r *UserRepo) UpdatePassword(ctx context.Context, username, passwordHash, salt string) error {
	_, err := r.db.Pool.Exec(ctx, `
		UPDATE users SET password_hash = $1, salt = $2 WHERE username = $3
	`, passwordHash, salt, username)
	return err
}

// ============================================================
// Session Repository
// ============================================================

type SessionRepo struct {
	db *DB
}

func NewSessionRepo(db *DB) *SessionRepo {
	return &SessionRepo{db: db}
}

func (r *SessionRepo) Create(ctx context.Context, token, username string, expiresAt time.Time) (*domain.Session, error) {
	row := r.db.Pool.QueryRow(ctx, `
		INSERT INTO sessions (token, username, created_at, expires_at)
		VALUES ($1, $2, NOW(), $3)
		RETURNING id, token, username, created_at, expires_at
	`, token, username, expiresAt)

	var s domain.Session
	err := row.Scan(&s.ID, &s.Token, &s.Username, &s.CreatedAt, &s.ExpiresAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *SessionRepo) Verify(ctx context.Context, token string) (*domain.SessionUser, error) {
	row := r.db.Pool.QueryRow(ctx, `
		SELECT s.username, u.full_name, u.role
		FROM sessions s
		JOIN users u ON s.username = u.username
		WHERE s.token = $1 AND s.expires_at > NOW() AND u.is_active = TRUE
	`, token)

	var su domain.SessionUser
	var roleStr string
	err := row.Scan(&su.Username, &su.FullName, &roleStr)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	su.Role = domain.UserRole(roleStr)
	su.Token = token
	return &su, nil
}

func (r *SessionRepo) Delete(ctx context.Context, token string) error {
	_, err := r.db.Pool.Exec(ctx, `DELETE FROM sessions WHERE token = $1`, token)
	return err
}

func (r *SessionRepo) DeleteExpired(ctx context.Context) error {
	_, err := r.db.Pool.Exec(ctx, `DELETE FROM sessions WHERE expires_at < NOW()`)
	return err
}

func (r *SessionRepo) DeleteByUsername(ctx context.Context, username string) error {
	_, err := r.db.Pool.Exec(ctx, `DELETE FROM sessions WHERE username = $1`, username)
	return err
}

// ============================================================
// Guest Repository
// ============================================================

type GuestRepo struct {
	db *DB
}

func NewGuestRepo(db *DB) *GuestRepo {
	return &GuestRepo{db: db}
}

func (r *GuestRepo) Create(ctx context.Context, guestID uuid.UUID, displayName, phone string) (*domain.Guest, error) {
	row := r.db.Pool.QueryRow(ctx, `
		INSERT INTO guests (guest_id, display_name, phone, created_at)
		VALUES ($1, $2, $3, NOW())
		RETURNING id, guest_id, display_name, phone, created_at
	`, guestID, displayName, phone)

	var g domain.Guest
	err := row.Scan(&g.ID, &g.GuestID, &g.DisplayName, &g.Phone, &g.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &g, nil
}

func (r *GuestRepo) GetByID(ctx context.Context, guestID uuid.UUID) (*domain.Guest, error) {
	row := r.db.Pool.QueryRow(ctx, `
		SELECT id, guest_id, display_name, phone, created_at
		FROM guests WHERE guest_id = $1
	`, guestID)

	var g domain.Guest
	err := row.Scan(&g.ID, &g.GuestID, &g.DisplayName, &g.Phone, &g.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &g, nil
}

func (r *GuestRepo) List(ctx context.Context) ([]*domain.CustomerProfile, error) {
	query := `
		SELECT 
			g.id,
			g.guest_id::text,
			g.display_name,
			COALESCE(g.phone, '') as phone,
			COALESCE(c.session_id, '') as last_session_id,
			COALESCE(c.last_message, '') as last_message,
			COALESCE(c.status::text, '') as last_status,
			g.created_at,
			COALESCE(c.updated_at, g.created_at) as updated_at
		FROM guests g
		LEFT JOIN LATERAL (
			SELECT session_id, last_message, status, updated_at
			FROM chat_cases
			WHERE guest_id = g.guest_id OR (g.display_name != '' AND customer_name = g.display_name)
			ORDER BY updated_at DESC
			LIMIT 1
		) c ON true
		ORDER BY g.created_at DESC
	`

	rows, err := r.db.Pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var profiles []*domain.CustomerProfile
	for rows.Next() {
		var p domain.CustomerProfile
		if err := rows.Scan(
			&p.ID,
			&p.GuestID,
			&p.DisplayName,
			&p.Phone,
			&p.LastSessionID,
			&p.LastMessage,
			&p.LastStatus,
			&p.CreatedAt,
			&p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		profiles = append(profiles, &p)
	}
	return profiles, nil
}

func (r *GuestRepo) Update(ctx context.Context, guestID uuid.UUID, displayName, phone string) error {
	_, err := r.db.Pool.Exec(ctx, `
		UPDATE guests
		SET display_name = $2, phone = $3
		WHERE guest_id = $1
	`, guestID, displayName, phone)
	if err != nil {
		return err
	}

	// Also sync active chat cases
	_, _ = r.db.Pool.Exec(ctx, `
		UPDATE chat_cases
		SET customer_name = $2, customer_phone = $3, updated_at = NOW()
		WHERE guest_id = $1
	`, guestID, displayName, phone)

	return nil
}

func (r *GuestRepo) Delete(ctx context.Context, guestID uuid.UUID) error {
	_, err := r.db.Pool.Exec(ctx, `DELETE FROM guests WHERE guest_id = $1`, guestID)
	return err
}
