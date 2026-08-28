# Cordis 架构对齐与系统重构设计规范 (Design Spec)

- **Date:** 2026-08-28
- **Topic:** Cordis Architecture Alignment & Full System Refactor
- **Status:** Approved

---

## 1. 目标与背景 (Goal & Background)

本项目遵循 **Cordis（时空可组合性元框架）** 的核心设计哲学：
- **时间可组合性（Time Composability）**：所有对运行时环境的修改（扩展点挂载、事件订阅、服务注册、状态配置）均具备显式可逆操作（Revertible Effects），卸载时按 LIFO（后进先出）严格回收。
- **空间可组合性（Space Composability）**：插件运行在独立的 Scoped Context 分支中，通过面向契约（`contracts`）与事件（`EventBus`）解耦，消除人工硬编码启动顺序与跨插件内部实现耦合。
- **表单一所有者原则（Single Owner Principle）**：每张数据表由且仅由一个所有者插件维护，严禁旁路 DML 读写。

本规范定义微内核层、服务契约层、基础设施层与业务域插件的全量重构设计。

---

## 2. 系统架构与分层设计 (Architecture & Layers)

```
┌────────────────────────────────────────────────────────────────────────┐
│                          Micro-Kernel (core/)                          │
│   Context Bus (Scoped Fork & LIFO Disposer) | Container | EventBus     │
│   Extpoints (Router, Tasks, Schedules, Settings, Migrations)           │
├────────────────────────────────────────────────────────────────────────┤
│                     Service Contracts (contracts/)                     │
│   DBService, CacheService, StorageService, UserService, AuthService... │
├─────────────────────────┬──────────────────────────────────────────────┤
│  Runtime Drivers        │  Platform Infra Plugins                      │
│  (plugins/drivers/)     │  (plugins/infra/)                            │
│  - driver_http (Gin)    │  - database (contracts.DBService)            │
│  - driver_asynq_worker  │  - cache (contracts.CacheService)            │
│  - driver_asynq_cron    │  - storage, logger                           │
├─────────────────────────┴──────────────────────────────────────────────┤
│  Self-Contained Domain Plugins (plugins/domain/)                       │
│  - user, auth, admin, cap, message_gateway, risk_control, system, upload│
└────────────────────────────────────────────────────────────────────────┘
```

---

## 3. 详细设计与核心组件规范 (Detailed Specifications)

### 3.1 微内核层 (`backend/core/`)

#### 1. 事件总线四大分发语义 (`core/events.go`)
- `Emit(ctx context.Context, topic string, payload any) error`：通知型广播，不短路，收集所有 Handler 产生的 error（`errors.Join`）。
- `Waterfall(ctx context.Context, topic string, initialPayload any) (any, error)`：链式流水线改写，上一个 handler 的返回值作为下一个 handler 的输入；一旦 handler 返回 error 立即短路中断并返回。
- `Parallel(ctx context.Context, topic string, payload any) error`：并发扇出执行所有 handler，通过 goroutine + WaitGroup 并发执行，收集所有 error。
- `Serial(ctx context.Context, topic string, payload any) error`：严格按序流水线执行所有 handler，遇到第一个 error 立即短路中断。

#### 2. 插件作用域上下文 (`core/context.go` & `core/app.go`)
- `App.ApplyPlugins()` 为每个插件生成专属的 `pluginCtx := app.ctx.Fork()`，并在 `Apply(pluginCtx)` 中挂载。
- Scoped Context 拥有独立的 `disposers`、`values` 与子 container，当插件被卸载或 context 被 dispose 时，仅回收该插件范围内的资源。
- 在 `Context` 上扩展 `ctx.DB()` 与 `ctx.Cache()` 辅助方法，内部通过 `core.Inject[contracts.DBService](c)` 与 `core.Inject[contracts.CacheService](c)` 解析。

#### 3. 扩展点可逆化与注销 (`core/extpoints/`)
- `RouterExtension`：注册路由时返回 `Disposer`；`RouterRegistry` 内部维护带 ID 的路由列表，支持动态移除路由。
- `TaskExtension` / `ScheduleExtension` / `SettingExtension` / `MigrationExtension`：提供与 Scoped Context 关联的注销机制与 Disposer 回收。

