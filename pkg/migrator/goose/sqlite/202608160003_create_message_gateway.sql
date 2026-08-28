-- +goose Up
CREATE TABLE IF NOT EXISTS w_message_channels (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    owner_scope TEXT NOT NULL DEFAULT 'system',
    owner_id INTEGER NULL,
    enabled INTEGER NOT NULL DEFAULT 1,
    credentials TEXT NOT NULL DEFAULT '',
    extra TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_w_message_channels_type ON w_message_channels (type);

CREATE TABLE IF NOT EXISTS w_message_bindings (
    id INTEGER PRIMARY KEY,
    user_id INTEGER NOT NULL,
    channel_id INTEGER NOT NULL,
    platform_user_id TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS uniq_w_message_bindings_channel_platform
    ON w_message_bindings (channel_id, platform_user_id);
CREATE INDEX IF NOT EXISTS idx_w_message_bindings_user ON w_message_bindings (user_id);

CREATE TABLE IF NOT EXISTS w_message_pairing_codes (
    code TEXT PRIMARY KEY,
    channel_id INTEGER NOT NULL,
    platform_user_id TEXT NOT NULL,
    expires_at DATETIME NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_w_message_pairing_lookup
    ON w_message_pairing_codes (channel_id, platform_user_id);

-- +goose Down
DROP TABLE IF EXISTS w_message_pairing_codes;
DROP TABLE IF EXISTS w_message_bindings;
DROP TABLE IF EXISTS w_message_channels;
