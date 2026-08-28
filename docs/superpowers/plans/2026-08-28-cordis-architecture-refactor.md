# Cordis 架构重构实施计划 (Cordis Architecture Refactor Implementation Plan)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 依据 Cordis 时空可组合性元框架，彻底消除 Wavelet 后端的包级静态单例、`init()` 隐式副作用建连以及跨插件私有实现依赖，实现微内核纯洁化与契约驱动解耦。

**Architecture:** 
1. 移除 `backend/core/context.go` 中的特权服务快捷方法（`DB()` / `Cache()`）。
2. 将 `infra/database` 与 `infra/cache` 的连接初始化移至 `Plugin.Apply(ctx)`，并在 `ctx.OnDispose` 中注册 LIFO 逆操作（Close）。
3. 重构全部 8 个 Domain 业务插件（`auth`、`user`、`admin`、`cap`、`message_gateway`、`risk_control`、`system`、`upload`），彻底斩断对 `infra/database`、`infra/cache` 及其他插件内部包的直接 import，统一面向 `contracts.DBService` / `contracts.CacheService`。
4. 清除 `admin` 等插件的包级全局变量。

**Tech Stack:** Go 1.24+, GORM, Redis (go-redis/v9), Cordis micro-kernel, Goose migration.

## Global Constraints

- 严禁任何业务插件跨包 import `Wavelet/plugins/infra/database` 或 `Wavelet/plugins/infra/cache`。
- 严禁跨插件 import 私有实现包（如 `admin` import `risk_control/logstore`）。
- 保持 `backend/pkg/util/` 绝对纯净，禁止导入 Web/数据库框架。
- 重构后必须确保 `go test ./...`、`make code-check` 与 `make format` 全部 0 错误通过。

---

### Task 1: 微内核纯洁化 (`backend/core/`)

**Files:**
- Modify: `backend/core/context.go:240-260`
- Test: `backend/core/context_test.go`

**Interfaces:**
- Consumes: `core.Context`, `core.Inject`
- Produces: 纯净无特权方法的 `core.Context`

- [ ] **Step 1: 编写/更新 Context 纯洁性测试**
- [ ] **Step 2: 移除 `Context.DB()` 与 `Context.Cache()` 方法**
- [ ] **Step 3: 运行 `go test ./backend/core/...` 验证通过**

---

### Task 2: 基础设施插件生命周期可逆化 (`backend/plugins/infra/`)

**Files:**
- Modify: `backend/plugins/infra/database/postgres.go`
- Modify: `backend/plugins/infra/database/plugin.go`
- Modify: `backend/plugins/infra/cache/redis.go`
- Modify: `backend/plugins/infra/cache/plugin.go`
- Test: `backend/plugins/infra/infra_test.go`

**Interfaces:**
- Consumes: `core.Plugin`, `contracts.DBService`, `contracts.CacheService`
- Produces: `contracts.DBService` 与 `contracts.CacheService`（带 `ctx.OnDispose` 逆操作）

- [ ] **Step 1: 移除 `infra/database` 中的 `func init()` 及全局 `var db`，在 `Plugin.Apply` 中建连并注册 `ctx.OnDispose(sqlDB.Close)`**
- [ ] **Step 2: 移除 `infra/cache` 中的 `func init()` 及全局 `var Redis`，在 `Plugin.Apply` 中建连并注册 `ctx.OnDispose(client.Close)`**
- [ ] **Step 3: 运行 `go test ./backend/plugins/infra/...` 验证通过**

---

### Task 3: 核心 Domain 插件防线重塑（Auth & User 插件）

**Files:**
- Modify: `backend/plugins/domain/auth/*`
- Modify: `backend/plugins/domain/user/*`
- Test: `backend/plugins/domain/auth/plugin_test.go`
- Test: `backend/plugins/domain/user/plugin_test.go`

**Interfaces:**
- Consumes: `contracts.DBService`, `contracts.CacheService`
- Produces: `contracts.AuthService`, `contracts.UserService`

- [ ] **Step 1: 移除 `auth` 插件中对 `Wavelet/plugins/infra/database` 和 `cache` 的 import，改用插件持有的 `contracts.DBService` 与 `contracts.CacheService`**
- [ ] **Step 2: 移除 `user` 插件中对 `Wavelet/plugins/infra/database` 和 `cache` 的 import，改用 `contracts.DBService` 与 `contracts.CacheService`**
- [ ] **Step 3: 运行 `go test ./backend/plugins/domain/auth/... ./backend/plugins/domain/user/...` 验证通过**

---

### Task 4: 业务 Domain 插件防线重塑（Cap, MessageGateway, RiskControl, System, Upload）

**Files:**
- Modify: `backend/plugins/domain/cap/*`
- Modify: `backend/plugins/domain/message_gateway/*`
- Modify: `backend/plugins/domain/risk_control/*`
- Modify: `backend/plugins/domain/system/*`
- Modify: `backend/plugins/domain/upload/*`
- Test: `backend/plugins/domain/domain_test.go`

**Interfaces:**
- Consumes: `contracts.DBService`, `contracts.CacheService`

- [ ] **Step 1: 改造 `cap`、`message_gateway`、`risk_control`、`system`、`upload` 插件，移除所有 `infra/database` 和 `infra/cache` 的直接 import**
- [ ] **Step 2: 统一各插件内部 Repository / Service 的 DB / Cache 获取途径**
- [ ] **Step 3: 运行各插件单测验证通过**

---

### Task 5: Admin 插件解耦与包级全局状态清除

**Files:**
- Modify: `backend/plugins/domain/admin/*`
- Test: `backend/plugins/domain/admin/plugin_test.go`

**Interfaces:**
- Consumes: `contracts.DBService`, `contracts.CacheService`, `contracts.UserService`, `contracts.AuthService`, `ctx.Tasks()`

- [ ] **Step 1: 移除 `admin` 插件中对 `risk_control/logstore`、`driver_asynq_worker`、`infra/storage/diskcache` 等私有包的直接 import**
- [ ] **Step 2: 清除 `admin/plugin.go` 中的 `globalUserSvc`、`globalAuthSvc`、`globalCoreCtx` 等包级变量**
- [ ] **Step 3: 运行 `go test ./backend/plugins/domain/admin/...` 验证通过**

---

### Task 6: 组装层对齐与全量质量门禁验证

**Files:**
- Modify: `backend/cmd/app.go`
- Modify: `backend/cmd/*`

- [ ] **Step 1: 检查并适配 `cmd/app.go` 及启动指令，确保 Goose 迁移与驱动正确接入新版 `DBService`**
- [ ] **Step 2: 运行全局跨包 import 检查：`grep -r "Wavelet/plugins/infra/database" backend/plugins/domain/` 必须为空**
- [ ] **Step 3: 运行全量单元测试与基准测试：`go test ./...`**
- [ ] **Step 4: 运行质量门禁：`make code-check && make format`**
