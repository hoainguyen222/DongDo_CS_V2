package sqlite

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/hoainguyen222/DongDo_CS_V2/pkg/security"
	_ "modernc.org/sqlite"
)

type DB struct {
	SQLDB *sql.DB
}

func NewDB(dbPath string) (*DB, error) {
	if dbPath == "" {
		dbPath = "chat_history.db"
	}

	db, err := sql.Open("sqlite", dbPath+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database %s: %w", dbPath, err)
	}

	db.SetMaxOpenConns(1) // SQLite single-writer WAL mode
	db.SetMaxIdleConns(1)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping sqlite database: %w", err)
	}

	log.Printf("📦 Connected to SQLite embedded database: %s", dbPath)

	sdb := &DB{SQLDB: db}
	if err := sdb.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to init sqlite schema: %w", err)
	}

	if err := sdb.seedDefaultAccounts(); err != nil {
		log.Printf("⚠️ Account seeding warning: %v", err)
	}

	return sdb, nil
}

func (db *DB) Close() error {
	if db.SQLDB != nil {
		return db.SQLDB.Close()
	}
	return nil
}

func (db *DB) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		salt TEXT NOT NULL,
		full_name TEXT NOT NULL DEFAULT '',
		role TEXT NOT NULL DEFAULT 'cskh',
		is_active INTEGER NOT NULL DEFAULT 1,
		created_at TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS sessions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		token TEXT UNIQUE NOT NULL,
		username TEXT NOT NULL,
		created_at TEXT NOT NULL,
		expires_at TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS guests (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		guest_id TEXT UNIQUE NOT NULL,
		display_name TEXT NOT NULL,
		phone TEXT DEFAULT '',
		created_at TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS chat_messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id TEXT NOT NULL,
		sender_type TEXT NOT NULL,
		sender_id TEXT DEFAULT '',
		content TEXT NOT NULL,
		client_msg_id TEXT UNIQUE,
		is_learned INTEGER NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS chat_cases (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id TEXT UNIQUE NOT NULL,
		guest_id TEXT,
		customer_name TEXT NOT NULL DEFAULT 'Khách hàng',
		status TEXT NOT NULL DEFAULT 'AI_ACTIVE',
		assigned_cs TEXT DEFAULT '',
		last_message TEXT DEFAULT '',
		resolution_note TEXT DEFAULT '',
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS learning_queue (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id TEXT DEFAULT '',
		question TEXT NOT NULL,
		answer TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'PENDING',
		created_by TEXT DEFAULT '',
		approved_by TEXT DEFAULT '',
		created_at TEXT NOT NULL,
		approved_at TEXT
	);

	CREATE TABLE IF NOT EXISTS system_settings (
		setting_key TEXT PRIMARY KEY,
		setting_value TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS voice_calls (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id TEXT NOT NULL,
		caller_type TEXT NOT NULL,
		caller_id TEXT NOT NULL,
		callee_type TEXT NOT NULL,
		callee_id TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'RINGING',
		duration_seconds INTEGER NOT NULL DEFAULT 0,
		recording_url TEXT DEFAULT '',
		transcript TEXT DEFAULT '',
		created_at TEXT NOT NULL,
		ended_at TEXT
	);

	INSERT OR IGNORE INTO system_settings (setting_key, setting_value) VALUES
		('auto_learning_enabled', '0'),
		('llm_model', 'claude-haiku-4-5-20251001'),
		('temperature', '0.1');
	`
	_, err := db.SQLDB.Exec(schema)
	return err
}

func (db *DB) seedDefaultAccounts() error {
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
		err := db.SQLDB.QueryRow("SELECT COUNT(*) FROM users WHERE username = ?", acc.username).Scan(&count)
		if err != nil {
			return err
		}

		if count == 0 {
			hash, salt, err := security.HashPassword(acc.password, "")
			if err != nil {
				return err
			}

			now := time.Now().Format(time.RFC3339)
			_, err = db.SQLDB.Exec(`
				INSERT INTO users (username, password_hash, salt, full_name, role, is_active, created_at)
				VALUES (?, ?, ?, ?, ?, 1, ?)
			`, acc.username, hash, salt, acc.fullName, acc.role, now)
			if err != nil {
				return err
			}
			log.Printf("🔑 Seeded SQLite account: %s (%s)", acc.username, acc.role)
		}
	}
	return nil
}
