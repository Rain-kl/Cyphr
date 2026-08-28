-- +goose Up
ALTER TABLE w_uploads DROP COLUMN IF EXISTS storage_driver;

-- +goose Down
ALTER TABLE w_uploads ADD COLUMN IF NOT EXISTS storage_driver VARCHAR(50) NOT NULL DEFAULT 'local';