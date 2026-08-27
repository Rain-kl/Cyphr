-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS w_system_configs (
    key VARCHAR(64) PRIMARY KEY,
    value TEXT NOT NULL,
    type VARCHAR(32) NOT NULL DEFAULT 'system',
    visibility INTEGER NOT NULL DEFAULT 0,
    description VARCHAR(255),
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS w_templates (
    id BIGINT PRIMARY KEY,
    key VARCHAR(80) NOT NULL UNIQUE,
    name VARCHAR(100) NOT NULL,
    type VARCHAR(20) NOT NULL DEFAULT 'email',
    subject VARCHAR(255),
    content TEXT NOT NULL,
    description VARCHAR(255),
    is_system BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_w_templates_is_system ON w_templates (is_system);
CREATE INDEX IF NOT EXISTS idx_w_templates_created_at ON w_templates (created_at);
CREATE INDEX IF NOT EXISTS idx_w_templates_updated_at ON w_templates (updated_at);

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
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS w_task_executions;
DROP TABLE IF EXISTS w_schedules;
DROP TABLE IF EXISTS w_templates;
DROP TABLE IF EXISTS w_system_configs;
-- +goose StatementEnd
