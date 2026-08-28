-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS w_uploads (
    id BIGINT PRIMARY KEY,
    user_id BIGINT NOT NULL,
    file_name VARCHAR(255) NOT NULL,
    file_path VARCHAR(500) NOT NULL,
    file_size BIGINT NOT NULL,
    mime_type VARCHAR(100) NOT NULL,
    extension VARCHAR(50) NOT NULL,
    hash VARCHAR(64),
    type VARCHAR(50) NOT NULL,
    status VARCHAR(20) NOT NULL,
    access_mode INTEGER NOT NULL DEFAULT 0,
    metadata JSONB,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_w_uploads_user_id ON w_uploads (user_id);
CREATE INDEX IF NOT EXISTS idx_w_uploads_file_path ON w_uploads (file_path);
CREATE INDEX IF NOT EXISTS idx_w_uploads_hash ON w_uploads (hash);
CREATE INDEX IF NOT EXISTS idx_w_uploads_type ON w_uploads (type);
CREATE INDEX IF NOT EXISTS idx_w_uploads_status_created_at ON w_uploads (status, created_at);
CREATE INDEX IF NOT EXISTS idx_w_uploads_hash_file_size_status ON w_uploads (hash, file_size, status);

CREATE TABLE IF NOT EXISTS w_upload_stats (
    dimension VARCHAR(32) NOT NULL,
    stat_key VARCHAR(64) NOT NULL DEFAULT '',
    file_count BIGINT NOT NULL DEFAULT 0,
    file_size BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (dimension, stat_key)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS w_upload_stats;
DROP TABLE IF EXISTS w_uploads;
-- +goose StatementEnd
