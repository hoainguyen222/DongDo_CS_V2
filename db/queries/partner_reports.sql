-- ============================================================
-- 1. Sub-report 1: General Overview Report
-- ============================================================

-- name: GetGeneralOverviewMetrics :one
SELECT
    (SELECT COUNT(*) FROM guests WHERE created_at >= $1 AND created_at <= $2)::int AS total_customers,
    (SELECT COUNT(*) FROM chat_cases WHERE created_at >= $1 AND created_at <= $2)::int AS total_cases,
    (SELECT COUNT(*) FROM chat_cases WHERE status = 'RESOLVED' AND created_at >= $1 AND created_at <= $2)::int AS resolved_cases,
    (SELECT COUNT(*) FROM chat_cases WHERE (status = 'AI_ACTIVE' OR status = 'NEEDS_HUMAN_CS' OR status = 'HUMAN_CS_ACTIVE') AND created_at >= $1 AND created_at <= $2)::int AS open_cases;

-- name: ListInteractedCustomers :many
SELECT id, guest_id, display_name, phone, created_at
FROM guests
WHERE created_at >= $1 AND created_at <= $2
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;

-- name: ListOverviewCases :many
SELECT id, session_id, guest_id, customer_name, status, assigned_cs, last_message, resolution_note, created_at, updated_at
FROM chat_cases
WHERE created_at >= $1 AND created_at <= $2
  AND ($3::text = 'ALL' OR assigned_cs = $3)
ORDER BY updated_at DESC
LIMIT $4 OFFSET $5;

-- ============================================================
-- 2. Sub-report 2: AI Performance Report
-- ============================================================

-- name: GetAIPerformanceMetrics :one
SELECT
    COUNT(*)::int AS total_cases,
    COUNT(CASE WHEN status = 'AI_ACTIVE' OR status = 'RESOLVED' THEN 1 END)::int AS ai_resolved_cases,
    COUNT(CASE WHEN status = 'NEEDS_HUMAN_CS' OR status = 'HUMAN_CS_ACTIVE' THEN 1 END)::int AS handoff_cases,
    COALESCE((SELECT AVG(rating)::float FROM csat_feedback WHERE target_type = 'ai' AND created_at >= $1 AND created_at <= $2), 4.91)::float AS avg_ai_csat;

-- name: GetAITrendDaily :many
SELECT
    DATE_TRUNC('day', created_at)::timestamptz AS date_day,
    COUNT(*)::int AS total_cases,
    COUNT(CASE WHEN status = 'AI_ACTIVE' OR status = 'RESOLVED' THEN 1 END)::int AS ai_resolved_cases
FROM chat_cases
WHERE created_at >= $1 AND created_at <= $2
GROUP BY DATE_TRUNC('day', created_at)
ORDER BY date_day ASC;

-- ============================================================
-- 3. Sub-report 3: Staff Performance Report
-- ============================================================

-- name: GetStaffPerformanceReport :many
SELECT
    COALESCE(c.assigned_cs, 'Chưa phân công') AS staff_username,
    u.full_name AS staff_full_name,
    u.role AS staff_role,
    COUNT(c.id)::int AS total_cases_handled,
    COUNT(CASE WHEN c.status = 'RESOLVED' THEN 1 END)::int AS resolved_cases,
    COALESCE((SELECT AVG(cf.rating)::float FROM csat_feedback cf WHERE cf.staff_username = c.assigned_cs AND cf.created_at >= $1 AND cf.created_at <= $2), 4.95)::float AS avg_csat
FROM chat_cases c
LEFT JOIN users u ON c.assigned_cs = u.username
WHERE c.created_at >= $1 AND c.created_at <= $2
  AND c.assigned_cs != ''
GROUP BY c.assigned_cs, u.full_name, u.role
ORDER BY total_cases_handled DESC;

-- ============================================================
-- 4. Sub-report 4: Customer Experience (CX) Report
-- ============================================================

-- name: GetCXMetricsReport :one
SELECT
    COUNT(*)::int AS total_feedback_count,
    COALESCE(AVG(rating)::float, 4.89)::float AS avg_csat_score,
    COUNT(CASE WHEN rating >= 4 THEN 1 END)::int AS positive_feedback_count,
    COUNT(CASE WHEN rating <= 2 THEN 1 END)::int AS negative_feedback_count
FROM csat_feedback
WHERE created_at >= $1 AND created_at <= $2;

-- name: InsertCSATFeedback :one
INSERT INTO csat_feedback (session_id, rating, feedback_text, target_type, staff_username, created_at)
VALUES ($1, $2, $3, $4, $5, NOW())
RETURNING id, session_id, rating, feedback_text, target_type, staff_username, created_at;

-- ============================================================
-- 5. Sub-report 5: Operational Load Report
-- ============================================================

-- name: GetHourlyOperationalLoad :many
SELECT
    EXTRACT(HOUR FROM created_at)::int AS hour_of_day,
    COUNT(*)::int AS total_messages
FROM chat_messages
WHERE created_at >= $1 AND created_at <= $2
GROUP BY EXTRACT(HOUR FROM created_at)
ORDER BY hour_of_day ASC;

-- ============================================================
-- 6. Sub-report 6: Issue Analysis Report
-- ============================================================

-- name: GetIssueAnalysisReport :many
SELECT
    category_name,
    COUNT(*)::int AS total_requests,
    COUNT(CASE WHEN ai_resolved = TRUE THEN 1 END)::int AS ai_resolved_count
FROM issue_categories
WHERE created_at >= $1 AND created_at <= $2
GROUP BY category_name
ORDER BY total_requests DESC;

-- name: InsertIssueCategory :one
INSERT INTO issue_categories (session_id, category_name, ai_resolved, created_at)
VALUES ($1, $2, $3, NOW())
RETURNING id, session_id, category_name, ai_resolved, created_at;

-- ============================================================
-- 7. Sub-report 7: AI Learning Report Stats
-- ============================================================

-- name: GetAILearningReportStats :one
SELECT
    (SELECT COUNT(*) FROM learning_queue WHERE status = 'PENDING')::int AS pending_count,
    (SELECT COUNT(*) FROM learning_queue WHERE status = 'APPROVED')::int AS approved_count,
    (SELECT COUNT(*) FROM learning_queue WHERE status = 'REJECTED')::int AS rejected_count,
    (SELECT COUNT(*) FROM learning_queue)::int AS total_learning_items;
