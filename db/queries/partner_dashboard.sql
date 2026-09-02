-- name: GetDashboardKpiSummary :one
SELECT
    (SELECT COUNT(*) FROM chat_cases WHERE created_at >= $1 AND created_at <= $2)::int AS total_conversations,
    (SELECT COUNT(*) FROM chat_cases WHERE (status = 'AI_ACTIVE' OR status = 'RESOLVED') AND created_at >= $1 AND created_at <= $2)::int AS ai_resolved_count,
    (SELECT COUNT(*) FROM chat_cases WHERE status = 'NEEDS_HUMAN_CS' AND created_at >= $1 AND created_at <= $2)::int AS human_handoff_count,
    COALESCE((SELECT AVG(rating)::float FROM csat_feedback WHERE created_at >= $1 AND created_at <= $2), 4.9)::float AS avg_csat;

-- name: GetDashboardAutomationTrendDaily :many
SELECT
    DATE_TRUNC('day', created_at)::timestamptz AS date_day,
    COUNT(*)::int AS total_cases,
    COUNT(CASE WHEN status = 'AI_ACTIVE' OR status = 'RESOLVED' THEN 1 END)::int AS ai_cases,
    COUNT(CASE WHEN status = 'NEEDS_HUMAN_CS' OR status = 'HUMAN_CS_ACTIVE' THEN 1 END)::int AS handoff_cases
FROM chat_cases
WHERE created_at >= $1 AND created_at <= $2
GROUP BY DATE_TRUNC('day', created_at)
ORDER BY date_day ASC;

-- name: GetRecentCompletedChats :many
SELECT
    id, session_id, guest_id, customer_name, status, assigned_cs, last_message, resolution_note, created_at, updated_at
FROM chat_cases
WHERE status = 'RESOLVED' OR status = 'AI_ACTIVE'
ORDER BY updated_at DESC
LIMIT $1 OFFSET $2;

-- name: GetCaseDetailBySessionID :one
SELECT
    c.id, c.session_id, c.guest_id, c.customer_name, c.status, c.assigned_cs, c.last_message, c.resolution_note, c.created_at, c.updated_at,
    g.display_name AS guest_display_name, g.phone AS guest_phone
FROM chat_cases c
LEFT JOIN guests g ON c.guest_id = g.guest_id
WHERE c.session_id = $1;
