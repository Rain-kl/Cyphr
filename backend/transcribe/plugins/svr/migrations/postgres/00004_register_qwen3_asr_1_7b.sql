-- +goose Up
-- +goose StatementBegin
-- Register the local Qwen3-ASR-1.7B model (served by Python agent).
INSERT INTO t_models (id, name, task_type, description, is_active, created_at, updated_at)
VALUES (3, 'qwen3-asr-1.7b', 'asr', 'Qwen3-ASR 1.7B SOTA local inference (Python agent)', TRUE, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (id) DO NOTHING;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM t_models WHERE name = 'qwen3-asr-1.7b';
-- +goose StatementEnd
