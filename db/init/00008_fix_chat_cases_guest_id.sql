-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- Fix chat_cases: add guest_id column if it doesn't exist
-- This handles the case where chat_cases was created by an older
-- init schema that didn't include the guest_id column.
-- ============================================================

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'chat_cases' AND column_name = 'guest_id'
    ) THEN
        ALTER TABLE chat_cases ADD COLUMN guest_id UUID;
        RAISE NOTICE 'Added guest_id column to chat_cases';
    ELSE
        RAISE NOTICE 'guest_id column already exists in chat_cases';
    END IF;
EXCEPTION
    WHEN undefined_table THEN
        RAISE NOTICE 'chat_cases table does not exist yet';
END $$;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'chat_cases' AND column_name = 'guest_id'
    ) THEN
        -- Only drop if no data depends on it
        IF NOT EXISTS (
            SELECT 1 FROM chat_cases WHERE guest_id IS NOT NULL LIMIT 1
        ) THEN
            ALTER TABLE chat_cases DROP COLUMN IF EXISTS guest_id;
            RAISE NOTICE 'Dropped guest_id column from chat_cases';
        ELSE
            RAISE NOTICE 'guest_id column has data, not dropping';
        END IF;
    END IF;
END $$;
-- +goose StatementEnd
