-- Migration: Add Call Service V2 Tables & Types

DO $$ BEGIN
    CREATE TYPE call_status_v2 AS ENUM (
        'CREATED', 'WAITING', 'WAITING_AGENT', 'CONNECTING', 
        'RINGING', 'IN_PROGRESS', 'TERMINATING', 'ENDED', 
        'CANCELLED', 'FAILED', 'TIMEOUT', 'MISSED'
    );
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

CREATE TABLE IF NOT EXISTS calls_v2 (
    id VARCHAR(100) PRIMARY KEY,
    session_id VARCHAR(100) NOT NULL,
    customer_id VARCHAR(100) NOT NULL,
    agent_id VARCHAR(100),
    status call_status_v2 NOT NULL DEFAULT 'CREATED',
    requested_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    assigned_at TIMESTAMPTZ,
    started_at TIMESTAMPTZ,
    ended_at TIMESTAMPTZ,
    duration_seconds INT DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_calls_v2_session ON calls_v2(session_id);
CREATE INDEX IF NOT EXISTS idx_calls_v2_customer ON calls_v2(customer_id);
CREATE INDEX IF NOT EXISTS idx_calls_v2_agent ON calls_v2(agent_id);
CREATE INDEX IF NOT EXISTS idx_calls_v2_status ON calls_v2(status);

CREATE TABLE IF NOT EXISTS call_sessions (
    call_id VARCHAR(100) PRIMARY KEY REFERENCES calls_v2(id) ON DELETE CASCADE,
    customer_channel_id VARCHAR(100),
    agent_channel_id VARCHAR(100),
    bridge_id VARCHAR(100),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS call_events (
    id BIGSERIAL PRIMARY KEY,
    call_id VARCHAR(100) NOT NULL REFERENCES calls_v2(id) ON DELETE CASCADE,
    event_type VARCHAR(50) NOT NULL,
    source VARCHAR(20) NOT NULL,
    payload JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_call_events_call ON call_events(call_id);

CREATE TABLE IF NOT EXISTS call_recordings (
    id BIGSERIAL PRIMARY KEY,
    call_id VARCHAR(100) NOT NULL REFERENCES calls_v2(id) ON DELETE CASCADE,
    recording_url TEXT NOT NULL,
    file_size_bytes BIGINT,
    transcript TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
