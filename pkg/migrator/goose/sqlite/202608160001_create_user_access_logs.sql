-- +goose Up
CREATE TABLE IF NOT EXISTS w_user_access_logs (
    id          INTEGER NOT NULL,
    user_id     INTEGER NOT NULL DEFAULT 0,
    path        TEXT NOT NULL DEFAULT '',
    method      TEXT NOT NULL DEFAULT '',
    ip          TEXT NOT NULL DEFAULT '',
    user_agent  TEXT NOT NULL DEFAULT '',
    headers     TEXT NOT NULL DEFAULT '',
    status      INTEGER NOT NULL DEFAULT 0,
    latency     INTEGER NOT NULL DEFAULT 0,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id, created_at)
);

CREATE INDEX IF NOT EXISTS idx_w_user_access_logs_user_id ON w_user_access_logs (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_w_user_access_logs_created_at ON w_user_access_logs (created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS w_user_access_logs;
DROP INDEX IF EXISTS idx_w_user_access_logs_user_id;
DROP INDEX IF EXISTS idx_w_user_access_logs_created_at;
