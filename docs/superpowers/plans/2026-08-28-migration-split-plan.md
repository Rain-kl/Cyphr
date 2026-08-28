# 迁移脚本拆分执行计划

## 背景现状

| 维度 | 实际状态 |
|---|---|
| 总 SQL 文件 | 26 个全局 (`pkg/migrator/goose/`) + 5 个插件 (`plugins/domain/*/migrations/`) + 1 个 ClickHouse |
| 实际运行的迁移 | **仅 26 个全局文件**（通过 `gooseEngine` → `pkg/migrator.Migrate()`） |
| 插件注册的迁移 | 3 个 (`auth`, `user`, `message_gateway`) — 注册了但被 `gooseEngine` 丢弃 |
| 有迁移文件但未注册的插件 | `admin`（2 个文件，0 个调用） |
| 无迁移文件的插件 | `upload`, `risk_control`, `cap`, `driver_asynq_worker`, `driver_asynq_cron` |
| ClickHouse 迁移 | 1 个文件 (`w_user_access_logs`)，通过 `pkg/migrator.MigrateClickHouse()` 单独运行 |

## 表所有者映射

以下列表基于"单一所有者原则"，每个表精确映射到一个插件：

| 表名 | 所有者插件 | 涉及全局迁移 |
|---|---|---|
| `w_users` | `domain/user` | 20260609 (create), 20260614 (seed system user) |
| `w_access_tokens` | `domain/auth` | 20260609 (create), 20260610 (is_admin), 20260611 (rm last_used_at) |
| `w_auth_sources` | `domain/auth` | 20260609 (create) |
| `w_external_accounts` | `domain/auth` | 20260609 (create) |
| `w_system_configs` | `domain/admin` | 20260609→20260611 (rename+seeds×7), 20260613 (TEXT), 20260816 (log_db) |
| `w_templates` | `domain/admin` | 20260609→20260611 (rename) |
| `w_schedules` | `driver_asynq_cron` | 20260610 (create), 20260611 (identity), 20260614 (update cleanup) |
| `w_task_executions` | `driver_asynq_worker` | 20260609→20260611 (rename) |
| `w_uploads` | `domain/upload` | 20260609→20260611 (rename), 20260613 (access_mode), 20260617 (indexes), 20260618 (drop storage_driver) |
| `w_upload_stats` | `domain/upload` | 20260617 (create+backfill) |
| `w_push_events` | `domain/message_gateway` | 20260614 (create), 20260615 (task_type), 20260616 (cleanup) |
| `w_push_histories` | `domain/message_gateway` | 20260614 (create) |
| `w_push_channels` | `domain/message_gateway` | 20260614 (create) |
| `w_message_channels` | `domain/message_gateway` | 20260816 (create) |
| `w_message_bindings` | `domain/message_gateway` | 20260816 (create) |
| `w_message_pairing_codes` | `domain/message_gateway` | 20260816 (create) |
| `w_user_access_logs` | `domain/risk_control` | 20260816 (create) + ClickHouse |

## 执行步骤（共 8 步）

---

### 步骤 1：创建 Bootstrap 迁移（保留在 `pkg/migrator`）

**文件**：`pkg/migrator/goose/postgres/00001_bootstrap.sql`

将以下全局迁移合并为一个 bootstrap 文件：
- **`202606090001_initial_schema.sql`** → 创建 `users`, `auth_sources`, `external_accounts`, `access_tokens`, `system_configs`, `uploads`, `task_executions`, `templates`（全部无前缀旧名）
- **`202606110003_rename_tables_to_w_prefix.sql`** → 全部重命名为 `w_` 前缀

**合并后，bootstrap 文件直接创建带 `w_` 前缀的表**，不再需要 rename 步骤：

