-- +goose Up
ALTER TABLE w_push_events ADD COLUMN IF NOT EXISTS task_type VARCHAR(100) NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_w_push_events_task_type ON w_push_events(task_type);

-- +goose Down
DROP INDEX IF EXISTS idx_w_push_events_task_type;
ALTER TABLE w_push_events DROP COLUMN IF EXISTS task_type;