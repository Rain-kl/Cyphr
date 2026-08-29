-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS w_system_configs (
    key VARCHAR(64) PRIMARY KEY,
    value TEXT NOT NULL,
    type VARCHAR(32) NOT NULL DEFAULT 'system',
    visibility INTEGER NOT NULL DEFAULT 0,
    description VARCHAR(255),
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS w_templates (
    id BIGINT PRIMARY KEY,
    key VARCHAR(80) NOT NULL UNIQUE,
    name VARCHAR(100) NOT NULL,
    type VARCHAR(20) NOT NULL DEFAULT 'email',
    subject VARCHAR(255),
    content TEXT NOT NULL,
    description VARCHAR(255),
    is_system BOOLEAN NOT NULL DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_w_templates_is_system ON w_templates (is_system);
CREATE INDEX IF NOT EXISTS idx_w_templates_created_at ON w_templates (created_at);
CREATE INDEX IF NOT EXISTS idx_w_templates_updated_at ON w_templates (updated_at);

CREATE TABLE IF NOT EXISTS w_schedules (
    id BIGINT PRIMARY KEY,
    name VARCHAR(128) NOT NULL,
    task_type VARCHAR(64) NOT NULL,
    cron VARCHAR(64) NOT NULL,
    payload TEXT,
    is_active BOOLEAN NOT NULL DEFAULT 1,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_w_schedules_is_active ON w_schedules (is_active);

-- Seed initial cleanup task
INSERT INTO w_schedules (id, name, task_type, cron, payload, is_active, created_at, updated_at)
VALUES (1, '系统定期垃圾清理', 'system_cleanup', '0 3 * * *', '{}', 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (id) DO NOTHING;

CREATE TABLE IF NOT EXISTS w_task_executions (
    id BIGINT PRIMARY KEY,
    task_id VARCHAR(128) NOT NULL UNIQUE,
    task_type VARCHAR(64) NOT NULL,
    task_name VARCHAR(128),
    status VARCHAR(32) NOT NULL,
    retryable BOOLEAN NOT NULL DEFAULT 0,
    max_retry INTEGER NOT NULL DEFAULT 0,
    retry_count INTEGER NOT NULL DEFAULT 0,
    log TEXT,
    error_message TEXT,
    result TEXT,
    started_at DATETIME,
    finished_at DATETIME,
    duration BIGINT,
    payload TEXT,
    triggered_by VARCHAR(32) NOT NULL DEFAULT 'system',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_w_task_executions_task_type ON w_task_executions (task_type);
CREATE INDEX IF NOT EXISTS idx_w_task_executions_status ON w_task_executions (status);
CREATE INDEX IF NOT EXISTS idx_w_task_executions_started_at ON w_task_executions (started_at);
CREATE INDEX IF NOT EXISTS idx_w_task_executions_created_at ON w_task_executions (created_at);

-- Seed system configs (all default platform configs)
INSERT INTO w_system_configs (key, value, type, visibility, description, created_at, updated_at) VALUES
    ('cap_login_enabled', 'false', 'system', 1, '是否启用登录人机验证（true/false）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('cap_auto_solve', 'true', 'system', 1, '打开页面后是否自动开始计算，关闭则需用户手动点击触发', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('cap_challenge_count', '1', 'system', 0, '客户端需求解的 PoW 难题总数，默认 1，推荐 1～5', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('cap_challenge_size', '32', 'system', 0, '人机验证盐值长度', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('cap_challenge_difficulty', '4', 'system', 0, '人机验证 PoW 难度（目标前缀长度）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('cap_challenge_ttl_seconds', '600', 'system', 0, '人机验证难题有效时间（秒）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('cap_token_ttl_seconds', '1200', 'system', 0, '人机验证兑换凭证有效时间（秒）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('server_address', '', 'system', 0, '服务器地址（用于跨域源控制，不设定则允许任意源）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('smtp_host', '', 'system', 0, 'SMTP 服务器地址（例如 smtp.example.com）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('smtp_port', '587', 'system', 0, 'SMTP 端口（例如 587 或 465）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('smtp_username', '', 'system', 0, 'SMTP 账户（如 sender@example.com）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('smtp_password', '', 'system', 0, 'SMTP 访问凭证（授权码/密码）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('upload_allowed_extensions', 'jpg,png,webp', 'system', 1, '允许上传的图片扩展名（逗号分隔）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('site_name', 'Wavelet', 'system', 1, '系统平台的展示名称', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('password_login_enabled', 'true', 'system', 1, '是否允许使用账号密码登录', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('registration_enabled', 'true', 'system', 1, '控制普通用户是否可以自主注册（true/false）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('password_register_enabled', 'true', 'system', 1, '是否允许通过密码创建本地账号', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('oidc_login_enabled', 'true', 'system', 1, '是否允许使用第三方 OIDC 认证源登录', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('max_api_keys_per_user', '5', 'business', 1, '限制每个普通用户可以创建的 API Key 最大数量', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('email_login_verification_enabled', 'false', 'system', 1, '是否开启邮箱登录验证（true/false）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('email_register_verification_enabled', 'false', 'system', 1, '是否开启邮箱注册验证（true/false）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('menu_display_config', '{}', 'system', 1, '目录显示配置（JSON 字符串，格式为 {url: enabled}）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('search_engine_indexing_enabled', 'false', 'system', 1, '是否允许搜索引擎爬取/检索该站点（true/false）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('update_upstream_repository', 'Rain-kl/Wavelet', 'system', 0, 'GitHub Actions Release 上游仓库（owner/repo 或 GitHub 仓库地址）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('storage_config', '{"driver":"local","local":{"root":"."},"s3":{"region":"us-east-1"},"r2":{"region":"auto"},"minio":{"region":"us-east-1","path_style":true},"oss":{},"webdav":{}}', 'system', 0, '文件存储驱动及连接配置（JSON）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('disk_cache_max_size_mb', '1024', 'system', 0, '磁盘缓存最大空间大小（MB）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('disk_cache_ttl_minutes', '1440', 'system', 0, '磁盘缓存默认有效期（分钟）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('disk_cache_lru_enabled', 'true', 'system', 0, '是否启用 LRU 淘汰机制', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('file_access_whitelist', '["avatar"]', 'system', 0, '免登录访问的文件业务类型白名单 (JSON 数组)', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('login_session_ttl_hours', '168', 'system', 0, '登录会话过期时间（小时）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('log_database', '', 'system', 0, '当前日志主库（postgres/sqlite/clickhouse），由切换任务写入', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('log_db_migration', '', 'system', 0, '日志库迁移冻结标记（空或 migrating）', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (key) DO NOTHING;

INSERT INTO w_templates (id, key, name, type, subject, content, description, is_system, created_at, updated_at) VALUES
    (1, 'login_email', '登录验证码邮件', 'email', 'Wavelet 登录验证码', '<h3>Wavelet 登录验证</h3><p>您的登录验证码为：<strong>{{.Code}}</strong>，5分钟内有效，请勿将验证码泄露给他人。</p>', '用户密码登录时发送的验证码邮件模板，支持变量：{{.Code}}', 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    (2, 'register_email', '注册验证码邮件', 'email', 'Wavelet 注册验证码', '<h3>Wavelet 注册验证</h3><p>您的注册验证码为：<strong>{{.Code}}</strong>，5分钟内有效，请勿泄露给他人。</p>', '用户注册时发送的验证码邮件模板，支持变量：{{.Code}}', 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (key) DO NOTHING;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM w_templates WHERE key IN ('login_email', 'register_email');
DELETE FROM w_system_configs WHERE key IN (
    'cap_login_enabled', 'cap_auto_solve', 'cap_challenge_count', 'cap_challenge_size',
    'cap_challenge_difficulty', 'cap_challenge_ttl_seconds', 'cap_token_ttl_seconds',
    'server_address', 'smtp_host', 'smtp_port', 'smtp_username', 'smtp_password',
    'upload_allowed_extensions', 'site_name', 'password_login_enabled', 'registration_enabled',
    'password_register_enabled', 'oidc_login_enabled', 'max_api_keys_per_user',
    'email_login_verification_enabled', 'email_register_verification_enabled',
    'menu_display_config', 'search_engine_indexing_enabled', 'update_upstream_repository',
    'storage_config', 'disk_cache_max_size_mb', 'disk_cache_ttl_minutes', 'disk_cache_lru_enabled',
    'file_access_whitelist', 'login_session_ttl_hours', 'log_database', 'log_db_migration'
);
DROP TABLE IF EXISTS w_task_executions;
DROP TABLE IF EXISTS w_schedules;
DROP TABLE IF EXISTS w_templates;
DROP TABLE IF EXISTS w_system_configs;
-- +goose StatementEnd
