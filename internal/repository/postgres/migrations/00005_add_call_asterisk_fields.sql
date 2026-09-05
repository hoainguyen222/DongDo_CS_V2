-- +goose Up
-- +goose StatementBegin
-- ============================================================
-- Migration: Add Asterisk channel tracking to voice_calls
-- ============================================================
--
-- channel_id and uniqueid are populated by the AMI integration when an
-- Originate is acknowledged; they let us correlate incoming AMI events
-- with the call record we already created. linkedid is Asterisk's
-- group id for both legs of a bridged call.

ALTER TABLE voice_calls
    ADD COLUMN IF NOT EXISTS channel_id  TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS unique_id    TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS linked_id    TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS target_exten TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_calls_channel_id ON voice_calls(channel_id);
CREATE INDEX IF NOT EXISTS idx_calls_linked_id ON voice_calls(linked_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_calls_linked_id;
DROP INDEX IF EXISTS idx_calls_channel_id;

ALTER TABLE voice_calls
    DROP COLUMN IF EXISTS target_exten,
    DROP COLUMN IF EXISTS linked_id,
    DROP COLUMN IF EXISTS unique_id,
    DROP COLUMN IF EXISTS channel_id;
-- +goose StatementEnd
