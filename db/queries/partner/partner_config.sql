-- ============================================================
-- Partner — system prompt history
-- ============================================================

-- name: GetLatestSystemPrompt :one
SELECT id, system_prompt, llm_model, temperature, created_by, created_at
FROM system_prompt_history
ORDER BY created_at DESC
LIMIT 1;

-- name: InsertSystemPromptHistory :one
INSERT INTO system_prompt_history (system_prompt, llm_model, temperature, created_by, created_at)
VALUES ($1, $2, $3, $4, NOW())
RETURNING id, system_prompt, llm_model, temperature, created_by, created_at;

-- name: ListSystemPromptHistory :many
SELECT id, system_prompt, llm_model, temperature, created_by, created_at
FROM system_prompt_history
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- ============================================================
-- Partner — quick templates
-- ============================================================

-- name: ListQuickTemplates :many
SELECT id, title, category, content, created_by, created_at, updated_at
FROM quick_templates
ORDER BY id DESC;

-- name: CreateQuickTemplate :one
INSERT INTO quick_templates (title, category, content, created_by, created_at, updated_at)
VALUES ($1, $2, $3, $4, NOW(), NOW())
RETURNING id, title, category, content, created_by, created_at, updated_at;

-- name: UpdateQuickTemplate :exec
UPDATE quick_templates
SET title = $1, category = $2, content = $3, updated_at = NOW()
WHERE id = $4;

-- name: DeleteQuickTemplate :exec
DELETE FROM quick_templates WHERE id = $1;

-- ============================================================
-- Partner — role permissions (RBAC)
-- ============================================================

-- name: ListRolePermissions :many
SELECT id, role_name, feature_key, permission_level, can_view, can_edit, updated_at
FROM role_permissions
ORDER BY role_name, feature_key;

-- name: UpsertRolePermission :exec
INSERT INTO role_permissions (role_name, feature_key, permission_level, can_view, can_edit, updated_at)
VALUES ($1, $2, $3, $4, $5, NOW())
ON CONFLICT (role_name, feature_key) DO UPDATE
SET permission_level = EXCLUDED.permission_level,
    can_view         = EXCLUDED.can_view,
    can_edit         = EXCLUDED.can_edit,
    updated_at       = NOW();

-- ============================================================
-- Partner — audit logs
-- ============================================================

-- name: InsertAuditLog :one
INSERT INTO system_audit_logs (action_type, details, performed_by, created_at)
VALUES ($1, $2, $3, NOW())
RETURNING id, action_type, details, performed_by, created_at;

-- name: ListAuditLogs :many
SELECT id, action_type, details, performed_by, created_at
FROM system_audit_logs
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- ============================================================
-- Partner — system errors (auto-purge after 30 days)
-- ============================================================

-- name: PurgeOldSystemErrors :exec
DELETE FROM system_errors WHERE created_at < NOW() - INTERVAL '30 days';

-- name: UpsertSystemError :exec
INSERT INTO system_errors (id, source, title, details, severity, is_handled, suggested_fix, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (id) DO UPDATE SET is_handled = EXCLUDED.is_handled;

-- name: ListSystemErrors :many
SELECT id, source, title, details, severity, is_handled, suggested_fix, created_at
FROM system_errors
ORDER BY created_at DESC;

-- name: MarkSystemErrorHandled :exec
UPDATE system_errors SET is_handled = TRUE WHERE id = $1;
