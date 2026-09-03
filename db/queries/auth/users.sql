-- ============================================================
-- Users (CSKH Staff & Admin)
-- ============================================================

-- name: CreateUser :one
INSERT INTO users (username, password_hash, salt, full_name, role, is_active, created_at)
VALUES ($1, $2, $3, $4, $5, TRUE, NOW())
RETURNING id, username, full_name, role, is_active, created_at;

-- name: GetUserByUsername :one
SELECT id, username, password_hash, salt, full_name, role, is_active, created_at
FROM users
WHERE username = $1;

-- name: GetUserByUsernameInsensitive :one
SELECT id, username, password_hash, salt, full_name, role, is_active, created_at
FROM users
WHERE LOWER(username) = LOWER($1);

-- name: ListUsers :many
SELECT id, username, full_name, role, is_active, created_at
FROM users
ORDER BY id ASC;

-- name: ListUsersByRoleAndSearch :many
SELECT id, username, full_name, role, is_active, created_at
FROM users
WHERE ($1::text = 'ALL' OR role::text = $1)
  AND ($2::text = '' OR username ILIKE '%' || $2 || '%' OR full_name ILIKE '%' || $2 || '%')
ORDER BY id ASC;

-- name: DeleteUser :exec
DELETE FROM users WHERE username = $1;

-- name: DeleteUserByUsernameInsensitive :exec
DELETE FROM users WHERE LOWER(username) = LOWER($1);

-- name: UpdatePassword :exec
UPDATE users SET password_hash = $1, salt = $2 WHERE username = $3;

-- name: UpdatePasswordByUsernameInsensitive :exec
UPDATE users SET password_hash = $1, salt = $2 WHERE LOWER(username) = LOWER($3);

-- name: UpdateUserWithPassword :one
UPDATE users
SET full_name = $1, role = $2, is_active = $3, password_hash = $4, salt = $5
WHERE LOWER(username) = LOWER($6)
RETURNING id, username, full_name, role, is_active, created_at;

-- name: UpdateUserWithoutPassword :one
UPDATE users
SET full_name = $1, role = $2, is_active = $3
WHERE LOWER(username) = LOWER($4)
RETURNING id, username, full_name, role, is_active, created_at;

-- name: CountUsersByUsername :one
SELECT COUNT(*)::int FROM users WHERE username = $1;
