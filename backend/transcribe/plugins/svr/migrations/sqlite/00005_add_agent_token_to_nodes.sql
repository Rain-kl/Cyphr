-- +goose Up
-- +goose StatementBegin
ALTER TABLE t_nodes ADD COLUMN agent_token VARCHAR(128) NOT NULL DEFAULT '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE t_nodes DROP COLUMN agent_token;
-- +goose StatementEnd