```sql
-- +goose Up
CREATE TABLE IF NOT EXISTS w_users (
    id BIGINT PRIMARY KEY,
    username VARCHAR(64) UNIQUE,
    ...
);
CREATE TABLE IF NOT EXISTS w_access_tokens (...);
CREATE TABLE IF NOT EXISTS w_auth_sources (...);
CREATE TABLE IF NOT EXISTS w_external_accounts (...);
CREATE TABLE IF NOT EXISTS w_system_configs (
    key VARCHAR(64) PRIMARY KEY,
    value TEXT NOT NULL,
    ...
);
CREATE TABLE IF NOT EXISTS w_uploads (...);
CREATE TABLE IF NOT EXISTS w_task_executions (...);
CREATE TABLE IF NOT EXISTS w_templates (...);
CREATE TABLE IF NOT EXISTS w_schedules (...);
```

> **为什么保留在 `pkg/migrator`**：这些是平台的"初始化基座"——无论哪些插件启用，这些表都存在。将 bootstrap 放到 `pkg/migrator` 之下回避了循环依赖问题（例如 `w_system_configs` 属于 admin，但 bootstrap 时 admin 插件尚未 apply）。

---

### 步骤 2：修复 `gooseEngine` 支持插件迁移

**文件**：`cmd/app.go`

```go
type gooseEngine struct{}

func (e *gooseEngine) Migrate(_ context.Context, entries []core.MigrationEntry) error {
    // 1. 先跑 bootstrap（初始化基座）
    _ = migrator.Migrate()

    // 2. 再跑每个插件注册的迁移
    for _, entry := range entries {
        gormDB := database.DB(context.Background())
        if gormDB == nil {
            continue
        }
        sqlDB, err := gormDB.DB()
        if err != nil {
            return err
        }

        goose.SetBaseFS(entry.FS)
        if err := goose.SetDialect(gooseDialect()); err != nil {
            return err
        }
        dir := entry.Dir
        if dir == "" {
            dir = "migrations"
        }
        if err := goose.Up(sqlDB, dir); err != nil {
            return fmt.Errorf("migrate %s: %w", entry.PluginID, err)
        }
    }

    // 3. ClickHouse 迁移
    _ = migrator.MigrateClickHouse()

    return nil
}
```

依赖项：`gooseDialect()` 从 `pkg/migrator` 导出。

---

### 步骤 3：按表所有者拆分迁移到各插件

| 全局源文件 | 目标插件 | 迁移文件名 |
|---|---|---|
| `202606100002` (access_tokens is_admin) | `domain/auth` | `migrations/00002_add_access_token_is_admin.sql` |
| `202606110001` (drop last_used_at) | `domain/auth` | `migrations/00003_drop_access_token_last_used_at.sql` |
| `202606140003` (system user seed) | `domain/user` | `migrations/00002_seed_system_user.sql` |
| `202606110004` (file_access_whitelist seed) | `domain/admin` | `migrations/00003_seed_file_access_whitelist.sql` |
| `202606110005` (disk_cache configs seed) | `domain/admin` | `migrations/00004_seed_disk_cache_configs.sql` |
| `202606120002` (update_upstream_repo seed) | `domain/admin` | `migrations/00005_seed_upstream_repo_config.sql` |
| `202606130002` (system_configs value TEXT) | `domain/admin` | `migrations/00006_expand_config_value.sql` |
| `202606130003` (storage_config seed) | `domain/admin` | `migrations/00007_seed_storage_config.sql` |
| `202608160002` (log database configs) | `domain/admin` | `migrations/00008_seed_log_db_configs.sql` |
| `202606120001` (login_session_ttl) | `domain/auth` | `migrations/00004_seed_login_session_ttl.sql` |
| `202606130001` (w_uploads access_mode) | `domain/upload` | `migrations/00001_add_access_mode.sql` |
| `202606170001` (upload indexes) | `domain/upload` | `migrations/00002_add_composite_indexes.sql` |
| `202606170002` (upload stats table) | `domain/upload` | `migrations/00003_create_upload_stats.sql` |
| `202606170003` (backfill stats) | `domain/upload` | `migrations/00004_backfill_upload_stats.sql` |
| `202606180001` (drop storage_driver) | `domain/upload` | `migrations/00005_drop_storage_driver.sql` |
| `202606140001` (push tables) | `domain/message_gateway` | `migrations/00002_create_push_tables.sql` |
| `202606140004` (push channels) | `domain/message_gateway` | `migrations/00003_create_push_channels.sql` |
| `202606150001` (push task_type) | `domain/message_gateway` | `migrations/00004_add_push_task_type.sql` |
| `202606160001` (remove push config) | `domain/message_gateway` | `migrations/00005_remove_push_config.sql` |
| `202608160003` (message gateway tables) | `domain/message_gateway` | `migrations/00006_create_message_tables.sql` |
| `202606100001` (schedules) | `driver_asynq_cron` | `migrations/00001_create_schedules.sql` |
| `202606110002` (schedules identity) | `driver_asynq_cron` | `migrations/00002_alter_schedules_identity.sql` |
| `202606140005` (update cleanup schedule) | `driver_asynq_cron` | `migrations/00003_update_cleanup_schedule.sql` |
| `202608160001` (user access logs) | `domain/risk_control/logstore` | `migrations/00001_create_access_logs.sql` |
| `202608160002` (log_database configs) | `domain/admin` | （合并到 admin 步骤 7） |

