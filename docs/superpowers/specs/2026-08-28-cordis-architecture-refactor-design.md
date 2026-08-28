# Cordis 架构重构设计规格书 (Cordis Architecture Refactor Design)

**日期**: 2026-08-28  
**目标**: 依据 Cordis 时空可组合性元框架（Spatiotemporal Composability）哲学，重构 Wavelet 后端包结构、包职责与插件边界，消除全局静态单例与跨插件私有实现依赖，实现真正的可逆副作用与契约化隔离。

---

## 1. 背景与核心设计原则

Cordis 是一个面向时空可组合性的元框架，核心在于：
1. **时间可组合性 (Temporal Composability / Revertible Effects)**：组件挂载到上下文时产生的任何副作用（数据库连接、Redis 客户端、路由、事件监听、定时任务）必须具备明确的逆操作，在卸载时按 LIFO（后进先出）干净撤销。
2. **空间可组合性 (Spatial Composability / Reactive Coeffects)**：组件通过 `Inject` 声明依赖；无特权微内核，所有基础设施与业务均以平等插件形态存在；组件之间严格面向抽象服务契约（Contracts）编程，严禁跨包引用私有实现。
3. **合流定理 (Confluence)**：任何插件的装载/卸载顺序，静止状态等同于从零静态装配，杜绝全局隐藏状态与启动顺序隐式假设。

---

## 2. 详细重构方案

### 2.1 微内核纯洁化 (`backend/core/`)

#### 改造点：
1. **移除特权辅助方法**：
   - 从 `backend/core/context.go` 中移除 `func (c *Context) DB() contracts.DBService` 与 `func (c *Context) Cache() contracts.CacheService`。
   - 所有服务消费方统一面向 `core.Inject[T](ctx)`、`core.MustInject[T](ctx)` 或 `core.Using[T](ctx, ...)`。
2. **保持依赖注入纯粹性**：
   - 内核仅保留：`Context`、`Container`、`Fiber`、`EventBus`、生命周期管理以及通用的扩展点挂载。

---

### 2.2 基础设施插件生命周期可逆化 (`backend/plugins/infra/`)

#### 1. 数据库插件 (`plugins/infra/database`)
- **移除隐式副作用**：
  - 删除 `postgres.go` 与 `sqlite.go` 中的 `func init() { ... }` 静态建连。
  - 删除包级导出的静态全局变量 `var db *gorm.DB` 以及全局 `DB(ctx)` / `SetDB()`。
- **生命周期受控与可逆释放**：
  - 在 `Plugin.Apply(ctx *core.Context)` 时根据配置建立数据库连接（GORM + underlying `*sql.DB`）。
  - 创建 `contracts.DBService` 实例并通过 `core.Provide[contracts.DBService](ctx, svc)` 注册。
  - 注册 `ctx.OnDispose` 逆操作，在插件卸载时调用 `sqlDB.Close()`。

#### 2. 缓存插件 (`plugins/infra/cache`)
- **移除隐式副作用**：
  - 删除 `redis.go` 中的 `func init() { ... }` 静态建连。
  - 删除包级导出的全局变量 `var Redis redis.UniversalClient`。
- **生命周期受控与可逆释放**：
  - 在 `Plugin.Apply(ctx *core.Context)` 时初始化 Redis 客户端并构造 `contracts.CacheService`。
  - 通过 `core.Provide[contracts.CacheService](ctx, svc)` 注册。
  - 注册 `ctx.OnDispose` 逆操作，在插件卸载时调用 `client.Close()`。

---

### 2.3 业务领域插件防线隔离与依赖重构 (`backend/plugins/domain/`)

#### 1. 消除跨插件私有 Import
- 遍历并重构以下 8 个 Domain 插件：
  - `auth`
  - `user`
  - `admin`
  - `cap`
  - `message_gateway`
  - `risk_control`
  - `system`
  - `upload`
- **规则**：
  - 严禁任何 domain 插件 `import "Wavelet/plugins/infra/database"` 或 `import "Wavelet/plugins/infra/cache"`。
  - 严禁任何 domain 插件直接 import 另一个 domain 插件的具体实现包（如 `admin` 严禁 import `risk_control/logstore` 或 `storage/diskcache`）。
  - 各插件内部的 Repository / Service 统一通过 `core.Inject[contracts.DBService](ctx)` 或插件内部 scoped context 获取数据库连接。

#### 2. `admin` 插件解耦与全局变量清除
- 移除 `admin/plugin.go` 中的包级变量（`globalUserSvc`, `globalAuthSvc`, `globalCoreCtx`）。
- 将 `admin` 的日志查询、任务触发、缓存清理等管理接口改造为通过 `contracts` 或 `ctx.Tasks()` 访问，消除对 `risk_control`、`driver_asynq_worker` 等的私有依赖。

---

## 3. 验证与门禁标准

1. **编译与依赖检查**：
   - 运行 `grep -r "Wavelet/plugins/infra/database" backend/plugins/domain/` 结果为空。
   - 运行 `grep -r "Wavelet/plugins/infra/cache" backend/plugins/domain/` 结果为空。
2. **自动化测试**：
   - 所有既有单元测试与集成测试（`go test ./...`）无回归，全部 PASS。
3. **代码质量门禁**：
   - `make code-check` 静态检查 0 告警通过。
   - `make format` 格式化通过。
