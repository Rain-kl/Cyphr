-- +goose Up
ALTER TABLE t_jobs ADD COLUMN retry_count INT NOT NULL DEFAULT 0;

-- +goose Down
-- SQLite does not support DROP COLUMN prior to 3.35; leave column in place.
