-- name: UpsertCase :one
INSERT INTO chat_cases (session_id, guest_id, customer_name, status, assigned_cs, last_message, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
ON CONFLICT (session_id) DO UPDATE SET
    status = CASE
        WHEN chat_cases.status = 'HUMAN_CS_ACTIVE' AND EXCLUDED.status = 'NEEDS_HUMAN_CS' THEN chat_cases.status
        ELSE EXCLUDED.status
    END,
    last_message = COALESCE(NULLIF(EXCLUDED.last_message, ''), chat_cases.last_message),
    assigned_cs = COALESCE(NULLIF(EXCLUDED.assigned_cs, ''), chat_cases.assigned_cs),
    updated_at = NOW()
RETURNING id, session_id, guest_id, customer_name, status, assigned_cs, last_message, resolution_note, created_at, updated_at;

-- name: ListCases :many
SELECT id, session_id, guest_id, customer_name, status, assigned_cs, last_message, resolution_note, created_at, updated_at
FROM chat_cases
ORDER BY updated_at DESC;

-- name: ListCasesByStatus :many
SELECT id, session_id, guest_id, customer_name, status, assigned_cs, last_message, resolution_note, created_at, updated_at
FROM chat_cases
WHERE status = $1
ORDER BY updated_at DESC;

-- name: GetCase :one
SELECT id, session_id, guest_id, customer_name, status, assigned_cs, last_message, resolution_note, created_at, updated_at
FROM chat_cases
WHERE session_id = $1;

-- name: AssignCase :exec
UPDATE chat_cases SET status = 'HUMAN_CS_ACTIVE', assigned_cs = $1, updated_at = NOW()
WHERE session_id = $2;

-- name: ResolveCase :exec
UPDATE chat_cases SET status = 'RESOLVED', assigned_cs = $1, resolution_note = $2, updated_at = NOW()
WHERE session_id = $3;

-- name: DeleteCase :exec
DELETE FROM chat_cases WHERE session_id = $1;

-- name: DeleteAllCases :exec
DELETE FROM chat_cases;
