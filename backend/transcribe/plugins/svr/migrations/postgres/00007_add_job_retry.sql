-- +goose Up
ALTER TABLE t_jobs ADD COLUMN retry_count INT NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE t_jobs DROP COLUMN retry_count;
