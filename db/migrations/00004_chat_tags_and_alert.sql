-- +goose Up
-- +goose StatementBegin
-- ============================================================
-- Table: chat_tags (Định nghĩa Tag cho hội thoại)
-- ============================================================
CREATE TABLE IF NOT EXISTS chat_tags (
    id          BIGSERIAL PRIMARY KEY,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    color       TEXT NOT NULL DEFAULT '#6366f1',
    created_by  TEXT NOT NULL DEFAULT '',
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT unique_tag_name UNIQUE (name)
);

CREATE INDEX IF NOT EXISTS idx_chat_tags_active ON chat_tags(is_active);
-- +goose StatementEnd

-- +goose StatementBegin
-- ============================================================
-- Table: case_tags (Quan hệ nhiều-nhiều: Case ↔ Tag)
-- ============================================================
CREATE TABLE IF NOT EXISTS case_tags (
    id          BIGSERIAL PRIMARY KEY,
    session_id  TEXT NOT NULL,
    tag_id      BIGINT NOT NULL REFERENCES chat_tags(id) ON DELETE CASCADE,
    assigned_by TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT unique_case_tag UNIQUE (session_id, tag_id)
);

CREATE INDEX IF NOT EXISTS idx_case_tags_session ON case_tags(session_id);
CREATE INDEX IF NOT EXISTS idx_case_tags_tag ON case_tags(tag_id);
-- +goose StatementEnd

-- +goose StatementBegin
-- ============================================================
-- Table: case_tag_history (Lịch sử gắn/gỡ tag — phục vụ báo cáo)
-- ============================================================
CREATE TABLE IF NOT EXISTS case_tag_history (
    id           BIGSERIAL PRIMARY KEY,
    session_id   TEXT NOT NULL,
    tag_id       BIGINT NOT NULL,
    tag_name     TEXT NOT NULL,
    tag_color    TEXT NOT NULL DEFAULT '#6366f1',
    action       TEXT NOT NULL CHECK (action IN ('attach', 'detach')),
    performed_by TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_case_tag_history_session ON case_tag_history(session_id);
CREATE INDEX IF NOT EXISTS idx_case_tag_history_tag ON case_tag_history(tag_id);
CREATE INDEX IF NOT EXISTS idx_case_tag_history_created ON case_tag_history(created_at DESC);
-- +goose StatementEnd

-- +goose StatementBegin
-- ============================================================
-- Table: alert_config (Cấu hình cảnh báo tin nhắn chờ — singleton)
-- ============================================================
CREATE TABLE IF NOT EXISTS alert_config (
    id              BIGINT PRIMARY KEY DEFAULT 1,
    is_enabled      BOOLEAN NOT NULL DEFAULT FALSE,
    timeout_seconds INT NOT NULL DEFAULT 60,
    alert_content   TEXT NOT NULL DEFAULT '⚠️ Có tin nhắn khách hàng chờ trả lời! Vui lòng xử lý ngay.',
    updated_by      TEXT NOT NULL DEFAULT '',
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT alert_config_singleton CHECK (id = 1)
);

-- Insert default config if not exists
INSERT INTO alert_config (id, is_enabled, timeout_seconds, alert_content, updated_by)
VALUES (1, FALSE, 60, '⚠️ Có tin nhắn khách hàng chờ trả lời! Vui lòng xử lý ngay.', 'system')
ON CONFLICT (id) DO NOTHING;
-- +goose StatementEnd

-- +goose StatementBegin
-- ============================================================
-- Table: alert_events (Lịch sử sự kiện cảnh báo — phục vụ báo cáo)
-- ============================================================
CREATE TABLE IF NOT EXISTS alert_events (
    id              BIGSERIAL PRIMARY KEY,
    session_id      TEXT NOT NULL,
    timeout_seconds INT NOT NULL DEFAULT 60,
    triggered_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at     TIMESTAMPTZ,
    is_resolved     BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE INDEX IF NOT EXISTS idx_alert_events_session ON alert_events(session_id);
CREATE INDEX IF NOT EXISTS idx_alert_events_resolved ON alert_events(is_resolved);
CREATE INDEX IF NOT EXISTS idx_alert_events_triggered ON alert_events(triggered_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS alert_events;
DROP TABLE IF EXISTS alert_config;
DROP TABLE IF EXISTS case_tag_history;
DROP TABLE IF EXISTS case_tags;
DROP TABLE IF EXISTS chat_tags;
-- +goose StatementEnd
