-- +goose Up
ALTER TABLE w_access_tokens DROP COLUMN IF EXISTS last_used_at;

-- +goose Down
ALTER TABLE w_access_tokens ADD COLUMN last_used_at TIMESTAMPTZ;