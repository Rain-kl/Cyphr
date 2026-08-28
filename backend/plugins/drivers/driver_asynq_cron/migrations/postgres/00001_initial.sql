-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS w_schedules (
    id BIGINT PRIMARY KEY,
    name VARCHAR(128) NOT NULL,
    task_type VARCHAR(64) NOT NULL,
    cron VARCHAR(64) NOT NULL,
    payload TEXT,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_w_schedules_is_active ON w_schedules (is_active);

-- Seed initial cleanup task
INSERT INTO w_schedules (id, name, task_type, cron, payload, is_active, created_at, updated_at)
VALUES (1, '系统定期垃圾清理', 'system_cleanup', '0 3 * * *', '{}', TRUE, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (id) DO NOTHING;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS w_schedules;
-- +goose StatementEnd
