-- +goose Up
-- +goose StatementBegin
-- Ensure the platform system config table exists (idempotent guard for standalone environments).
CREATE TABLE IF NOT EXISTS w_system_configs (
    id          BIGSERIAL PRIMARY KEY,
    key         VARCHAR(128) UNIQUE NOT NULL,
    value       TEXT NOT NULL DEFAULT '',
    type        VARCHAR(32) NOT NULL DEFAULT 'system',
    is_public   INTEGER NOT NULL DEFAULT 0,
    description TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Upsert audio/video extensions into the allowed extensions config.
INSERT INTO w_system_configs (key, value, type, is_public, description)
VALUES ('upload_allowed_extensions', 'jpg,png,webp,mp3,wav,m4a,flac,aac,ogg,webm,mp4,mkv,avi,mov,flv', 'system', 1, '允许上传的文件扩展名（逗号分隔）')
ON CONFLICT (key) DO UPDATE SET
    value      = EXCLUDED.value,
    updated_at = CURRENT_TIMESTAMP;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
UPDATE w_system_configs
SET value      = 'jpg,png,webp',
    updated_at = CURRENT_TIMESTAMP
WHERE key = 'upload_allowed_extensions';
-- +goose StatementEnd
