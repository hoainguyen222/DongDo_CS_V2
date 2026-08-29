-- name: GetAnalyticsStats :one
SELECT
    (SELECT COUNT(*) FROM chat_cases)::int AS total_cases,
    (SELECT COUNT(DISTINCT session_id) FROM chat_messages)::int AS total_sessions,
    (SELECT COUNT(*) FROM chat_cases WHERE status = 'AI_ACTIVE')::int AS ai_active_cases,
    (SELECT COUNT(*) FROM chat_cases WHERE status = 'NEEDS_HUMAN_CS')::int AS needs_human_cases,
    (SELECT COUNT(*) FROM chat_cases WHERE status = 'HUMAN_CS_ACTIVE')::int AS active_human_cases,
    (SELECT COUNT(*) FROM chat_cases WHERE status = 'RESOLVED')::int AS resolved_cases,
    (SELECT COUNT(*) FROM learning_queue WHERE status = 'APPROVED')::int AS total_learned_qa,
    (SELECT COUNT(*) FROM learning_queue WHERE status = 'PENDING')::int AS pending_learn_count;
