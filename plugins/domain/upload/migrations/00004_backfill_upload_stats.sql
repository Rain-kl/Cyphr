-- +goose Up
INSERT INTO w_upload_stats (dimension, stat_key, file_count, file_size)
SELECT 'total', '', COUNT(*), COALESCE(SUM(file_size), 0)
FROM w_uploads
WHERE status != 'deleted'
ON CONFLICT (dimension, stat_key) DO UPDATE SET
    file_count = EXCLUDED.file_count,
    file_size = EXCLUDED.file_size,
    updated_at = CURRENT_TIMESTAMP;

-- +goose Down
DELETE FROM w_upload_stats;