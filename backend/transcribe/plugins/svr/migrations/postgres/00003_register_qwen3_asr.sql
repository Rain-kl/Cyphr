-- +goose Up
-- +goose StatementBegin
-- Register the local Qwen3-ASR-0.6B model (weights live in backend/agent/models/, served by the Python agent).
INSERT INTO t_models (id, name, task_type, description, is_active, created_at, updated_at)
VALUES (2, 'qwen3-asr-0.6b', 'asr', 'Qwen3-ASR 0.6B local inference (Python agent)', TRUE, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (id) DO NOTHING;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM t_models WHERE name = 'qwen3-asr-0.6b';
-- +goose StatementEnd
