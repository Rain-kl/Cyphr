-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS t_models (
    id          BIGINT PRIMARY KEY,
    name        VARCHAR(64) UNIQUE NOT NULL,
    task_type   VARCHAR(32) NOT NULL DEFAULT 'asr',
    description TEXT,
    is_active   BOOLEAN NOT NULL DEFAULT 1,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS t_nodes (
    id           BIGINT PRIMARY KEY,
    name         VARCHAR(64) NOT NULL,
    token_hash   VARCHAR(64) UNIQUE NOT NULL,
    token_prefix VARCHAR(16) NOT NULL,
    is_active    BOOLEAN NOT NULL DEFAULT 1,
    last_ip      VARCHAR(45),
    last_seen_at DATETIME,
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS t_jobs (
    id                  BIGINT PRIMARY KEY,
    user_id             BIGINT NOT NULL,
    node_id             BIGINT,
    model_name          VARCHAR(64) NOT NULL,
    task_type           VARCHAR(32) NOT NULL DEFAULT 'asr',
    status              VARCHAR(32) NOT NULL DEFAULT 'pending',
    progress            INT NOT NULL DEFAULT 0,
    audio_storage_path  TEXT NOT NULL,
    original_file_name  VARCHAR(255) NOT NULL,
    duration_seconds    REAL NOT NULL DEFAULT 0,
    result_text         TEXT,
    result_json         TEXT,
    error_msg           TEXT,
    created_at          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    started_at          DATETIME,
    completed_at        DATETIME,
    updated_at          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_t_jobs_user_status ON t_jobs(user_id, status);

CREATE TABLE IF NOT EXISTS t_job_logs (
    id         BIGINT PRIMARY KEY,
    job_id     BIGINT NOT NULL,
    seq        INT NOT NULL,
    progress   INT NOT NULL DEFAULT 0,
    message    TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_t_job_logs_job_id_seq ON t_job_logs(job_id, seq);

-- Seed initial model
INSERT INTO t_models (id, name, task_type, description, is_active, created_at, updated_at)
VALUES (1, 'mock-whisper-base', 'asr', 'Mock ASR base model for testing and demonstration', 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (id) DO NOTHING;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS t_job_logs;
DROP TABLE IF EXISTS t_jobs;
DROP TABLE IF EXISTS t_nodes;
DROP TABLE IF EXISTS t_models;
-- +goose StatementEnd
