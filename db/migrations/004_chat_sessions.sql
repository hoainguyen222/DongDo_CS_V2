-- Guest session management (stateless customer auth)
-- Separate from staff JWT (revoked_tokens) — customer uses opaque tokens stored in DB.
CREATE TABLE IF NOT EXISTS chat_sessions (
    session_id     TEXT PRIMARY KEY,                              -- 32-byte random hex
    guest_id       TEXT,                                            -- nullable, set when customer registers
    display_name   TEXT NOT NULL DEFAULT 'Khách',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_active_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at     TIMESTAMPTZ NOT NULL,                            -- DEFAULT NOW() + 30 days
    ip_address     TEXT,
    user_agent     TEXT,
    is_active      BOOLEAN NOT NULL DEFAULT TRUE
);

-- Index for cleanup job
CREATE INDEX IF NOT EXISTS idx_chat_sessions_expires ON chat_sessions(expires_at) WHERE is_active = TRUE;
CREATE INDEX IF NOT EXISTS idx_chat_sessions_guest   ON chat_sessions(guest_id)    WHERE guest_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_chat_sessions_active  ON chat_sessions(last_active_at);

-- Trigger auto-update last_active_at on UPDATE
CREATE OR REPLACE FUNCTION update_last_active()
RETURNS TRIGGER AS $$
BEGIN
    NEW.last_active_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_session_activity ON chat_sessions;
CREATE TRIGGER trg_session_activity
    BEFORE UPDATE ON chat_sessions
    FOR EACH ROW
    EXECUTE FUNCTION update_last_active();