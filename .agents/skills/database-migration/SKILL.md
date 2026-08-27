---
name: "database-migration"
description: "Wavelet 项目专用：当新增或修改数据库表结构、索引、初始化数据、插件自包含 Goose SQL 迁移、embed.FS 注册、PG/SQLite 双方言支持或 ClickHouse 分析库 DDL 时必须使用。"
---

# 数据库独立迁移与表结构开发规范 (Cordis 插件化架构)

本技能是 Wavelet 在 Cordis 微内核与插件化架构下，进行数据库表结构设计、Goose SQL 迁移与插件嵌入式注册的唯一指导规范。

---

## 1. 核心架构：插件自包含迁移 (Self-Contained Migrations)

在 Cordis 架构中，**彻底告别集中式单体大迁移目录**。
每个插件在自身包内维护专属的 `migrations/` 目录，通过 Go 语言内置 `//go:embed` 打包为嵌入式文件系统，并在 `Apply(ctx *core.Context)` 时通过微内核扩展点 `ctx.Migrations().Register(...)` 自主注入。

```
plugins/domain/order/
├── plugin.go
├── models.go
└── migrations/
    ├── 20260827000001_create_orders_table.sql
    └── 20260827000002_add_order_discount_column.sql
```

---

## 2. 插件迁移代码集成标准

### 步骤 1：在插件内嵌入并注册迁移

```go
package order

import (
	"embed"
	"github.com/Rain-kl/Wavelet/core"
)

//go:embed migrations/*.sql
var orderMigrations embed.FS

func (p *Plugin) Apply(ctx *core.Context) error {
	// 注册本插件的专属迁移（系统启动时由微内核统一收集并按版本执行）
	ctx.Migrations().Register("order", orderMigrations)
	return nil
}
```

### 步骤 2：编写 Goose SQL 脚本 (`migrations/YYYYMMDDNNNN_name.sql`)

```sql
-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS w_orders (
    id VARCHAR(64) PRIMARY KEY,
    user_id VARCHAR(64) NOT NULL,
    amount BIGINT NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_w_orders_user_id ON w_orders(user_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS w_orders;
-- +goose StatementEnd
```

---

## 3. 核心设计与防线原则 (Guardrails)

1. **表单一所有者原则 (Single Owner Principle)**：
   - 每张数据表归属且仅归属于一个所有者插件（如 `w_orders` 归 `order` 插件）。
   - **严禁**插件 B 跨包编写 SQL 直接读写插件 A 拥有的表；必须通过插件 A 暴露的 `contracts` 接口或事件总线进行交互。
2. **表名前缀规范**：
   - 所有表名必须带有前缀（如 `w_orders`、`w_auth_users`），杜绝跨插件表名冲突。
3. **DDL 与 DML 分离**：
   - 表结构变更（DDL）与初始数据插入（DML/Seed）必须分成两个独立的递增版本 SQL 文件。
4. **禁止物理外键**：
   - 关系字段统一显式建立单列或联合索引，禁止在数据库中创建物理外键约束。
5. **双方言兼容性（PostgreSQL & SQLite）**：
   - 自增主键：PG 用 `BIGSERIAL`，SQLite 用 `INTEGER PRIMARY KEY AUTOINCREMENT`。
   - 时间类型：PG 用 `TIMESTAMPTZ`，SQLite 用 `DATETIME`。
   - JSON 类型：PG 用 `JSONB`，SQLite 用 `JSON` 或 `TEXT`。
6. **定时调度插入规范**：
   - 若迁移中包含初始定时任务插入（`schedules` 表），绝对不能硬编码 `id`，必须依靠数据库自增分配。

---

## 4. ClickHouse 分析库迁移规则 (辅助 OLAP)

ClickHouse 作为辅助 OLAP 分析存储，采用独立迁移通道：
- 迁移文件位于专属目录（仅单方言 DDL，不创建 SQLite 镜像）。
- 日志/分析用途表必须同时在关系型主库建回落表并接入 `logstore`。
- 分析表高频写入统一接入 `batchwriter` 进行异步批量刷盘。

---

## 5. 质量与验证门禁

```bash
make format
make code-check
go test ./plugins/...
```
