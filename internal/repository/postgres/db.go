package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"log"
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
)

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
		return nil, fmt.Errorf("invalid DATABASE_URL: %w", err)
	}

	config.MaxConns = 25
	config.MinConns = 5
	config.MaxConnLifetime = 1 * time.Hour
	config.MaxConnIdleTime = 30 * time.Minute
	config.HealthCheckPeriod = 1 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create pgxpool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping PostgreSQL: %w", err)
	}

	log.Println("✅ Connected to PostgreSQL database pool")

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

	// Apply pending migrations (idempotent — goose tracks applied versions
	// in the `goose_db_version` table).
	if err := db.RunMigrations(ctx, dsn); err != nil {
		log.Printf("⚠️ Migration warning: %v", err)
	}

	// Seed default admin/cskh accounts (kept out of SQL migrations because
	// the passwords must be hashed with PBKDF2 in Go).
	if err := db.SeedDefaultAccounts(ctx); err != nil {
		log.Printf("⚠️ Account seeding warning: %v", err)
	}

	return db, nil
}

// Close releases the underlying pgx connection pool. Safe to call on a nil
// pool.
func (db *DB) Close() {
	if db.Pool != nil {
		db.Pool.Close()
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
		return fmt.Errorf("failed to open SQL handle for migrations: %w", err)
	}
	defer func() {
		if cerr := sqlDB.Close(); cerr != nil {
			log.Printf("⚠️ migrations sqlDB close error: %v", cerr)
		}
	}()

	goose.SetBaseFS(migrationsFS)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("failed to set goose dialect: %w", err)
	}

	if err := goose.UpContext(ctx, sqlDB, "migrations"); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	log.Println("✅ Database schema migrations applied successfully")
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

	for _, acc := range defaultAccounts {
		count, err := db.Auth.CountUsersByUsername(ctx, acc.username)
		if err != nil {
			return fmt.Errorf("failed to check existing user %s: %w", acc.username, err)
		}

		if count > 0 {
			continue
		}

		hash, salt, err := security.HashPassword(acc.password, "")
		if err != nil {
			return fmt.Errorf("failed to hash password for %s: %w", acc.username, err)
		}

		if _, err := db.Auth.CreateUser(ctx, authdb.CreateUserParams{
			Username:     acc.username,
			PasswordHash: hash,
			Salt:         salt,
			FullName:     acc.fullName,
			Role:         acc.role,
		}); err != nil {
			return fmt.Errorf("failed to seed user %s: %w", acc.username, err)
		}

		log.Printf("🔑 Seeded default account: %s (%s)", acc.username, acc.role)
	}

	return nil
}