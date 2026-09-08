-- +goose Up
-- +goose StatementBegin
-- Backfill chat_cases.guest_id. Older Postgres volumes may have been
-- initialised before the column was added to the canonical 00001 schema
-- (or before this column was added to chat_cases). Without this column,
-- UpsertCase / SyncActiveCasesForGuest fail with SQLSTATE 42703
-- ("column guest_id of relation chat_cases does not exist").
ALTER TABLE chat_cases
    ADD COLUMN IF NOT EXISTS guest_id UUID;

CREATE INDEX IF NOT EXISTS idx_cases_guest_id ON chat_cases(guest_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_cases_guest_id;
ALTER TABLE chat_cases DROP COLUMN IF EXISTS guest_id;
-- +goose StatementEnd
