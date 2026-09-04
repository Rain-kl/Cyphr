-- +goose Up
-- +goose StatementBegin
ALTER TABLE t_nodes ADD COLUMN work_mode VARCHAR(16) NOT NULL DEFAULT 'gpu';
ALTER TABLE t_nodes ADD COLUMN allow_auto_load BOOLEAN NOT NULL DEFAULT 1;
ALTER TABLE t_nodes ADD COLUMN auto_unload_minutes INT NOT NULL DEFAULT 0;
ALTER TABLE t_nodes ADD COLUMN model_vram_estimates TEXT NOT NULL DEFAULT '{}';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE t_nodes DROP COLUMN work_mode;
ALTER TABLE t_nodes DROP COLUMN allow_auto_load;
ALTER TABLE t_nodes DROP COLUMN auto_unload_minutes;
ALTER TABLE t_nodes DROP COLUMN model_vram_estimates;
-- +goose StatementEnd
