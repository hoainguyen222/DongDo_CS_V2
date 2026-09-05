-- +goose Up
-- +goose StatementBegin
-- ============================================================
-- Migration: Extend call_status enum to support Asterisk lifecycle states
-- ============================================================
--
-- Adds DIALING, BRIDGED, FAILED values used by the AMI integration. Existing
-- values (RINGING, ACTIVE, ENDED, MISSED, REJECTED) are kept for backward
-- compatibility. ACTIVE is retained as an alias for BRIDGED and the
-- application layer is responsible for translating between them.

ALTER TYPE call_status ADD VALUE IF NOT EXISTS 'DIALING';
ALTER TYPE call_status ADD VALUE IF NOT EXISTS 'BRIDGED';
ALTER TYPE call_status ADD VALUE IF NOT EXISTS 'FAILED';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- PostgreSQL does not allow removing enum values directly. A manual
-- recreation of the type would be required, which is intentionally
-- omitted to keep downgrade safe in production.
SELECT 'down migration not supported for ALTER TYPE ... ADD VALUE' AS warning;
-- +goose StatementEnd
