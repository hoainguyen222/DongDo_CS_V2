-- name: CreateGuest :one
INSERT INTO guests (guest_id, display_name, phone, created_at)
VALUES ($1, $2, $3, NOW())
RETURNING id, guest_id, display_name, phone, created_at;

-- name: GetGuestByID :one
SELECT id, guest_id, display_name, phone, created_at
FROM guests WHERE guest_id = $1;
