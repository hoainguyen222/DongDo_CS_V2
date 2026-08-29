package postgres

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/hoainguyen222/DongDo_CS_V2/pkg/security"
)

type DB struct {
	Pool *pgxpool.Pool
}

// NewDB initializes PostgreSQL connection pool and executes migrations.
func NewDB(ctx context.Context, databaseURL string) (*DB, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
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
		return nil, fmt.Errorf("failed to ping PostgreSQL: %w", err)
	}

	log.Println("✅ Connected to PostgreSQL database pool")

	db := &DB{Pool: pool}

	// Auto-run schema migration
	if err := db.RunMigrations(ctx, "db/migrations/001_init.sql"); err != nil {
		log.Printf("⚠️ Migration warning: %v", err)
	}

	// Seed default accounts
	if err := db.SeedDefaultAccounts(ctx); err != nil {
		log.Printf("⚠️ Account seeding warning: %v", err)
	}

	return db, nil
}

func (db *DB) Close() {
	if db.Pool != nil {
		db.Pool.Close()
	}
}

// RunMigrations executes the SQL migration file.
func (db *DB) RunMigrations(ctx context.Context, migrationPath string) error {
	content, err := os.ReadFile(migrationPath)
	if err != nil {
		return fmt.Errorf("failed to read migration file %s: %w", migrationPath, err)
	}

	_, err = db.Pool.Exec(ctx, string(content))
	if err != nil {
		return fmt.Errorf("failed to execute migration: %w", err)
	}

	log.Println("✅ Database schema migrations applied successfully")
	return nil
}

// SeedDefaultAccounts creates default admin and CSKH accounts if they do not exist.
func (db *DB) SeedDefaultAccounts(ctx context.Context) error {
	defaultAccounts := []struct {
		username string
		password string
		fullName string
		role     string
	}{
		{"admin", "DongDo@2026", "Quản trị viên Đông Đô", "admin"},
		{"cskh01", "DongDo@123", "Chuyên viên CSKH 01", "cskh"},
		{"cskh02", "DongDo@123", "Chuyên viên CSKH 02", "cskh"},
		{"cskh03", "DongDo@123", "Chuyên viên CSKH 03", "cskh"},
		{"cskh04", "DongDo@123", "Chuyên viên CSKH 04", "cskh"},
		{"cskh05", "DongDo@123", "Chuyên viên CSKH 05", "cskh"},
	}

	for _, acc := range defaultAccounts {
		var count int
		err := db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM users WHERE username = $1", acc.username).Scan(&count)
		if err != nil {
			return err
		}

		if count == 0 {
			hash, salt, err := security.HashPassword(acc.password, "")
			if err != nil {
				return err
			}

			_, err = db.Pool.Exec(ctx, `
				INSERT INTO users (username, password_hash, salt, full_name, role, is_active, created_at)
				VALUES ($1, $2, $3, $4, $5::user_role, TRUE, NOW())
			`, acc.username, hash, salt, acc.fullName, acc.role)
			if err != nil {
				return fmt.Errorf("failed to seed user %s: %w", acc.username, err)
			}
			log.Printf("🔑 Seeded default account: %s (%s)", acc.username, acc.role)
		}
	}

	return nil
}
