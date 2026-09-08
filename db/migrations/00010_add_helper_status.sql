-- +goose Up
-- +goose StatementBegin
ALTER TABLE chat_cases
    ADD COLUMN IF NOT EXISTS helper_status VARCHAR(20) DEFAULT 'new' NOT NULL;

CREATE INDEX IF NOT EXISTS idx_cases_helper_status ON chat_cases(helper_status);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_cases_helper_status;
ALTER TABLE chat_cases DROP COLUMN IF EXISTS helper_status;
-- +goose StatementEnd
