# Cordis 架构插件标准分层设计规范 (Plugin Layered Architecture Spec)

- **文档状态**: 已敲定 (Approved)
- **版本**: v1.1.0 (2026-08-28)
- **适用范围**: Wavelet 官方插件 (`backend/plugins/`)、下游定制插件 (`downstream/custom_plugins/`)

---

## 1. 架构总览与统一标准规范 (Architecture & Unified Standard Spec)

在 Wavelet 的 Cordis 微内核架构中，系统通过 **微内核 (`core/`) + 服务契约 (`core/contracts/`) + 自包含插件 (`plugins/` 及 `downstream/plugins/`)** 实现高度解耦与单向依赖。
所有插件统一以 [`backend/downstream/plugins/custom_example`](file:///Users/ryan/Code/Go/Wavelet/backend/downstream/plugins/custom_example) 为基准模板，严格遵循物理子包隔离的分层架构（`controller -> service -> dao -> model`）。

---

## 2. 统一标准分层目录结构 (以 `custom_example` 为基准)

```text
backend/downstream/plugins/custom_example/ (或 backend/plugins/domain/<plugin_name>/)
├── plugin.go              # [插件根入口] 实现 core.Plugin，装配各子包并向 Cordis 注册
│
├── consts/                # package consts：常量与模块内部错误码定义
│   └── consts.go
│
├── controller/            # package controller：HTTP API 接入层 (参数绑定、会话获取、统一信封响应)
│   └── hello/             # 业务分组/实体子包
│       └── hello.go       # 接口处理 Handler（直接以业务命名，禁止 controller_hello.go）
│
├── service/               # package service：核心业务用例层 (业务用例、事务编排、事件发布)
│   └── hello.go           # 业务用例实现（纯 Go 逻辑，禁止依赖 *gin.Context）
│
├── dao/                   # package dao：数据持久化访问层 DAL (GORM CRUD、SQL 转义防注入)
│   └── hello.go           # 数据访问实现（直接以业务命名，禁止 dao_hello.go）
│
├── model/                 # package model：纯数据实体与传输对象 (零 Web/数据库框架依赖)
│   ├── entity/            # 数据库映射实体 (TableName() 必须带 w_<plugin>_ 前缀)
│   │   └── hello.go
│   └── do/                # 领域对象、请求 Request DTO 与响应 Response DTO
│       └── hello.go
│
└── migrations/            # Goose SQL 独立迁移嵌入目录 (//go:embed)
    ├── postgres/          # PostgreSQL 专属迁移 SQL
    │   └── 20260901000001_init.sql
    └── sqlite/            # SQLite 专属迁移 SQL
        └── 20260901000001_init.sql
```

> ⚠️ **严禁规则**：严禁在插件根目录下平铺 `handlers_*.go`、`service_*.go`、`dao_*.go` 等前缀文件，子包内文件直接按业务实体命名。严格约束 `controller -> service -> dao -> model` 单向依赖。
│   ├── auth.go            # 认证业务逻辑（直接命名为 auth.go，禁止 service_auth.go）
│   ├── user.go            # 用户业务逻辑（直接命名为 user.go，禁止 service_user.go）
│   ├── config.go          # 配置业务逻辑（直接命名为 config.go，禁止 service_config.go）
│   └── logs.go            # 日志业务逻辑（直接命名为 logs.go，禁止 service_logs.go）
│
├── repository/            # package repository：数据访问持久化层 (DAL)
│   ├── repository.go      # 仓储通用方法与工厂
│   ├── user.go            # 用户仓储实现（直接命名为 user.go，禁止 repository_user.go）
│   ├── config.go          # 配置仓储实现（直接命名为 config.go，禁止 repository_config.go）
│   └── log.go             # 日志仓储实现（直接命名为 log.go，禁止 repository_log.go）
│
├── model/                 # package model (或 models/)：纯领域实体与传输对象
│   ├── entity.go          # 数据库映射实体 (TableName() 必须带 w_<plugin>_ 前缀)
│   ├── dto.go             # 请求入参与响应出参 DTO
│   └── events.go          # 插件内部/广播事件结构体定义
│
├── errs/                  # package errs：错误常量与错误码定义 (或根目录 errs.go)
│   └── errs.go
│
└── migrations/            # Goose SQL 独立迁移嵌入文件 (//go:embed)
    └── 20260828000001_init_<plugin_name>.sql
```

### 3.2 依赖方向约束 (Strict Dependency Flow)
```mermaid
graph TD
    Plugin[plugin.go 入口] --> Handler[handler/ 接入层]
    Plugin --> Service[service/ 业务层]
    Plugin --> Repository[repository/ 仓储层]
    Handler --> Service
    Handler --> Model[model/ 实体与DTO]
    Handler --> Errs[errs/ 错误常量]
    Service --> Repository
    Service --> Model
    Service --> Errs
    Repository --> Model
```
* **单向依赖铁律**：
  1. `handler/` 依赖 `service/`、`model/`、`errs/`；
  2. `service/` 依赖 `repository/`、`model/`、`errs/`，**严禁 import gin**；
  3. `repository/` 依赖 `model/` 和数据库底层，**严禁反向依赖 service 或 handler**；
  4. `model/` 纯粹由 Go 结构体组成，**严禁依赖上层 handler/service/repository**。

---

## 4. 各层职责边界与编码守则 (Layer Responsibilities & Guardrails)

### 4.1 Handler 层 (`handler/`)
1. **参数绑定**：使用 `c.ShouldBindJSON` 或 `c.ShouldBindQuery`。
2. **上下文提取**：从 `*gin.Context` 提取登录态（如 `oauth.GetCurrentUser(c)`）。
3. **调用下游**：调用 Service 方法，禁止直接调用 Repository 或编写 SQL。
4. **统一信封响应**：
   - 成功：`c.JSON(http.StatusOK, response.OK(data))` 或 `response.OKNil()`。
   - 失败：使用 `backend/pkg/response` 的 `Abort*` 系列函数（如 `AbortBadRequest`、`AbortUnauthorized`、`AbortNotFound`、`AbortInternal`）。
5. **Swagger 注释**：每个导出 Handler 必须编写完整的 OpenAPI/Swagger 注解。

### 4.2 Service 层 (`service/`)
1. **纯 Go 逻辑**：第一参数必须为 `ctx context.Context`，返回 `(result, error)`。
2. **禁止依赖 Web 框架**：严禁 import `github.com/gin-gonic/gin`，严禁接收 `*gin.Context`，严禁调用 `c.JSON`/`Abort*`。
3. **事务编排**：涉及插件内多表原子操作时，通过 `ctx.DB().Transaction(...)` 编排。
4. **事件驱动解耦**：跨插件业务通知与状态联动统一通过 `ctx.Events().Emit(...)` 广播领域事件，杜绝直接跨插件调用私有方法。

### 4.3 Repository 层 (`repository/`)
1. **GORM / SQL 操作**：统一接收 `context.Context`，通过 `db.WithContext(ctx)` 操作数据。
2. **SQL LIKE 防注入**：所有含用户输入的模糊查询必须调用 `backend/pkg/util.EscapeLike` 并显式声明 `ESCAPE '\\'`。
3. **表单一所有者原则**：仅操作本插件所属表（前缀 `w_<plugin>_*`），严禁越权 DML/DDL 其他插件所有表。

### 4.4 Model 层 (`model/` 或 `models/`)
1. **GORM 映射**：显式实现 `TableName() string` 返回带前缀表名。
2. **零值对齐**：Go 结构体字段零值必须与数据库默认值匹配。
3. **无物理外键**：禁止物理外键约束，显式建立单列/复合索引。

### 4.5 Plugin 入口 (`plugin.go`)
1. 实现 `core.Plugin` 接口（`Name() string` 与 `Apply(ctx *core.Context) error`）。
2. 在 `Apply` 中完成：
   - 依赖注入与解析（`core.Provide` / `core.Inject` / `ctx.Using`）
   - 路由与中间件声明（`ctx.Router().Group(...)`）
   - 异步与定时任务注册（`ctx.Task().Register` / `ctx.Schedule().RegisterCron`）
   - 配置与设置声明（`ctx.Settings().Register` / `ctx.Config().Bind`）
   - 数据库迁移注册（`ctx.Migrations().Register`）
