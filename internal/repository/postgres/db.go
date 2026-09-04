package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	"github.com/hoainguyen222/DongDo_CS_V2/internal/domain"
	authdb "github.com/hoainguyen222/DongDo_CS_V2/internal/repository/sqlcdb/auth"
	chatdb "github.com/hoainguyen222/DongDo_CS_V2/internal/repository/sqlcdb/chat"
	learningdb "github.com/hoainguyen222/DongDo_CS_V2/internal/repository/sqlcdb/learning"
	voicedb "github.com/hoainguyen222/DongDo_CS_V2/internal/repository/sqlcdb/voice"
	settingsdb "github.com/hoainguyen222/DongDo_CS_V2/internal/repository/sqlcdb/settings"
	analyticsdb "github.com/hoainguyen222/DongDo_CS_V2/internal/repository/sqlcdb/analytics"
	partnerdb "github.com/hoainguyen222/DongDo_CS_V2/internal/repository/sqlcdb/partner"
	"github.com/hoainguyen222/DongDo_CS_V2/pkg/security"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"
	"github.com/rs/zerolog"
)

// logger is the global logger instance for the postgres package.
var logger = zerolog.New(os.Stderr).With().Str("component", "postgres").Logger()

// SetLogger sets a custom zerolog logger for the postgres package.
func SetLogger(l zerolog.Logger) {
	logger = l.With().Str("component", "postgres").Logger()
}

// DB is the top-level PostgreSQL access handle. It bundles the underlying
// connection pool with the generated sqlc query sets for every sub-domain
// (auth, chat, learning, voice, settings, analytics, partner).
type DB struct {
	Pool      *pgxpool.Pool
	Auth      *authdb.Queries
	Chat      *chatdb.Queries
	Learning  *learningdb.Queries
	Voice     *voicedb.Queries
	Settings  *settingsdb.Queries
	Analytics *analyticsdb.Queries
	Partner   *partnerdb.Queries
}

// NewDB initializes a pgx connection pool, pings the database to confirm
// reachability, applies any pending goose migrations, and seeds the default
// admin/cskh accounts.
//
// Pool tuning:
//   - MaxConns           = 25
//   - MinConns           = 5
//   - MaxConnLifetime    = 1h
//   - MaxConnIdleTime    = 30m
//   - HealthCheckPeriod  = 1m
func NewDB(ctx context.Context, dsn string) (*DB, error) {
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		logger.Error().Err(err).Msg("failed to parse PostgreSQL DSN")
		return nil, fmt.Errorf("invalid DATABASE_URL: %w", err)
	}

	config.MaxConns = 25
	config.MinConns = 5
	config.MaxConnLifetime = 1 * time.Hour
	config.MaxConnIdleTime = 30 * time.Minute
	config.HealthCheckPeriod = 1 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		logger.Error().Err(err).Msg("failed to create pgx connection pool")
		return nil, fmt.Errorf("failed to create pgxpool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		logger.Error().Err(err).Msg("failed to ping PostgreSQL database")
		return nil, fmt.Errorf("failed to ping PostgreSQL: %w", err)
	}

	logger.Info().Str("status", "connected").Msg("Connected to PostgreSQL")

	db := &DB{
		Pool:      pool,
		Auth:      authdb.New(pool),
		Chat:      chatdb.New(pool),
		Learning:  learningdb.New(pool),
		Voice:     voicedb.New(pool),
		Settings:  settingsdb.New(pool),
		Analytics: analyticsdb.New(pool),
		Partner:   partnerdb.New(pool),
	}

	if err := db.RunMigrations(ctx, dsn); err != nil {
		logger.Warn().Err(err).Msg("migration warning")
	}

	if err := db.SeedDefaultAccounts(ctx); err != nil {
		logger.Warn().Err(err).Msg("account seeding warning")
	}

	logger.Info().Msg("PostgreSQL initialization complete")
	return db, nil
}

// Close releases the underlying pgx connection pool. Safe to call on a nil
// pool.
func (db *DB) Close() {
	if db.Pool != nil {
		db.Pool.Close()
		logger.Info().Msg("PostgreSQL pool closed")
	}
}

// RunMigrations applies all pending migrations embedded in migrationsFS.
//
// The DSN is opened with database/sql via the pgx stdlib driver (the pgxpool
// pool itself does not expose a *sql.DB, and goose expects one). The migration
// source is the embedded embed.FS rather than a filesystem path so the binary
// is self-contained.
//
// Returns nil when no migrations are pending. Errors other than
// "already at head" are returned to the caller.
func (db *DB) RunMigrations(ctx context.Context, dsn string) error {
	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		logger.Error().Err(err).Msg("failed to open SQL handle for migrations")
		return fmt.Errorf("failed to open SQL handle for migrations: %w", err)
	}
	defer func() {
		if cerr := sqlDB.Close(); cerr != nil {
			logger.Warn().Err(cerr).Msg("migration sqlDB close error")
		}
	}()

	goose.SetBaseFS(migrationsFS)
	if err := goose.SetDialect("postgres"); err != nil {
		logger.Error().Err(err).Msg("failed to set goose dialect")
		return fmt.Errorf("failed to set goose dialect: %w", err)
	}

	if err := goose.UpContext(ctx, sqlDB, "migrations"); err != nil {
		logger.Error().Err(err).Msg("failed to run migrations")
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	logger.Info().Msg("database migrations applied")
	return nil
}

// SeedDefaultAccounts inserts the default admin and CSKH accounts into the
// users table if they do not already exist. Passwords are PBKDF2-hashed
// (golang.org/x/crypto/pbkdf2 via pkg/security).
func (db *DB) SeedDefaultAccounts(ctx context.Context) error {
	defaultAccounts := []struct {
		username string
		password string
		fullName string
		role     domain.UserRole
	}{
		{"admin", "DongDo@2026", "Quản trị viên Đông Đô", domain.RoleOwner},
		{"cskh01", "DongDo@123", "Chuyên viên CSKH 01", domain.RoleCSKH},
		{"cskh02", "DongDo@123", "Chuyên viên CSKH 02", domain.RoleCSKH},
		{"cskh03", "DongDo@123", "Chuyên viên CSKH 03", domain.RoleCSKH},
		{"cskh04", "DongDo@123", "Chuyên viên CSKH 04", domain.RoleCSKH},
		{"cskh05", "DongDo@123", "Chuyên viên CSKH 05", domain.RoleCSKH},
	}

	seededCount := 0
	for _, acc := range defaultAccounts {
		count, err := db.Auth.CountUsersByUsername(ctx, acc.username)
		if err != nil {
			logger.Error().Err(err).Str("username", acc.username).Msg("failed to check existing user")
			return fmt.Errorf("failed to check existing user %s: %w", acc.username, err)
		}

		if count > 0 {
			continue
		}

		hash, salt, err := security.HashPassword(acc.password, "")
		if err != nil {
			logger.Error().Err(err).Str("username", acc.username).Msg("failed to hash password")
			return fmt.Errorf("failed to hash password for %s: %w", acc.username, err)
		}

		if _, err := db.Auth.CreateUser(ctx, authdb.CreateUserParams{
			Username:     acc.username,
			PasswordHash: hash,
			Salt:         salt,
			FullName:     acc.fullName,
			Role:         acc.role,
		}); err != nil {
			logger.Error().Err(err).Str("username", acc.username).Msg("failed to seed user")
			return fmt.Errorf("failed to seed user %s: %w", acc.username, err)
		}

		seededCount++
	}

	logger.Info().Int("seeded", seededCount).Int("total", len(defaultAccounts)).Msg("default accounts seeded")
	return nil
}