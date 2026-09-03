-- +goose Up
-- +goose StatementBegin
-- NOTE: w_system_configs is owned by the admin plugin; do not DDL it here.
-- Upsert keeps the admin-owned schema columns only.
-- Upsert audio/video extensions into the allowed extensions config.
INSERT INTO w_system_configs (key, value, type, description)
VALUES ('upload_allowed_extensions', 'jpg,png,webp,mp3,wav,m4a,flac,aac,ogg,webm,mp4,mkv,avi,mov,flv', 'system', '允许上传的文件扩展名（逗号分隔）')
ON CONFLICT (key) DO UPDATE SET
    value      = 'jpg,png,webp,mp3,wav,m4a,flac,aac,ogg,webm,mp4,mkv,avi,mov,flv',
    updated_at = CURRENT_TIMESTAMP;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
UPDATE w_system_configs
SET value      = 'jpg,png,webp',
    updated_at = CURRENT_TIMESTAMP
WHERE key = 'upload_allowed_extensions';
-- +goose StatementEnd
