-- ============================================================
-- Chat cases (CS inbox)
-- ============================================================

-- name: UpsertCase :one
INSERT INTO chat_cases (
    session_id, guest_id, customer_name, customer_phone,
    status, assigned_cs, last_message, created_at, updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
ON CONFLICT (session_id) DO UPDATE SET
    customer_name = CASE
        WHEN EXCLUDED.customer_name <> '' AND EXCLUDED.customer_name <> 'Khách hàng'
            THEN EXCLUDED.customer_name
        ELSE chat_cases.customer_name
    END,
    customer_phone = CASE
        WHEN EXCLUDED.customer_phone <> ''
            THEN EXCLUDED.customer_phone
        ELSE chat_cases.customer_phone
    END,
    status = CASE
        WHEN chat_cases.status = 'HUMAN_CS_ACTIVE' AND EXCLUDED.status = 'NEEDS_HUMAN_CS'::case_status
            THEN chat_cases.status
        ELSE EXCLUDED.status
    END,
    last_message = COALESCE(NULLIF(EXCLUDED.last_message, ''), chat_cases.last_message),
    assigned_cs  = COALESCE(NULLIF(EXCLUDED.assigned_cs, ''), chat_cases.assigned_cs),
    updated_at = NOW()
RETURNING id, session_id, guest_id, customer_name, customer_phone,
          status, assigned_cs, last_message, resolution_note, created_at, updated_at;

-- name: ListCases :many
SELECT id, session_id, guest_id, customer_name, customer_phone,
       status, assigned_cs, last_message, resolution_note, created_at, updated_at
FROM chat_cases
ORDER BY updated_at DESC;

-- name: ListCasesByStatus :many
SELECT id, session_id, guest_id, customer_name, customer_phone,
       status, assigned_cs, last_message, resolution_note, created_at, updated_at
FROM chat_cases
WHERE status = $1::case_status
ORDER BY updated_at DESC;

-- name: ListCasesPage :many
-- Paginated variant used by HandleListCases so we never load the full table
-- into Go memory before slicing. The shape is identical to ListCases; only
-- LIMIT/OFFSET differs. statusFilter empty string means "any status".
SELECT id, session_id, guest_id, customer_name, customer_phone,
       status, assigned_cs, last_message, resolution_note, created_at, updated_at
FROM chat_cases
WHERE ($1::text = '' OR status::text = $1)
ORDER BY updated_at DESC
LIMIT $2 OFFSET $3;

-- name: CountCases :one
SELECT COUNT(*)::bigint FROM chat_cases
WHERE ($1::text = '' OR status::text = $1);

-- name: SearchCasesPage :many
-- Search-and-paginate variant for HandleListCases. The OR-of-LIKE pattern
-- is acceptable for the small admin-inbox dataset; if it ever grows past
-- ~50k rows swap to a trigram index (pg_trgm).
SELECT id, session_id, guest_id, customer_name, customer_phone,
       status, assigned_cs, last_message, resolution_note, created_at, updated_at
FROM chat_cases
WHERE ($1::text = '' OR status::text = $1)
  AND (LOWER(customer_name)  LIKE '%' || LOWER($4) || '%'
    OR LOWER(customer_phone) LIKE '%' || $4          || '%'
    OR LOWER(session_id)     LIKE '%' || LOWER($4) || '%'
    OR LOWER(COALESCE(last_message, '')) LIKE '%' || LOWER($4) || '%')
ORDER BY updated_at DESC
LIMIT $2 OFFSET $3;

-- name: CountCasesSearch :one
SELECT COUNT(*)::bigint FROM chat_cases
WHERE ($1::text = '' OR status::text = $1)
  AND (LOWER(customer_name)  LIKE '%' || LOWER($2) || '%'
    OR LOWER(customer_phone) LIKE '%' || $2          || '%'
    OR LOWER(session_id)     LIKE '%' || LOWER($2) || '%'
    OR LOWER(COALESCE(last_message, '')) LIKE '%' || LOWER($2) || '%');

-- name: GetCase :one
SELECT id, session_id, guest_id, customer_name, customer_phone,
       status, assigned_cs, last_message, resolution_note, created_at, updated_at
FROM chat_cases
WHERE session_id = $1;

-- name: AssignCase :exec
UPDATE chat_cases
SET status = 'HUMAN_CS_ACTIVE'::case_status, assigned_cs = $1, updated_at = NOW()
WHERE session_id = $2;

-- name: ResolveCase :exec
UPDATE chat_cases
SET status = 'RESOLVED'::case_status,
    assigned_cs = $1,
    resolution_note = $2,
    updated_at = NOW()
WHERE session_id = $3;

-- name: DeleteCase :exec
DELETE FROM chat_cases WHERE session_id = $1;

-- name: DeleteAllCases :exec
DELETE FROM chat_cases;

-- name: GetRecentCompletedChats :many
SELECT id, session_id, guest_id, customer_name, customer_phone,
       status, assigned_cs, last_message, resolution_note, created_at, updated_at
FROM chat_cases
WHERE status IN ('RESOLVED'::case_status, 'AI_ACTIVE'::case_status)
ORDER BY updated_at DESC
LIMIT $1 OFFSET $2;

-- name: GetCaseDetailWithGuest :one
SELECT
    c.id, c.session_id, c.guest_id, c.customer_name, c.customer_phone,
    c.status, c.assigned_cs, c.last_message, c.resolution_note, c.created_at, c.updated_at,
    g.display_name AS guest_display_name,
    g.phone        AS guest_phone
FROM chat_cases c
LEFT JOIN guests g ON c.guest_id = g.guest_id
WHERE c.session_id = $1;
