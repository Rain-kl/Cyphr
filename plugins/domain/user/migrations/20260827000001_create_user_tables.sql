-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS w_users (
    id BIGINT PRIMARY KEY,
    username VARCHAR(64) NOT NULL UNIQUE,
    password VARCHAR(255),
    nickname VARCHAR(255),
    email VARCHAR(255),
    avatar_url VARCHAR(255),
    is_active BOOLEAN DEFAULT TRUE,
    is_admin BOOLEAN DEFAULT FALSE,
    bio VARCHAR(500),
    phone VARCHAR(32),
    gender VARCHAR(16),
    website VARCHAR(255),
    location VARCHAR(255),
    last_login_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_w_users_email ON w_users (email);
CREATE INDEX IF NOT EXISTS idx_w_users_is_active ON w_users (is_active);
CREATE INDEX IF NOT EXISTS idx_w_users_last_login_at ON w_users (last_login_at);
CREATE INDEX IF NOT EXISTS idx_w_users_created_at ON w_users (created_at);

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
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS w_access_tokens;
DROP TABLE IF EXISTS w_users;
-- +goose StatementEnd
