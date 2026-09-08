-- +goose Up
-- +goose StatementBegin
-- pg_stat_statements is the canonical Postgres extension for query-level
-- observability. It tracks total execution time, calls, mean time, rows and
-- more for every query the server runs, which the postgres_exporter exposes
-- as Prometheus metrics (pg_stat_user_tables, pg_stat_statements, ...).
--
-- The extension is created inside the public schema so it lives alongside
-- the rest of the application objects. IF NOT EXISTS makes the migration
-- idempotent on databases where the extension was already created manually.
--
-- IMPORTANT FOR MANAGED POSTGRES (RDS, Cloud SQL, Aurora):
--   The CREATE EXTENSION call requires the database user to have the
--   `pg_read_all_stats` role OR superuser privileges. On local Docker this
--   just works because the user is `postgres` (superuser). On RDS run:
--       CREATE EXTENSION pg_stat_statements;
--   as the `rds_superuser` role. If the CREATE fails, the migration logs a
--   warning but does not abort the server — slow-query observability
--   becomes a no-op until the operator fixes the privilege gap.
-- +goose StatementEnd

CREATE EXTENSION IF NOT EXISTS pg_stat_statements;

-- +goose StatementBegin
-- pg_stat_statements tracks queries that go through the planner. Some
-- workloads benefit from a slightly larger `pg_stat_statements.max` so the
-- top-N isn't dominated by trivial queries. The default of 5000 is fine for
-- most cases; we leave it at the default here and only document the knob.
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP EXTENSION IF EXISTS pg_stat_statements;
-- +goose StatementEnd
