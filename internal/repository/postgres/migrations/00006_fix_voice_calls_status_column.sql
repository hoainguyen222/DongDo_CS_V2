-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Fix voice_calls table: add status column if missing
-- Run this SQL to fix: column "status" does not exist error
-- ============================================================

-- Step 1: Ensure call_status enum exists
DO $$ BEGIN
    CREATE TYPE call_status AS ENUM ('RINGING', 'ACTIVE', 'ENDED', 'MISSED', 'REJECTED');
EXCEPTION WHEN duplicate_object THEN null;
END $$;

-- Step 2: Add status column if not exists
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'voice_calls' AND column_name = 'status'
    ) THEN
        ALTER TABLE voice_calls ADD COLUMN status call_status NOT NULL DEFAULT 'RINGING';
        RAISE NOTICE 'Added status column to voice_calls table';
    ELSE
        RAISE NOTICE 'status column already exists in voice_calls table';
    END IF;
EXCEPTION
    WHEN undefined_table THEN
        RAISE NOTICE 'Table voice_calls does not exist yet - it will be created by the application';
END $$;

-- Step 3: Add indexes if not exists
CREATE INDEX IF NOT EXISTS idx_calls_session ON voice_calls(session_id);
CREATE INDEX IF NOT EXISTS idx_calls_status ON voice_calls(status);

-- Verify the fix
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'voice_calls' AND column_name = 'status'
    ) THEN
        RAISE NOTICE 'SUCCESS: voice_calls table is properly configured with status column';
    END IF;
END $$;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- This migration cannot be safely reversed as it ensures data integrity
SELECT 1;
-- +goose StatementEnd
