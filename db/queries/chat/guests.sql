-- ============================================================
-- Guests (customer pre-chat)
-- ============================================================

-- name: CreateGuest :one
INSERT INTO guests (guest_id, display_name, phone, created_at)
VALUES ($1, $2, $3, NOW())
RETURNING id, guest_id, display_name, phone, created_at;

-- name: GetGuestByID :one
SELECT id, guest_id, display_name, phone, created_at
FROM guests
WHERE guest_id = $1;

-- name: UpdateGuest :exec
UPDATE guests
SET display_name = $2, phone = $3
WHERE guest_id = $1;

-- name: DeleteGuest :exec
DELETE FROM guests WHERE guest_id = $1;

-- name: ListGuestsWithLastCase :many
SELECT
    g.id,
    g.guest_id::text                              AS guest_id,
    g.display_name,
    COALESCE(g.phone, '')                         AS phone,
    COALESCE(c.session_id, '')                    AS last_session_id,
    COALESCE(c.last_message, '')                  AS last_message,
    COALESCE(c.status::text, '')                  AS last_status,
    g.created_at,
    COALESCE(c.updated_at, g.created_at)          AS updated_at
FROM guests g
LEFT JOIN LATERAL (
    SELECT session_id, last_message, status, updated_at
    FROM chat_cases
    WHERE guest_id = g.guest_id
       OR (g.display_name <> '' AND customer_name = g.display_name)
    ORDER BY updated_at DESC
    LIMIT 1
) c ON true
ORDER BY g.created_at DESC;

-- name: SyncActiveCasesForGuest :exec
UPDATE chat_cases
SET customer_name = $2, customer_phone = $3, updated_at = NOW()
WHERE guest_id = $1;
