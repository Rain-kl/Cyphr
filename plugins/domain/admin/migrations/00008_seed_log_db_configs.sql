-- +goose Up
INSERT INTO w_system_configs (key, value, type, visibility, description, created_at, updated_at)
VALUES
  ('log_database', '', 'system', 0, '当前日志主库（postgres/sqlite/clickhouse），由切换任务写入', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
  ('log_db_migration', '', 'system', 0, '日志库迁移冻结标记（空或 migrating）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (key) DO NOTHING;

-- +goose Down
DELETE FROM w_system_configs WHERE key IN ('log_database', 'log_db_migration');