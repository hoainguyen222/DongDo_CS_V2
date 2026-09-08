-- +goose Up
-- +goose StatementBegin
ALTER TABLE chat_cases 
ADD COLUMN IF NOT EXISTS active_assigned_cs TEXT NOT NULL DEFAULT '',
ADD COLUMN IF NOT EXISTS assigned_cs_history JSONB NOT NULL DEFAULT '[]'::jsonb,
ADD COLUMN IF NOT EXISTS requires_help BOOLEAN NOT NULL DEFAULT FALSE,
ADD COLUMN IF NOT EXISTS help_content TEXT NOT NULL DEFAULT '',
ADD COLUMN IF NOT EXISTS help_requested_by TEXT NOT NULL DEFAULT '',
ADD COLUMN IF NOT EXISTS help_requested_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_chat_cases_requires_help ON chat_cases(requires_help);
CREATE INDEX IF NOT EXISTS idx_chat_cases_active_cs ON chat_cases(active_assigned_cs);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE chat_cases
DROP COLUMN IF EXISTS active_assigned_cs,
DROP COLUMN IF EXISTS assigned_cs_history,
DROP COLUMN IF EXISTS requires_help,
DROP COLUMN IF EXISTS help_content,
DROP COLUMN IF EXISTS help_requested_by,
DROP COLUMN IF EXISTS help_requested_at;
-- +goose StatementEnd
