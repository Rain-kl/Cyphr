-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS w_upload_stats (
    dimension VARCHAR(32) NOT NULL,
    stat_key VARCHAR(64) NOT NULL DEFAULT '',
    file_count BIGINT NOT NULL DEFAULT 0,
    file_size BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (dimension, stat_key)
);

CREATE INDEX IF NOT EXISTS idx_w_uploads_status_created_at ON w_uploads (status, created_at);
CREATE INDEX IF NOT EXISTS idx_w_uploads_hash_file_size_status ON w_uploads (hash, file_size, status);

ALTER TABLE w_uploads ADD COLUMN IF NOT EXISTS access_mode INTEGER NOT NULL DEFAULT 0;
UPDATE w_uploads SET access_mode = 1 WHERE type = 'avatar';

-- Backfill upload stats
INSERT INTO w_upload_stats (dimension, stat_key, file_count, file_size)
SELECT 'total', '', COUNT(*), COALESCE(SUM(file_size), 0)
FROM w_uploads
WHERE status != 'deleted'
ON CONFLICT (dimension, stat_key) DO UPDATE SET
    file_count = EXCLUDED.file_count,
    file_size = EXCLUDED.file_size,
    updated_at = CURRENT_TIMESTAMP;

INSERT INTO w_upload_stats (dimension, stat_key, file_count, file_size)
SELECT
    'type',
    COALESCE(NULLIF(type, ''), 'generic'),
    COUNT(*),
    COALESCE(SUM(file_size), 0)
FROM w_uploads
WHERE status != 'deleted'
GROUP BY COALESCE(NULLIF(type, ''), 'generic')
ON CONFLICT (dimension, stat_key) DO UPDATE SET
    file_count = EXCLUDED.file_count,
    file_size = EXCLUDED.file_size,
    updated_at = CURRENT_TIMESTAMP;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS w_upload_stats;
DROP INDEX IF EXISTS idx_w_uploads_hash_file_size_status;
DROP INDEX IF EXISTS idx_w_uploads_status_created_at;
ALTER TABLE w_uploads DROP COLUMN IF EXISTS access_mode;
-- +goose StatementEnd