-- +goose Up
-- +goose StatementBegin
-- ============================================================
-- Migration: Add call_events audit log + idempotency + lifecycle enums
-- ============================================================
--
-- Implements the audit log required by the call architecture spec
-- (docs/call-architecture.md §4 / §9). One row per state transition
-- with (call_id, event_type, source) deduped to keep ARI reconnects
-- safe.
--
-- Also adds the new lifecycle states (CREATED, WAITING, WAITING_AGENT,
-- CONNECTING, IN_PROGRESS, CANCELLED, TIMEOUT) to the call_status
-- enum so the application can store the canonical names without
-- aliasing.

CREATE TABLE IF NOT EXISTS call_events (
    id          BIGSERIAL PRIMARY KEY,
    call_id     BIGINT      NOT NULL,
    event_type  TEXT        NOT NULL,
    source      TEXT        NOT NULL,
    payload     TEXT        NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS call_events_dedup_idx
    ON call_events (call_id, event_type, source);

CREATE INDEX IF NOT EXISTS call_events_call_idx
    ON call_events (call_id, created_at);

-- Add the new lifecycle states. We do not remove the legacy values
-- (RINGING/DIALING/BRIDGED/ACTIVE) so existing rows keep working.
ALTER TYPE call_status ADD VALUE IF NOT EXISTS 'CREATED';
ALTER TYPE call_status ADD VALUE IF NOT EXISTS 'WAITING';
ALTER TYPE call_status ADD VALUE IF NOT EXISTS 'WAITING_AGENT';
ALTER TYPE call_status ADD VALUE IF NOT EXISTS 'CONNECTING';
ALTER TYPE call_status ADD VALUE IF NOT EXISTS 'IN_PROGRESS';
ALTER TYPE call_status ADD VALUE IF NOT EXISTS 'CANCELLED';
ALTER TYPE call_status ADD VALUE IF NOT EXISTS 'TIMEOUT';

-- Add an idempotency_key column on voice_calls so the application
-- can detect duplicate API calls without round-tripping to Redis.
ALTER TABLE voice_calls
    ADD COLUMN IF NOT EXISTS idempotency_key TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS voice_calls_idem_idx
    ON voice_calls (idempotency_key)
    WHERE idempotency_key IS NOT NULL;

-- Index for the queue reconciliation worker.
CREATE INDEX IF NOT EXISTS voice_calls_status_idx
    ON voice_calls (status, created_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS call_events;
-- +goose StatementEnd
