-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Fix voice_calls table: ensure status column exists
-- ============================================================

-- First, ensure the call_status enum type exists
DO $$ BEGIN
    CREATE TYPE call_status AS ENUM ('RINGING', 'ACTIVE', 'ENDED', 'MISSED', 'REJECTED');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- Add status column if it doesn't exist (for databases created before this column was added)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_name = 'voice_calls' AND column_name = 'status'
    ) THEN
        ALTER TABLE voice_calls ADD COLUMN status call_status NOT NULL DEFAULT 'RINGING';
    END IF;
EXCEPTION WHEN undefined_column THEN
    -- If table doesn't exist at all, create it
    IF NOT EXISTS (SELECT FROM pg_tables WHERE tablename = 'voice_calls') THEN
        CREATE TABLE IF NOT EXISTS voice_calls (
            id               BIGSERIAL PRIMARY KEY,
            session_id       TEXT NOT NULL,
            caller_type      TEXT NOT NULL DEFAULT 'guest',
            caller_id        TEXT NOT NULL,
            callee_type      TEXT NOT NULL DEFAULT 'cskh',
            callee_id        TEXT NOT NULL,
            status           call_status NOT NULL DEFAULT 'RINGING',
            duration_seconds INT NOT NULL DEFAULT 0,
            recording_url    TEXT DEFAULT '',
            transcript       TEXT DEFAULT '',
            created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            ended_at         TIMESTAMPTZ
        );
    ELSE
        ALTER TABLE voice_calls ADD COLUMN status TEXT NOT NULL DEFAULT 'RINGING';
    END IF;
END $$;

-- Add indexes if they don't exist
CREATE INDEX IF NOT EXISTS idx_calls_session ON voice_calls(session_id);
CREATE INDEX IF NOT EXISTS idx_calls_status ON voice_calls(status);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- This migration cannot be safely reversed as it ensures data integrity
-- +goose StatementEnd
