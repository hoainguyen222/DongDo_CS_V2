-- ============================================================
-- Partner — dashboard KPIs & trend
-- ============================================================

-- name: GetDashboardKpiSummary :one
SELECT
    (SELECT COUNT(*) FROM chat_cases cc WHERE cc.created_at >= $1 AND cc.created_at <= $2)::int AS total_conversations,
    (SELECT COUNT(*) FROM chat_cases cc
        WHERE (cc.status = 'AI_ACTIVE' OR cc.status = 'RESOLVED') AND cc.created_at >= $1 AND cc.created_at <= $2)::int AS ai_resolved_count,
    (SELECT COUNT(*) FROM chat_cases cc
        WHERE cc.status = 'NEEDS_HUMAN_CS' AND cc.created_at >= $1 AND cc.created_at <= $2)::int AS human_handoff_count,
    COALESCE((SELECT AVG(cf.rating)::float FROM csat_feedback cf
        WHERE cf.created_at >= $1 AND cf.created_at <= $2), 4.9)::float AS avg_csat;

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
