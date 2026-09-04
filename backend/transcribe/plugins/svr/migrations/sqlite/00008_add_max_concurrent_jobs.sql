-- +goose Up
-- +goose StatementBegin
ALTER TABLE t_nodes ADD COLUMN max_concurrent_jobs INT NOT NULL DEFAULT 2;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE t_nodes DROP COLUMN max_concurrent_jobs;
-- +goose StatementEnd
