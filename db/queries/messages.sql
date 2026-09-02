-- name: InsertMessage :one
INSERT INTO chat_messages (session_id, sender_type, sender_id, content, client_msg_id, is_learned, created_at)
VALUES ($1, $2, $3, $4, $5, FALSE, NOW())
ON CONFLICT (client_msg_id) WHERE client_msg_id IS NOT NULL DO NOTHING
RETURNING id, session_id, sender_type, sender_id, content, client_msg_id, is_learned, created_at;

-- name: GetSessionHistory :many
SELECT id, session_id, sender_type, sender_id, content, client_msg_id, is_learned, created_at
FROM chat_messages
WHERE session_id = $1
ORDER BY created_at ASC;

-- name: GetUnlearnedMessages :many
SELECT id, session_id, sender_type, sender_id, content, created_at
FROM chat_messages
WHERE is_learned = FALSE
ORDER BY session_id, created_at;

-- name: MarkMessagesLearned :exec
UPDATE chat_messages SET is_learned = TRUE WHERE id = ANY($1::bigint[]);

-- name: DeleteSessionMessages :exec
DELETE FROM chat_messages WHERE session_id = $1;

-- name: DeleteAllMessages :exec
DELETE FROM chat_messages;

-- name: ResetLearnedFlags :exec
UPDATE chat_messages SET is_learned = FALSE;
