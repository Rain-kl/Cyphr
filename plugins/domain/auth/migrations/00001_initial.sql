-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS w_auth_sources (
    id BIGINT PRIMARY KEY,
    name VARCHAR(80) NOT NULL UNIQUE,
    type VARCHAR(20) NOT NULL,
    display_name VARCHAR(100),
    is_active BOOLEAN NOT NULL DEFAULT FALSE,
    client_id VARCHAR(255),
    client_secret VARCHAR(1024),
    openid_discovery_url VARCHAR(1024),
    scopes VARCHAR(255),
    icon_url VARCHAR(1024),
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_w_auth_sources_is_active ON w_auth_sources (is_active);

CREATE TABLE IF NOT EXISTS w_external_accounts (
    id BIGINT PRIMARY KEY,
    auth_source_id BIGINT,
    user_id BIGINT NOT NULL,
    external_id VARCHAR(255) NOT NULL,
    external_username VARCHAR(255),
    email VARCHAR(255),
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_w_external_accounts_auth_source_id ON w_external_accounts (auth_source_id);
CREATE INDEX IF NOT EXISTS idx_w_external_accounts_user_id ON w_external_accounts (user_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_w_external_accounts_source_external ON w_external_accounts (auth_source_id, external_id);

CREATE TABLE IF NOT EXISTS w_access_tokens (
    id BIGINT PRIMARY KEY,
    user_id BIGINT NOT NULL,
    token_hash VARCHAR(64) NOT NULL UNIQUE,
    name VARCHAR(128) NOT NULL,
    description VARCHAR(255),
    is_admin BOOLEAN DEFAULT FALSE,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_w_access_tokens_user_id ON w_access_tokens (user_id);

-- Seed: login session TTL
INSERT INTO w_system_configs (key, value, type, visibility, description, created_at, updated_at)
VALUES ('login_session_ttl_hours', '0', 'system', 0, '登录会话过期时间 (小时，0表示浏览器关闭后自动退出，-1表示永不过期)', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (key) DO NOTHING;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM w_system_configs WHERE key = 'login_session_ttl_hours';
DROP TABLE IF EXISTS w_access_tokens;
DROP TABLE IF EXISTS w_external_accounts;
DROP TABLE IF EXISTS w_auth_sources;
-- +goose StatementEnd