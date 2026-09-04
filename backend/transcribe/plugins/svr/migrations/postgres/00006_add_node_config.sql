-- +goose Up
-- +goose StatementBegin
ALTER TABLE t_nodes ADD COLUMN IF NOT EXISTS work_mode VARCHAR(16) NOT NULL DEFAULT 'gpu';
ALTER TABLE t_nodes ADD COLUMN IF NOT EXISTS allow_auto_load BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE t_nodes ADD COLUMN IF NOT EXISTS auto_unload_minutes INT NOT NULL DEFAULT 0;
ALTER TABLE t_nodes ADD COLUMN IF NOT EXISTS model_vram_estimates TEXT NOT NULL DEFAULT '{}';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE t_nodes DROP COLUMN IF EXISTS work_mode;
ALTER TABLE t_nodes DROP COLUMN IF EXISTS allow_auto_load;
ALTER TABLE t_nodes DROP COLUMN IF EXISTS auto_unload_minutes;
ALTER TABLE t_nodes DROP COLUMN IF EXISTS model_vram_estimates;
-- +goose StatementEnd