---

### 步骤 4：补充缺失的 `go:embed` 和 `Register()` 调用

**`plugins/domain/admin/plugin.go`**：
```go
//go:embed migrations/*.sql
var adminMigrations embed.FS

// 在 Apply() 中:
ctx.Migrations().Register("admin", adminMigrations)
```

**`plugins/domain/upload/plugin.go`**：
```go
//go:embed migrations/*.sql
var uploadMigrations embed.FS

// 在 Apply() 中:
ctx.Migrations().Register("upload", uploadMigrations)
```

**`plugins/domain/risk_control/plugin.go`**：
```go
// go:embed 由 logstore 子包自行处理（它已有自己的 moved 文件）
// 在 Apply() 中:
ctx.Migrations().Register("risk_control/logstore", logstoreMigrationFS)
```

**`plugins/drivers/driver_asynq_cron/plugin.go`**：
```go
//go:embed migrations/*.sql
var cronMigrations embed.FS

// 在 Apply() 中:
ctx.Migrations().Register("driver_asynq_cron", cronMigrations)
```

---

### 步骤 5：解决 Admin 插件迁移与 Bootstrap 的冲突

当前 `admin/migrations/00001` 执行 `CREATE TABLE IF NOT EXISTS w_system_configs (...)`，但 bootstrap 已在步骤 1 中创建过这张表。需要：
1. **保持 `IF NOT EXISTS`** 保证幂等性
2. **从 admin migration 中移除 `w_schedules` 和 `w_task_executions` 的 CREATE**（它们在 bootstrap 中创建，属于 driver 插件）
3. **仅保留 admin 自己的表**：`w_system_configs`, `w_templates`
4. Seed 数据使用 `ON CONFLICT DO NOTHING` 避免重复：

当前 admin 的 seed 包含 29 个系统配置，其中约 14 个与全局迁移重复。整理后的 admin seed 应：

```sql
INSERT INTO w_system_configs (...) VALUES
    ('cap_login_enabled', 'false', ...),
    ('cap_auto_solve', 'true', ...),
    -- ... (所有 29 个配置)
ON CONFLICT (key) DO NOTHING;
```

> 全局迁移中 `202606110004` 到 `202608160002` 的 7 个种子 INSERT 将被迁移到 admin，全部使用 `ON CONFLICT DO NOTHING`。

---

### 步骤 6：清理已迁移的全局文件

拆分完成后，从 `pkg/migrator/goose/postgres/` 中删除以下文件：

```
202606100002_access_token_is_admin.sql
202606100001_create_schedules.sql
202606110001_remove_access_token_last_used_at.sql
202606110002_alter_schedules_id_auto_increment.sql
202606110004_add_file_access_whitelist_config.sql
202606110005_add_disk_cache_configs.sql
202606120001_add_login_session_ttl_config.sql
202606120002_add_update_upstream_repository_config.sql
202606130001_add_upload_access_mode.sql
202606130002_expand_system_config_value.sql
202606130003_add_storage_config.sql
202606140001_create_push_tables.sql
202606140003_add_system_user.sql
202606140004_create_push_channels.sql
202606140005_update_system_cleanup_schedule.sql
202606150001_add_task_type_to_push_events.sql
202606160001_remove_push_config.sql
202606170001_add_upload_composite_indexes.sql
202606170002_create_upload_stats_table.sql
202606170003_backfill_upload_stats.sql
202606180001_drop_upload_storage_driver.sql
202608160001_create_user_access_logs.sql
202608160002_log_database_configs.sql
202608160003_create_message_gateway.sql
```

