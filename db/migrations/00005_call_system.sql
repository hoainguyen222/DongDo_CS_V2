-- +goose Up
-- +goose StatementBegin
-- Same content as internal/repository/postgres/migrations/00005_call_system.sql
-- (mirrored to db/migrations for local init scripts).
-- ============================================================
-- Migration 00005: Call System v2
-- ============================================================

DO $$ BEGIN
    CREATE TYPE call_status AS ENUM (
        'CREATED','WAITING','WAITING_AGENT','CONNECTING','RINGING','IN_PROGRESS',
        'ENDED','REJECTED','CANCELLED','MISSED','FAILED','TIMEOUT'
    );
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
    CREATE TYPE call_event_source AS ENUM ('API','ARI','SYSTEM');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
    CREATE TYPE call_party_role AS ENUM ('customer','agent','system');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
    CREATE TYPE call_recording_state AS ENUM ('pending','recording','stopped','failed');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

CREATE TABLE IF NOT EXISTS calls (
    id               UUID PRIMARY KEY,
    customer_id      TEXT NOT NULL,
    agent_id         TEXT,
    status           call_status NOT NULL DEFAULT 'CREATED',
    priority         INT NOT NULL DEFAULT 0,
    requested_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    assigned_at      TIMESTAMPTZ,
    started_at       TIMESTAMPTZ,
    ended_at         TIMESTAMPTZ,
    duration_seconds INT NOT NULL DEFAULT 0,
    last_event_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ari_bridge_id    TEXT,
    failure_reason   TEXT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_calls_customer   ON calls(customer_id);
CREATE INDEX IF NOT EXISTS idx_calls_agent      ON calls(agent_id);
CREATE INDEX IF NOT EXISTS idx_calls_status     ON calls(status);
CREATE INDEX IF NOT EXISTS idx_calls_updated    ON calls(updated_at DESC);

CREATE TABLE IF NOT EXISTS call_events (
    id          BIGSERIAL PRIMARY KEY,
    call_id     UUID NOT NULL REFERENCES calls(id) ON DELETE CASCADE,
    event_type  TEXT NOT NULL,
    source      call_event_source NOT NULL,
    agent_id    TEXT,
    payload     JSONB NOT NULL DEFAULT '{}'::jsonb,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_call_events_dedupe
    ON call_events(call_id, event_type, occurred_at);

CREATE INDEX IF NOT EXISTS idx_call_events_call
    ON call_events(call_id, occurred_at);

CREATE TABLE IF NOT EXISTS call_participants (
    id             BIGSERIAL PRIMARY KEY,
    call_id        UUID NOT NULL REFERENCES calls(id) ON DELETE CASCADE,
    participant_id TEXT NOT NULL,
    role           call_party_role NOT NULL,
    action         TEXT NOT NULL,
    action_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (call_id, role, action)
);

CREATE INDEX IF NOT EXISTS idx_call_participants_call ON call_participants(call_id);

CREATE TABLE IF NOT EXISTS call_recordings (
    id            BIGSERIAL PRIMARY KEY,
    call_id       UUID NOT NULL REFERENCES calls(id) ON DELETE CASCADE,
    ari_recording TEXT,
    storage_url   TEXT,
    started_at    TIMESTAMPTZ,
    ended_at      TIMESTAMPTZ,
    state         call_recording_state NOT NULL DEFAULT 'pending'
);

CREATE INDEX IF NOT EXISTS idx_call_recordings_call ON call_recordings(call_id);

CREATE TABLE IF NOT EXISTS call_idempotency (
    idempotency_key TEXT PRIMARY KEY,
    call_id         UUID NOT NULL REFERENCES calls(id) ON DELETE CASCADE,
    response_body   JSONB NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_call_idempotency_created ON call_idempotency(created_at);

-- Backfill from voice_calls (audit only).
--
-- IMPORTANT: voice_calls has NO `status` column (see schema dump from
-- migration 00001). Earlier versions of this migration referenced
-- `vc.status` which caused the whole transaction to abort when the
-- backfill SELECT was evaluated — that rolled back the entire migration
-- including the `calls` table creation, leaving us with empty enums and
-- no schema. We now derive a synthetic status from `ended_at`.
-- Pre-create call rows for any voice_calls that don't already have a
-- matching call row (md5(session_id)::uuid is stable but voice_calls
-- pre-dates the `calls` table, so most legacy sessions would otherwise
-- produce orphan call_events rows that violate the FK).
INSERT INTO calls (id, customer_id, status, requested_at, duration_seconds)
SELECT md5(vc.session_id)::uuid, vc.session_id,
       CASE WHEN vc.ended_at IS NULL THEN 'IN_PROGRESS'::call_status ELSE 'ENDED'::call_status END,
       vc.created_at,
       COALESCE(vc.duration_seconds, 0)
FROM (
    SELECT DISTINCT ON (md5(vc2.session_id)::uuid)
        md5(vc2.session_id)::uuid AS call_uuid,
        vc2.session_id,
        vc2.ended_at,
        vc2.duration_seconds,
        vc2.created_at
    FROM voice_calls vc2
) vc
ON CONFLICT (id) DO NOTHING;

INSERT INTO call_events (call_id, event_type, source, agent_id, payload, occurred_at)
SELECT
    md5(vc.session_id)::uuid,
    'legacy_voice_call',
    'SYSTEM'::call_event_source,
    CASE WHEN vc.caller_type = 'cskh' THEN vc.caller_id ELSE vc.callee_id END,
    jsonb_build_object(
        'legacy_id', vc.id,
        'session_id', vc.session_id,
        'caller_type', vc.caller_type,
        'caller_id', vc.caller_id,
        'callee_type', vc.callee_type,
        'callee_id', vc.callee_id,
        'duration_seconds', vc.duration_seconds,
        'recording_url', vc.recording_url,
        'transcript', vc.transcript,
        'status', CASE WHEN vc.ended_at IS NULL THEN 'IN_PROGRESS' ELSE 'ENDED' END
    ),
    vc.created_at
FROM voice_calls vc
WHERE NOT EXISTS (
    SELECT 1 FROM call_events ce
    WHERE ce.event_type = 'legacy_voice_call'
      AND (ce.payload->>'legacy_id')::bigint = vc.id
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS call_idempotency;
DROP TABLE IF EXISTS call_recordings;
DROP TABLE IF EXISTS call_participants;
DROP TABLE IF EXISTS call_events;
DROP TABLE IF EXISTS calls;
DROP TYPE IF EXISTS call_recording_state;
DROP TYPE IF EXISTS call_party_role;
DROP TYPE IF EXISTS call_event_source;
DROP TYPE IF EXISTS call_status;
-- +goose StatementEnd
