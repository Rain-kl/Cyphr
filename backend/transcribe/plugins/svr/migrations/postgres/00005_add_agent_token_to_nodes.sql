-- +goose Up
-- +goose StatementBegin
ALTER TABLE t_nodes ADD COLUMN IF NOT EXISTS agent_token VARCHAR(128) NOT NULL DEFAULT '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE t_nodes DROP COLUMN IF EXISTS agent_token;
-- +goose StatementEnd
