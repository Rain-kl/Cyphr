-- +goose Up
INSERT OR IGNORE INTO w_system_configs (key, value, type, visibility, description, created_at, updated_at)
VALUES
    ('log_database', '', 'system', 0, '当前日志主库（postgres/sqlite/clickhouse），由切换任务写入', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('log_db_migration', '', 'system', 0, '日志库迁移冻结标记（空或 migrating）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('log_retention_days_postgres', '30', 'business', 0, 'PostgreSQL 用户访问日志保留天数', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('log_retention_days_sqlite', '30', 'business', 0, 'SQLite 用户访问日志保留天数', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('log_retention_days_clickhouse', '30', 'business', 0, 'ClickHouse 用户访问日志保留天数', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);

-- +goose Down
DELETE FROM w_system_configs WHERE key IN (
    'log_database',
    'log_db_migration',
    'log_retention_days_postgres',
    'log_retention_days_sqlite',
    'log_retention_days_clickhouse'
);
