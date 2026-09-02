-- name: CreateVoiceCall :one
INSERT INTO voice_calls (session_id, caller_type, caller_id, callee_type, callee_id, status, created_at)
VALUES ($1, $2, $3, $4, $5, 'RINGING', NOW())
RETURNING id, session_id, caller_type, caller_id, callee_type, callee_id, status, duration_seconds, recording_url, transcript, created_at, ended_at;

-- name: UpdateCallStatus :exec
UPDATE voice_calls SET status = $1, ended_at = NOW() WHERE id = $2;

-- name: EndCall :exec
UPDATE voice_calls SET status = 'ENDED', duration_seconds = $1, recording_url = $2, ended_at = NOW() WHERE id = $3;

-- name: SetCallTranscript :exec
UPDATE voice_calls SET transcript = $1 WHERE id = $2;

-- name: GetCallsBySession :many
SELECT id, session_id, caller_type, caller_id, callee_type, callee_id, status, duration_seconds, recording_url, transcript, created_at, ended_at
FROM voice_calls
WHERE session_id = $1
ORDER BY created_at DESC;

-- name: GetCallByID :one
SELECT id, session_id, caller_type, caller_id, callee_type, callee_id, status, duration_seconds, recording_url, transcript, created_at, ended_at
FROM voice_calls
WHERE id = $1;
