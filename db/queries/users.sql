-- name: CreateUser :one
INSERT INTO users (username, password_hash, salt, full_name, role, is_active, created_at)
VALUES ($1, $2, $3, $4, $5, TRUE, NOW())
RETURNING id, username, full_name, role, is_active, created_at;

-- name: GetUserByUsername :one
SELECT id, username, password_hash, salt, full_name, role, is_active, created_at
FROM users WHERE username = $1;

-- name: ListUsers :many
SELECT id, username, full_name, role, is_active, created_at
FROM users ORDER BY id ASC;

-- name: DeleteUser :exec
DELETE FROM users WHERE username = $1;

-- name: UpdatePassword :exec
UPDATE users SET password_hash = $1, salt = $2 WHERE username = $3;
