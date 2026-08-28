-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS w_user_access_logs (
    id          BIGINT NOT NULL,
    user_id     BIGINT NOT NULL DEFAULT 0,
    path        VARCHAR(2048) NOT NULL DEFAULT '',
    method      VARCHAR(16) NOT NULL DEFAULT '',
    ip          VARCHAR(128) NOT NULL DEFAULT '',
    user_agent  TEXT NOT NULL DEFAULT '',
    headers     TEXT NOT NULL DEFAULT '',
    status      INTEGER NOT NULL DEFAULT 0,
    latency     BIGINT NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id, created_at)
);

CREATE INDEX IF NOT EXISTS idx_w_user_access_logs_user_id ON w_user_access_logs (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_w_user_access_logs_created_at ON w_user_access_logs (created_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS w_user_access_logs;
-- +goose StatementEnd