---

### 3.2 服务契约层 (`backend/core/contracts/`)

#### 1. `contracts.UserService` 扩展
收拢所有用户管理操作：
```go
type UserService interface {
    GetByID(ctx context.Context, id uint64) (*UserDTO, error)
    GetByUsername(ctx context.Context, username string) (*UserDTO, error)
    GetByEmail(ctx context.Context, email string) (*UserDTO, error)
    Create(ctx context.Context, user *UserDTO, password string) (*UserDTO, error)
    Update(ctx context.Context, user *UserDTO) error
    Delete(ctx context.Context, id uint64) error
    // Admin 扩展方法
    AdminListUsers(ctx context.Context, req AdminListUsersRequest) (int64, []*UserDTO, error)
    AdminGetUser(ctx context.Context, id uint64) (*UserDTO, error)
    AdminCreateUser(ctx context.Context, req AdminCreateUserRequest) (*UserDTO, error)
    AdminUpdateUser(ctx context.Context, currentUserID uint64, req AdminUpdateUserRequest) error
    AdminUpdateUserStatus(ctx context.Context, id uint64, active bool) error
    AdminDeleteUser(ctx context.Context, currentUserID, targetID uint64) error
}
```

#### 2. `contracts.AuthService` 扩展
收拢令牌管理与认证源查询：
```go
type AuthService interface {
    Authenticate(ctx context.Context, username, password string) (*UserDTO, error)
    GenerateToken(ctx context.Context, userID uint64, opts ...TokenOption) (string, error)
    ValidateToken(ctx context.Context, token string) (*TokenClaims, error)
    RevokeToken(ctx context.Context, tokenHash string) error
    RevokeUserTokens(ctx context.Context, userID uint64) error
    InvalidateCachedUser(ctx context.Context, userID uint64)
    InvalidateCachedToken(ctx context.Context, tokenHash string)
}
```

#### 3. 强类型领域事件 (`core/contracts/events.go`)
- `EventUserUpdated`: `{ UserID, UpdatedFields }`
- `EventUserDeleted`: `{ CurrentUserID, TargetUserID }`
- `EventUserStatusChanged`: `{ UserID, IsActive }`
- `EventTokenRevoked`: `{ UserID, TokenHash }`

---

### 3.3 基础设施去全局化 (`backend/plugins/infra/`)

1. **`plugins/infra/database`**：
   - 彻底去全局化：弃用全局静态变量直读，统一提供 `contracts.DBService` 实例并在 `Apply` 中 `core.Provide[contracts.DBService](ctx, svc)`。
2. **`plugins/infra/cache`**：
   - 弃用全局 `cache.Client()` 包级直连，统一通过 `contracts.CacheService` 接口与 `ctx.Cache()` 操作。

---

### 3.4 业务域插件边界治理 (`backend/plugins/domain/`)

1. **`domain/admin` 治理**：
   - 移除全部跨插件直接导入（`domain/auth`、`domain/risk_control`、`domain/cap`、`drivers/driver_asynq_*`、`infra/database`）。
   - 用户管理委派给 `contracts.UserService`。
   - 认证与令牌操作委派给 `contracts.AuthService`。
   - 配置管理通过 `ctx.Settings()` / `contracts.SettingService`。
2. **表单一所有者原则（Single Owner Principle）**：
   - `w_users` 表有且仅由 `domain/user` 插件读写。
   - `w_access_tokens` / `w_auth_sources` / `w_external_accounts` 表有且仅由 `domain/auth` 插件读写。
   - `w_system_configs` 表由 `domain/admin` 维护。

---

## 4. 实施与验证流程 (Verification & Quality Gates)

1. **内核与契约单测**：
   - `core/events_test.go`：覆盖 `Emit`、`Waterfall`、`Parallel`、`Serial`。
   - `core/context_test.go`：覆盖 Scoped Fork、Disposer LIFO、扩展点注销。
2. **全局代码检查与格式化**：
   - `make code-check`
   - `make format`
   - `go test -v -race ./backend/...`
