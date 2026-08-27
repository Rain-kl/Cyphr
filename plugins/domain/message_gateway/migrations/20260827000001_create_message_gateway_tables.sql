-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS w_message_channels (
    id BIGINT PRIMARY KEY,
    name VARCHAR(128) NOT NULL,
    type VARCHAR(32) NOT NULL,
    owner_scope VARCHAR(16) NOT NULL DEFAULT 'system',
    owner_id BIGINT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    credentials TEXT NOT NULL DEFAULT '',
    extra TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_w_message_channels_type ON w_message_channels (type);

CREATE TABLE IF NOT EXISTS w_message_bindings (
    id BIGINT PRIMARY KEY,
    user_id BIGINT NOT NULL,
    channel_id BIGINT NOT NULL,
    platform_user_id VARCHAR(128) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS uniq_w_message_bindings_channel_platform
    ON w_message_bindings (channel_id, platform_user_id);
CREATE INDEX IF NOT EXISTS idx_w_message_bindings_user ON w_message_bindings (user_id);

CREATE TABLE IF NOT EXISTS w_message_pairing_codes (
    code VARCHAR(16) PRIMARY KEY,
    channel_id BIGINT NOT NULL,
    platform_user_id VARCHAR(128) NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_w_message_pairing_lookup
    ON w_message_pairing_codes (channel_id, platform_user_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS w_message_pairing_codes;
DROP TABLE IF EXISTS w_message_bindings;
DROP TABLE IF EXISTS w_message_channels;
-- +goose StatementEnd
