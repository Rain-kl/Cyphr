-- +goose Up
ALTER TABLE w_access_tokens ADD COLUMN IF NOT EXISTS is_admin BOOLEAN NOT NULL DEFAULT FALSE;

-- +goose Down
ALTER TABLE w_access_tokens DROP COLUMN IF EXISTS is_admin;