-- ============================================================
-- Learning queue (knowledge extraction)
-- ============================================================

-- name: AddToLearningQueue :one
INSERT INTO learning_queue (session_id, question, answer, status, created_by, created_at)
VALUES ($1, $2, $3, $4, $5, NOW())
RETURNING id, session_id, question, answer, status, created_by, approved_by, created_at, approved_at;

-- name: ListLearningByStatus :many
SELECT id, session_id, question, answer, status, created_by, approved_by, created_at, approved_at
FROM learning_queue
WHERE status = $1::learn_status
ORDER BY id DESC;

-- name: ListAllLearning :many
SELECT id, session_id, question, answer, status, created_by, approved_by, created_at, approved_at
FROM learning_queue
ORDER BY id DESC;

-- name: GetLearningItem :one
SELECT id, session_id, question, answer, status, created_by, approved_by, created_at, approved_at
FROM learning_queue
WHERE id = $1;

-- name: UpdateLearningContent :exec
UPDATE learning_queue SET question = $1, answer = $2 WHERE id = $3;

-- name: MarkLearningStatus :exec
UPDATE learning_queue
SET status = $1::learn_status, approved_by = $2, approved_at = NOW()
WHERE id = $3;

-- name: DeleteSessionLearning :exec
DELETE FROM learning_queue WHERE session_id = $1 AND status = 'PENDING'::learn_status;

-- name: ClearAllLearning :exec
DELETE FROM learning_queue;
