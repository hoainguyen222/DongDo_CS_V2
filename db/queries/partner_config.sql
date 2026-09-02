-- ============================================================
-- 1. System Prompt & LLM Model Configuration Queries
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
-- 2. User & Role Management Queries
-- ============================================================

-- name: ListUsersByRoleAndSearch :many
SELECT id, username, full_name, role, is_active, created_at
FROM users
WHERE ($1::text = 'ALL' OR role::text = $1)
  AND ($2::text = '' OR username ILIKE '%' || $2 || '%' OR full_name ILIKE '%' || $2 || '%')
ORDER BY id ASC;

-- ============================================================
-- 3. Quick Templates (Tin nhắn mẫu CSKH) Queries
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
-- 4. Role Permissions Matrix (Phân quyền RBAC) Queries
-- ============================================================

-- name: ListRolePermissions :many
SELECT id, role_name, feature_key, can_view, can_edit, updated_at
FROM role_permissions
ORDER BY role_name, feature_key;

-- name: UpsertRolePermission :exec
INSERT INTO role_permissions (role_name, feature_key, can_view, can_edit, updated_at)
VALUES ($1, $2, $3, $4, NOW())
ON CONFLICT (role_name, feature_key) DO UPDATE
SET can_view = EXCLUDED.can_view, can_edit = EXCLUDED.can_edit, updated_at = NOW();

-- ============================================================
-- 5. Audit Log & System Maintenance Queries
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
