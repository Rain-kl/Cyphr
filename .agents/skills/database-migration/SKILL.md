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
backend/plugins/domain/order/
├── plugin.go
├── models.go
└── migrations/
    └── 00001_initial.sql        ← 每个插件仅一个初始迁移文件
```

---

## 2. 插件迁移代码集成标准

### 步骤 1：在插件内嵌入并注册迁移

```go
package order

import (
	"embed"
	"github.com/Rain-kl/Wavelet/backend/core"
)

//go:embed migrations/*.sql
var orderMigrations embed.FS

func (p *Plugin) Apply(ctx *core.Context) error {
	// 注册本插件的专属迁移（系统启动时由微内核统一收集并按版本执行）
	ctx.Migrations().Register("order", orderMigrations)
	return nil
}
```

### 步骤 2：编写 Goose SQL 脚本 (`migrations/00001_initial.sql`)

每个插件只需维护一个 `00001_initial.sql`，包含其全部建表语句与种子数据。

```sql
-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS w_orders (
    id VARCHAR(64) PRIMARY KEY,
    user_id VARCHAR(64) NOT NULL,
    amount BIGINT NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_w_orders_user_id ON w_orders(user_id);

-- 种子数据
INSERT INTO w_orders (id, user_id, amount, status)
VALUES ('init_001', 'system', 0, 'completed')
ON CONFLICT (id) DO NOTHING;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS w_orders;
-- +goose StatementEnd
```

---

## 3. 版本管理与升级机制

### 3.1 版本表结构

所有插件共享一张 `w_schema_versions` 表，以 `plugin_id` 为区分：

```sql
w_schema_versions (
    plugin_id   VARCHAR(64)  NOT NULL,   -- 如 "auth", "user", "admin"
    version_id  BIGINT       NOT NULL,   -- 迁移文件版本号 (00001 → 1)
    applied_at  TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (plugin_id, version_id)
)
```

### 3.2 升级判定逻辑

启动时，`gooseEngine` 遍历每个已注册的插件：

```
for each plugin:
  1. 查询 w_schema_versions WHERE plugin_id = 'auth'
  2. 获取该插件的最大 version_id
  3. 读取插件 migration/ 目录下的所有 .sql 文件
  4. 如果存在 version_id 更大的文件 → 执行升级
  5. 如果全部已应用 → 跳过
```

### 3.3 什么情况下升级？

| 场景 | 例子 | 是否升级 |
|------|------|---------|
| 首次部署，插件第一次运行 | auth 插件，表不存在 | ✅ 执行 `00001_initial.sql` |
| 第二次启动，无变化 | 文件未变，版本已记录 | ❌ 跳过 |
| 追加新迁移文件 | 新增 `00002_add_index.sql` | ✅ 执行 `00002_*` |
| 移除一个插件 | 该插件不再注册 | ❌ 其记录在表中被忽略 |
| 新增一个插件 | 新插件有 `00001_initial.sql` | ✅ 执行 |

### 3.4 查看全局迁移状态

```sql
SELECT * FROM w_schema_versions ORDER BY plugin_id, version_id;
```

输出示例：

```
plugin_id            | version_id | applied_at
---------------------+------------+---------------------------
admin                |          1 | 2026-08-28 10:00:00+00
auth                 |          1 | 2026-08-28 10:00:00+00
user                 |          1 | 2026-08-28 10:00:00+00
upload               |          1 | 2026-08-28 10:00:00+00
```

---

## 4. 核心设计与防线原则 (Guardrails)

1. **表单一所有者原则 (Single Owner Principle)**：
   - 每张数据表归属且仅归属于一个所有者插件（如 `w_orders` 归 `order` 插件）。
   - **严禁**插件 B 跨包编写 SQL 直接读写插件 A 拥有的表；必须通过插件 A 暴露的 `contracts` 接口或事件总线进行交互。

2. **表名前缀规范**：
   - 所有表名必须带有前缀（如 `w_orders`、`w_auth_users`），杜绝跨插件表名冲突。

3. **单文件初始迁移**：
   - 每个插件只维护一个 `00001_initial.sql`，包含该插件所有表的建表语句与初始种子数据。
   - 未来如需追加 DDL，新增 `00002_xxx.sql`，Goose 会根据 `w_schema_versions` 判断增量执行。

4. **禁止物理外键**：
   - 关系字段统一显式建立单列或联合索引，禁止在数据库中创建物理外键约束。

5. **双方言兼容性（PostgreSQL & SQLite）**：
   - 自增主键：PG 用 `BIGSERIAL`，SQLite 用 `INTEGER PRIMARY KEY AUTOINCREMENT`。
   - 时间类型：PG 用 `TIMESTAMPTZ`，SQLite 用 `DATETIME`。
   - JSON 类型：PG 用 `JSONB`，SQLite 用 `JSON` 或 `TEXT`。

6. **幂等性要求**：
   - 所有 `CREATE TABLE` 必须使用 `IF NOT EXISTS`。
   - 所有 `INSERT` 种子数据必须使用 `ON CONFLICT DO NOTHING`。
   - 所有 `ALTER TABLE ADD COLUMN` 必须使用 `IF NOT EXISTS`（如果数据库方言支持）。

---

## 5. ClickHouse 分析库迁移规则 (辅助 OLAP)

ClickHouse 作为辅助 OLAP 分析存储，采用独立迁移通道：
- 迁移文件位于专属目录 `migrations-clickhouse/`（仅单方言 DDL，不创建 SQLite 镜像）。
- 日志/分析用途表必须同时在关系型主库建回落表并接入 `logstore` 门面。
- 分析表高频写入统一接入 `batchwriter` 进行异步批量刷盘。

---

## 6. 质量与验证门禁

```bash
make format
make code-check
go test ./plugins/...
```

验证迁移注册完整性：

```bash
# 检查每个有 migrations/ 目录的插件是否同时有 go:embed + Register()
grep -rn 'go:embed.*migrations' backend/plugins/domain/*/plugin.go backend/plugins/drivers/*/plugin.go
grep -rn 'Migrations()\.Register' backend/plugins/domain/*/plugin.go backend/plugins/drivers/*/plugin.go
```