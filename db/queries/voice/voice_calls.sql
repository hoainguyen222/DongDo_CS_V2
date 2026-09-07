-- ============================================================
-- Voice calls (WebRTC)
-- ============================================================

-- name: CreateVoiceCall :one
INSERT INTO voice_calls (session_id, caller_type, caller_id, callee_type, callee_id, status, created_at)
VALUES ($1, $2, $3, $4, $5, 'RINGING'::call_status, NOW())
RETURNING id, session_id, caller_type, caller_id, callee_type, callee_id,
          status, duration_seconds, recording_url, transcript, created_at, ended_at;

-- name: UpdateCallStatus :exec
UPDATE voice_calls SET status = $1::call_status, ended_at = NOW() WHERE id = $2;

-- name: EndCall :exec
UPDATE voice_calls
SET status = 'ENDED'::call_status,
    duration_seconds = $1,
    recording_url    = $2,
    ended_at         = NOW()
WHERE id = $3;

-- name: SetCallTranscript :exec
UPDATE voice_calls SET transcript = $1 WHERE id = $2;

-- name: GetCallsBySession :many
-- Use COALESCE with a derived status from ended_at so this works even on
-- legacy deployments where the `status` column was never created. The
-- synthetic status mirrors what the legacy migration 00005 backfill uses
-- ('IN_PROGRESS' while the call is still up, 'ENDED' otherwise). When
-- the column IS present we prefer it (the COALESCE returns it first).
SELECT id, session_id, caller_type, caller_id, callee_type, callee_id,
       COALESCE(status,
                CASE WHEN ended_at IS NULL THEN 'IN_PROGRESS'::call_status
                     ELSE 'ENDED'::call_status END) AS status,
       duration_seconds, recording_url, transcript, created_at, ended_at
FROM voice_calls
WHERE session_id = $1
ORDER BY created_at DESC;

-- name: ListAllCalls :many
-- Same defensive COALESCE — see GetCallsBySession.
SELECT id, session_id, caller_type, caller_id, callee_type, callee_id,
       COALESCE(status,
                CASE WHEN ended_at IS NULL THEN 'IN_PROGRESS'::call_status
                     ELSE 'ENDED'::call_status END) AS status,
       duration_seconds, recording_url, transcript, created_at, ended_at
FROM voice_calls
ORDER BY created_at DESC
LIMIT 100;

-- name: GetCallByID :one
SELECT id, session_id, caller_type, caller_id, callee_type, callee_id,
       COALESCE(status,
                CASE WHEN ended_at IS NULL THEN 'IN_PROGRESS'::call_status
                     ELSE 'ENDED'::call_status END) AS status,
       duration_seconds, recording_url, transcript, created_at, ended_at
FROM voice_calls
WHERE id = $1;

-- name: DeleteCall :exec
DELETE FROM voice_calls WHERE id = $1;

-- name: MarkMissedCall :exec
UPDATE voice_calls
SET status = 'MISSED'::call_status,
    duration_seconds = 0,
    ended_at         = NOW()
WHERE id = $1;
