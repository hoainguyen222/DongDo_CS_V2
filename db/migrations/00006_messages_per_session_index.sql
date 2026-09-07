-- +goose Up
-- +goose StatementBegin
-- ============================================================
-- Migration 00006: optimize latest-message-per-session queries
-- ============================================================
-- The HandleListCases flow uses a DISTINCT ON (session_id) … ORDER BY
-- session_id, created_at DESC, id DESC query to enrich each chat case
-- with its last_sender_type. The plain (session_id) index forces a sort
-- for every page request; this composite index lets PG return the
-- newest row per session directly from the index without a sort.

CREATE INDEX IF NOT EXISTS idx_messages_session_created
    ON chat_messages (session_id, created_at DESC, id DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_messages_session_created;
-- +goose StatementEnd
