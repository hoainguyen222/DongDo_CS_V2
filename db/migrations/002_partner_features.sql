-- Migration 002: Partner System Features (Dashboard, Config, Reports)
-- Do not alter existing tables from 001_init.sql

-- ============================================================
-- Table: quick_templates (Mẫu tin nhắn CSKH phản hồi nhanh)
-- ============================================================
CREATE TABLE IF NOT EXISTS quick_templates (
    id          BIGSERIAL PRIMARY KEY,
    title       TEXT NOT NULL,
    category    TEXT NOT NULL DEFAULT 'Gắn thẻ',
    content     TEXT NOT NULL,
    created_by  TEXT DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_quick_templates_category ON quick_templates(category);

-- ============================================================
-- Table: system_prompt_history (Lịch sử lưu vết Prompt & Config LLM)
-- ============================================================
CREATE TABLE IF NOT EXISTS system_prompt_history (
    id            BIGSERIAL PRIMARY KEY,
    system_prompt TEXT NOT NULL,
    llm_model     TEXT NOT NULL,
    temperature   NUMERIC(3,2) NOT NULL DEFAULT 0.10,
    created_by    TEXT DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_prompt_history_created ON system_prompt_history(created_at DESC);

-- ============================================================
-- Table: role_permissions (Ma trận phân quyền RBAC: Act / View / None)
-- ============================================================
CREATE TABLE IF NOT EXISTS role_permissions (
    id               BIGSERIAL PRIMARY KEY,
    role_name        TEXT NOT NULL,
    feature_key      TEXT NOT NULL,
    permission_level TEXT NOT NULL DEFAULT 'act',
    can_view         BOOLEAN NOT NULL DEFAULT TRUE,
    can_edit         BOOLEAN NOT NULL DEFAULT FALSE,
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT unique_role_feature UNIQUE (role_name, feature_key)
);

ALTER TABLE role_permissions ADD COLUMN IF NOT EXISTS permission_level TEXT NOT NULL DEFAULT 'act';

CREATE INDEX IF NOT EXISTS idx_role_permissions_role ON role_permissions(role_name);

-- ============================================================
-- Table: csat_feedback (Đánh giá chất lượng dịch vụ CSAT)
-- ============================================================
CREATE TABLE IF NOT EXISTS csat_feedback (
    id             BIGSERIAL PRIMARY KEY,
    session_id     TEXT NOT NULL,
    rating         INT NOT NULL CHECK (rating >= 1 AND rating <= 5),
    feedback_text  TEXT DEFAULT '',
    target_type    TEXT NOT NULL DEFAULT 'ai',
    staff_username TEXT DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_csat_session ON csat_feedback(session_id);
CREATE INDEX IF NOT EXISTS idx_csat_rating ON csat_feedback(rating);

-- ============================================================
-- Table: issue_categories (Phân loại chủ đề thắc mắc/vấn đề khách hàng)
-- ============================================================
CREATE TABLE IF NOT EXISTS issue_categories (
    id            BIGSERIAL PRIMARY KEY,
    session_id    TEXT NOT NULL,
    category_name TEXT NOT NULL,
    ai_resolved   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_issue_category_name ON issue_categories(category_name);
CREATE INDEX IF NOT EXISTS idx_issue_category_session ON issue_categories(session_id);

-- ============================================================
-- Table: system_audit_logs (Nhật ký thao tác & bảo trì hệ thống)
-- ============================================================
CREATE TABLE IF NOT EXISTS system_audit_logs (
    id           BIGSERIAL PRIMARY KEY,
    action_type  TEXT NOT NULL,
    details      TEXT DEFAULT '',
    performed_by TEXT DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_audit_logs_action ON system_audit_logs(action_type);
CREATE INDEX IF NOT EXISTS idx_audit_logs_created ON system_audit_logs(created_at DESC);

-- ============================================================
-- Table: system_errors (Lưu trữ Cảnh báo Lỗi tập trung & tự động dọn dẹp sau 30 ngày)
-- ============================================================
CREATE TABLE IF NOT EXISTS system_errors (
    id            TEXT PRIMARY KEY,
    source        TEXT NOT NULL,
    title         TEXT NOT NULL,
    details       TEXT DEFAULT '',
    severity      TEXT NOT NULL DEFAULT 'high',
    is_handled    BOOLEAN NOT NULL DEFAULT FALSE,
    suggested_fix TEXT DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_system_errors_handled ON system_errors(is_handled);
CREATE INDEX IF NOT EXISTS idx_system_errors_created ON system_errors(created_at DESC);

