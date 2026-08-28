-- +goose Up
CREATE TABLE IF NOT EXISTS w_task_executions (
    id BIGINT PRIMARY KEY,
    task_id VARCHAR(128) NOT NULL UNIQUE,
    task_type VARCHAR(64) NOT NULL,
    task_name VARCHAR(128),
    status VARCHAR(32) NOT NULL,
    retryable BOOLEAN NOT NULL DEFAULT FALSE,
    max_retry INTEGER NOT NULL DEFAULT 0,
    retry_count INTEGER NOT NULL DEFAULT 0,
    log TEXT,
    error_message TEXT,
    result TEXT,
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    duration BIGINT,
    payload TEXT,
    triggered_by VARCHAR(32) NOT NULL DEFAULT 'system',
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_w_task_executions_task_type ON w_task_executions (task_type);
CREATE INDEX IF NOT EXISTS idx_w_task_executions_status ON w_task_executions (status);
CREATE INDEX IF NOT EXISTS idx_w_task_executions_started_at ON w_task_executions (started_at);
CREATE INDEX IF NOT EXISTS idx_w_task_executions_created_at ON w_task_executions (created_at);

-- +goose Down
DROP TABLE IF EXISTS w_task_executions;