**保留在 `pkg/migrator/goose/postgres/` 的仅限**：
```
00001_bootstrap.sql       （合并后的初始化基座）
```

**注意**：`202606110003_rename_tables_to_w_prefix.sql` 也被合并进 bootstrap。`202606090001_initial_schema.sql` 也被合并掉。

---

### 步骤 7：更新 `pkg/migrator` 导出 `gooseDialect()`

在 `pkg/migrator/migrator.go` 中将 `gooseDialect()` 和 `migrationDir()` 改为导出，供 `cmd/app.go` 的 `gooseEngine.Migrate()` 引用。

---

### 步骤 8：验证 + 提交

```bash
cd /Users/ryan/Code/Go/Wavelet

# 1. 编译验证
go build -mod=mod ./...
go vet ./...

# 2. 架构门禁验证
make code-check

# 3. 验证插件迁移注册完整性
grep -rn 'go:embed.*migrations' plugins/domain/*/plugin.go plugins/drivers/*/plugin.go
grep -rn 'Migrations()\.Register' plugins/domain/*/plugin.go plugins/drivers/*/plugin.go
# → 每个有 migrations/ 目录的插件既要有 go:embed 又要有 Register()

# 4. 验证 admin 插件迁移完整性
grep -rn 'w_schedules\|w_task_executions' plugins/domain/admin/migrations/
# → 不应有（这些属于 driver 插件）

# 5. 提交
git add -A && git commit -m "refactor(migration): split global SQL into per-plugin migrations

- Merge 26 global SQLs into bootstrap + per-plugin migrations
- Fix gooseEngine to iterate plugin-registered MigrationEntry
- Add go:embed + Register() to admin, upload, risk_control, driver_asynq_cron
- Remove 23 migrated SQL files from pkg/migrator/goose/
- Keep only bootstrap in pkg/migrator/goose/
- All CREATE TABLE use IF NOT EXISTS, all INSERT use ON CONFLICT DO NOTHING"
```

---

## 依赖关系图

```
Bootstrap (pkg/migrator)
  ├── 创建 w_users, w_access_tokens, w_auth_sources, w_external_accounts
  ├── 创建 w_system_configs, w_templates, w_schedules, w_task_executions
  ├── 创建 w_uploads, w_upload_stats
  └── 创建所有 w_ 前缀表
       │
       ├─ auth/00002 (access_tokens is_admin)
       ├─ auth/00003 (drop last_used_at)
       ├─ auth/00004 (login_session_ttl seed)
       │
       ├─ user/00002 (system user seed)
       │
       ├─ admin/00001 (w_system_configs, w_templates) [IF NOT EXISTS]
       ├─ admin/00002 (29 config seeds + 2 template seeds)
       ├─ admin/00003–00008 (拆分后的种子迁移)
       │
       ├─ upload/00001–00005 (access_mode → indexes → stats → backfill → drop)
       │
       ├─ message_gateway/00001 (w_message_* tables)
       ├─ message_gateway/00002–00006 (push tables → channels → task_type → cleanup)
       │
       ├─ driver_asynq_cron/00001–00003 (schedules → identity → cleanup)
       │
       ├─ driver_asynq_worker/00001 (task_executions — 如果有追加操作)
       │
       └─ risk_control/logstore/00001 (w_user_access_logs)
```

所有步骤执行的迁移顺序由 Goose 的文件名前缀控制。Bootstrap 使用 `00001_`，每个插件的迁移从 `00002_` 开始编号（`00001` 留给插件自身表 CREATE，若插件 bootstrap 已创建则从 `00002` 开始）。