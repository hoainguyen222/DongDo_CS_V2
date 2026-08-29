-- DongDo_CS_V2 — Full Database Schema (PostgreSQL)
-- All tables, indexes, enums, and seed data.

-- ============================================================
-- ENUM Types
-- ============================================================
DO $$ BEGIN
    CREATE TYPE user_role AS ENUM ('admin', 'cskh', 'customer');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
    CREATE TYPE case_status AS ENUM ('AI_ACTIVE', 'NEEDS_HUMAN_CS', 'HUMAN_CS_ACTIVE', 'RESOLVED');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
    CREATE TYPE sender_type AS ENUM ('guest', 'ai', 'human_cs', 'system');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
    CREATE TYPE learn_status AS ENUM ('PENDING', 'APPROVED', 'REJECTED');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
    CREATE TYPE call_status AS ENUM ('RINGING', 'ACTIVE', 'ENDED', 'MISSED', 'REJECTED');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
    CREATE TYPE caller_type AS ENUM ('guest', 'cskh');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- ============================================================
-- Table: users (CSKH Staff & Admin accounts)
-- ============================================================
CREATE TABLE IF NOT EXISTS users (
    id            BIGSERIAL PRIMARY KEY,
    username      TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    salt          TEXT NOT NULL,
    full_name     TEXT NOT NULL DEFAULT '',
    role          user_role NOT NULL DEFAULT 'cskh',
    is_active     BOOLEAN NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);

-- ============================================================
-- Table: sessions (Auth tokens for CSKH & Admin)
-- ============================================================
CREATE TABLE IF NOT EXISTS sessions (
    id         BIGSERIAL PRIMARY KEY,
    token      TEXT UNIQUE NOT NULL,
    username   TEXT NOT NULL REFERENCES users(username) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_sessions_token ON sessions(token);
CREATE INDEX IF NOT EXISTS idx_sessions_expires ON sessions(expires_at);

-- ============================================================
-- Table: guests (Customer pre-chat registration)
-- ============================================================
CREATE TABLE IF NOT EXISTS guests (
    id           BIGSERIAL PRIMARY KEY,
    guest_id     UUID UNIQUE NOT NULL DEFAULT gen_random_uuid(),
    display_name TEXT NOT NULL,
    phone        TEXT DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_guests_guest_id ON guests(guest_id);

-- ============================================================
-- Table: chat_messages (All messages - Source of Truth)
-- ============================================================
CREATE TABLE IF NOT EXISTS chat_messages (
    id            BIGSERIAL PRIMARY KEY,
    session_id    TEXT NOT NULL,
    sender_type   sender_type NOT NULL,
    sender_id     TEXT DEFAULT '',
    content       TEXT NOT NULL,
    client_msg_id UUID,
    is_learned    BOOLEAN NOT NULL DEFAULT FALSE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_messages_session ON chat_messages(session_id);
CREATE INDEX IF NOT EXISTS idx_messages_client_msg ON chat_messages(client_msg_id);
CREATE INDEX IF NOT EXISTS idx_messages_learned ON chat_messages(is_learned);
CREATE UNIQUE INDEX IF NOT EXISTS idx_messages_idempotent ON chat_messages(client_msg_id) WHERE client_msg_id IS NOT NULL;

-- ============================================================
-- Table: chat_cases (Live CS Inbox)
-- ============================================================
CREATE TABLE IF NOT EXISTS chat_cases (
    id              BIGSERIAL PRIMARY KEY,
    session_id      TEXT UNIQUE NOT NULL,
    guest_id        UUID,
    customer_name   TEXT NOT NULL DEFAULT 'Khách hàng',
    status          case_status NOT NULL DEFAULT 'AI_ACTIVE',
    assigned_cs     TEXT DEFAULT '',
    last_message    TEXT DEFAULT '',
    resolution_note TEXT DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_cases_status ON chat_cases(status);
CREATE INDEX IF NOT EXISTS idx_cases_updated ON chat_cases(updated_at DESC);

-- ============================================================
-- Table: learning_queue (Knowledge learning pipeline)
-- ============================================================
CREATE TABLE IF NOT EXISTS learning_queue (
    id          BIGSERIAL PRIMARY KEY,
    session_id  TEXT DEFAULT '',
    question    TEXT NOT NULL,
    answer      TEXT NOT NULL,
    status      learn_status NOT NULL DEFAULT 'PENDING',
    created_by  TEXT DEFAULT '',
    approved_by TEXT DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    approved_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_learn_status ON learning_queue(status);

-- ============================================================
-- Table: system_settings (Key-Value configuration store)
-- ============================================================
CREATE TABLE IF NOT EXISTS system_settings (
    setting_key   TEXT PRIMARY KEY,
    setting_value TEXT NOT NULL
);

-- ============================================================
-- Table: voice_calls (WebRTC voice call history)
-- ============================================================
CREATE TABLE IF NOT EXISTS voice_calls (
    id               BIGSERIAL PRIMARY KEY,
    session_id       TEXT NOT NULL,
    caller_type      caller_type NOT NULL,
    caller_id        TEXT NOT NULL,
    callee_type      caller_type NOT NULL,
    callee_id        TEXT NOT NULL,
    status           call_status NOT NULL DEFAULT 'RINGING',
    duration_seconds INT NOT NULL DEFAULT 0,
    recording_url    TEXT DEFAULT '',
    transcript       TEXT DEFAULT '',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ended_at         TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_calls_session ON voice_calls(session_id);
CREATE INDEX IF NOT EXISTS idx_calls_status ON voice_calls(status);

-- ============================================================
-- Seed: Default accounts
-- ============================================================
-- Note: Passwords are seeded via Go application startup (hashed with PBKDF2).
-- Default settings
INSERT INTO system_settings (setting_key, setting_value) VALUES
    ('auto_learning_enabled', '0'),
    ('llm_model', 'claude-haiku-4-5-20251001'),
    ('temperature', '0.1')
ON CONFLICT (setting_key) DO NOTHING;
