# 日志用途表

Wavelet 的访问审计等日志表不绑死 ClickHouse。`internal/repository/logstore` 按 `log_database` 在 PostgreSQL / SQLite / ClickHouse 之间切换；关闭 ClickHouse 时由当前业务主库承接写入、查询与清理。

逐步落地步骤见 `.agents/skills/logstore/SKILL.md`。本文只约定判定、分层与切换协议。

## 什么算日志表

同时满足才进 logstore：

- 追加写入，几乎不更新单行
- 按时间查询或聚合，允许按保留天数删除
- 关闭 ClickHouse 后仍要能写、能查
- 不参与用户 / 配置 / 任务等事务一致性

用户、系统配置、任务执行、上传元数据走业务主库 `repository`，不要塞进 logstore。

当前已接入：`w_user_access_logs`（管理端 API 访问审计），接口 `UserAccessLogStore`。

## 分层

| 层级 | 路径 | 职责 |
| :--- | :--- | :--- |
| 抽象 | `internal/repository/logstore` | 接口 + `Active` / `BuildForMigration`；apps 只面向这里 |
| CH 实现 | `logstore` 委托 `repository/analytics` | 原生批量与现有查询 |
| 主库实现 | `logstore` GORM | PostgreSQL 按月分区；SQLite 普通表 |
| 入队 | `risk_control` + `batchwriter` | `FlushFunc` → `logstore.Active` |
| 切换 | `logs:db_switch` | 冻结 → 排空 → 复制 → 翻转 |
| 清理 | `logstore.CleanupExpired` | `system:cleanup` 按库读 `log_retention_days_*`：PG 先 `DropExpiredPartitions` 再 `DeleteBefore`，最后 `DropEmptyPartitions` |

`log_database` 只能是「随业务主库」或 `clickhouse`。`log_database` / `log_db_migration` 受保护，管理端不可改。

## 切换协议

1. 校验 `target` 合法且不等于当前库。
2. 写 `log_db_migration=migrating`，`Drain` 在途队列（不要 `Stop` writer）；写入返回明确错误，不排队。
3. 清空目标表后按 id 分页复制；PostgreSQL 目标先 `EnsurePartitions`。
4. 全部成功才翻转 `log_database`；失败清标记，写入继续走源库。
5. 源数据不删。

不要另起切换协议，也不要在任务或 Handler 里直连 `analyticsrepo` / `db.ChConn`。

## 新增一张日志表

必须同时提供 ClickHouse / PostgreSQL / SQLite 三套 goose，列名一致。接口至少包含 `BatchInsert`、业务查询、`ListForMigration` / `MigrationRange` / `DeleteAll` / `EnsurePartitions`、`DeleteBefore`。`FlushFunc` 调 `logstore.Active`。细节与禁止项见 `logstore` skill。
