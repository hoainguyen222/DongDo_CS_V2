-- ============================================================
-- Auth sessions
-- ============================================================

-- name: CreateSession :one
INSERT INTO sessions (token, username, created_at, expires_at)
VALUES ($1, $2, NOW(), $3)
RETURNING id, token, username, created_at, expires_at;

-- name: VerifySession :one
SELECT s.username, u.full_name, u.role
FROM sessions s
JOIN users u ON s.username = u.username
WHERE s.token = $1 AND s.expires_at > NOW() AND u.is_active = TRUE;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE token = $1;

-- name: DeleteExpiredSessions :exec
DELETE FROM sessions WHERE expires_at < NOW();

-- name: DeleteUserSessions :exec
DELETE FROM sessions WHERE username = $1;
