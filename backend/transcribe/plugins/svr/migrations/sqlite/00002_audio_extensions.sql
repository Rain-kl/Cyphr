-- +goose Up
-- +goose StatementBegin
-- NOTE: w_system_configs is owned by the admin plugin (row seeded there); only extend the value.
-- Extend the allowed extensions with audio/video formats.
UPDATE w_system_configs
SET value      = 'jpg,png,webp,mp3,wav,m4a,flac,aac,ogg,webm,mp4,mkv,avi,mov,flv',
    updated_at = CURRENT_TIMESTAMP
WHERE key = 'upload_allowed_extensions';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
UPDATE w_system_configs
SET value      = 'jpg,png,webp',
    updated_at = CURRENT_TIMESTAMP
WHERE key = 'upload_allowed_extensions';
-- +goose StatementEnd
