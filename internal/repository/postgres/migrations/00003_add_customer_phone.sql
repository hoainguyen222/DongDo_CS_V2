-- +goose Up
-- +goose StatementBegin
ALTER TABLE chat_cases
    ADD COLUMN IF NOT EXISTS customer_phone TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_cases_phone ON chat_cases(customer_phone);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_cases_phone;
ALTER TABLE chat_cases DROP COLUMN IF EXISTS customer_phone;
-- +goose StatementEnd
