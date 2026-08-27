# Wavelet Cordis 微内核与全插件化改造实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 Wavelet 架构重构为基于 Cordis 理念的微内核与全插件化架构，支持一切能力插件化、自包含数据迁移、多运行切面（API/Worker/Schedule/All）与下游极简二开扩展。

**Architecture:** 
- **Core (`core/`)**: 纯净微内核，提供 Context 服务总线、泛型 IoC 容器（`Provide/Inject/Using`）、强类型 EventBus、生命周期状态机与 6 大扩展点协议，零外部业务依赖。
- **Drivers (`plugins/drivers/`)**: Gin HTTP Server、Asynq Worker、Asynq Scheduler 封装为标准运行时驱动插件。
- **Infra Plugins (`plugins/infra/`)**: 数据库（GORM/DBResolver）、三层缓存（RAM/Redis/PubSub）、日志（Zap/Otel）、对象存储插件化。
- **Domain Plugins (`plugins/domain/`)**: Auth、User、MessageGateway、RiskControl、Admin 模块拆分为扁平自包含插件，自带独立 Goose 迁移。

**Tech Stack:** Go 1.25+, Gin, GORM, Asynq, Redis, Zap, OpenTelemetry, Goose, Viper, Cobra.

## Global Constraints
- 保持 `core/` 绝对纯净，禁止 import Gin、GORM、Asynq 或具体业务包。
- 插件之间严禁相互跨包 import 具体实现，跨插件交互一律通过 `core/contracts` 接口或 `ctx.Events()` 事件总线。
- 严格遵循 Go 单元测试规范，测试临时目录统一使用 `t.TempDir()`，测试覆盖率严格达标。
- 完成每个 Task 后必须确保代码能通过 `go build ./...` 与 `go test ./...` 检验并及时提交 Git。

---

### Task 1: 微内核基础契约与泛型 Context 服务总线 (`core/`)

**Files:**
- Create: `core/types.go`
- Create: `core/manifest.go`
- Create: `core/container.go`
- Create: `core/context.go`
- Test: `core/context_test.go`

**Interfaces:**
- Produces: `core.Plugin`, `core.Manifest`, `core.Context`, `core.Provide[T]`, `core.Inject[T]`, `core.Using[T]`

- [ ] **Step 1: 编写 Context 与 IoC 容器的失败测试**

```go
// core/context_test.go
package core_test

import (
	"context"
	"testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/Rain-kl/Wavelet/core"
)

type SampleService interface {
	Greet(name string) string
}

type sampleServiceImpl struct{}

func (s *sampleServiceImpl) Greet(name string) string {
	return "Hello, " + name
}

func TestContextProvideAndInject(t *testing.T) {
	ctx := core.NewContext(context.Background())
	core.Provide[SampleService](ctx, &sampleServiceImpl{})

	svc, err := core.Inject[SampleService](ctx)
	require.NoError(t, err)
	assert.Equal(t, "Hello, Wavelet", svc.Greet("Wavelet"))
}

func TestContextUsing(t *testing.T) {
	ctx := core.NewContext(context.Background())
	var called bool

	err := core.Using(ctx, func(s SampleService) {
		called = true
		assert.Equal(t, "Hello, Cordis", s.Greet("Cordis"))
	})
	assert.Error(t, err, "service not ready yet")
	assert.False(t, called)

	core.Provide[SampleService](ctx, &sampleServiceImpl{})
	err = core.Using(ctx, func(s SampleService) {
		called = true
		assert.Equal(t, "Hello, Cordis", s.Greet("Cordis"))
	})
	assert.NoError(t, err)
	assert.True(t, called)
}
```

- [ ] **Step 2: 运行测试验证失败**

Run: `go test -v ./core`
Expected: FAIL with compilation error (package not found).

- [ ] **Step 3: 实现 Core 核心接口与泛型容器**

编写 `core/types.go`、`core/manifest.go`、`core/container.go`、`core/context.go`，提供基于反射与类型推导的安全泛型服务存取、Scope 隔离与 Disposer 回调链。

- [ ] **Step 4: 运行测试验证通过**

Run: `go test -v ./core`
Expected: PASS

- [ ] **Step 5: 提交 Task 1 代码**

```bash
git add core/
git commit -m "feat(core): implement context service hub and generic ioc container"
```

---

### Task 2: 领域扩展点规范与强类型 EventBus (`core/extpoints/`, `core/events.go`)

**Files:**
- Create: `core/events.go`
- Create: `core/extpoints/router.go`
- Create: `core/extpoints/migration.go`
- Create: `core/extpoints/task.go`
- Create: `core/extpoints/schedule.go`
- Create: `core/extpoints/setting.go`
- Test: `core/events_test.go`
- Test: `core/extpoints/extpoints_test.go`

**Interfaces:**
- Consumes: `core.Context`
- Produces: `core.EventBus`, `core.RouterExtension`, `core.MigrationExtension`, `core.TaskExtension`, `core.ScheduleExtension`, `core.SettingExtension`

- [ ] **Step 1: 编写 EventBus 与扩展点测试用例**

```go
// core/events_test.go
package core_test

import (
	"context"
	"testing"
	"github.com/stretchr/testify/assert"
	"github.com/Rain-kl/Wavelet/core"
)

type UserRegisteredEvent struct {
	UserID string
}

func TestEventBusPublishSubscribe(t *testing.T) {
	bus := core.NewEventBus()
	var receivedID string

	bus.On("user:registered", func(ctx context.Context, e UserRegisteredEvent) error {
		receivedID = e.UserID
		return nil
	})

	err := bus.Emit(context.Background(), "user:registered", UserRegisteredEvent{UserID: "u_999"})
	assert.NoError(t, err)
	assert.Equal(t, "u_999", receivedID)
}
```

- [ ] **Step 2: 运行测试验证失败**

Run: `go test -v ./core/...`
Expected: FAIL

- [ ] **Step 3: 实现 EventBus 与 6 大扩展点适配器**

编写 `core/events.go` 及 `core/extpoints/` 下各个领域的挂载收集器（Router 注册收集、Goose embed.FS 聚合器、Task/Schedule 声明表、Setting 模式注册表）。

- [ ] **Step 4: 运行测试验证通过**

Run: `go test -v ./core/...`
Expected: PASS

- [ ] **Step 5: 提交 Task 2 代码**

```bash
git add core/
git commit -m "feat(core): add typed eventbus and domain extension points"
```

---

### Task 3: 运行时驱动插件下沉 (`plugins/drivers/`)

**Files:**
- Create: `plugins/drivers/driver_http/plugin.go`
- Create: `plugins/drivers/driver_asynq_worker/plugin.go`
- Create: `plugins/drivers/driver_asynq_cron/plugin.go`
- Test: `plugins/drivers/drivers_test.go`

**Interfaces:**
- Consumes: `core.Plugin`, `core.Driver`, `core.Context`
- Produces: `DriverTypeHTTP`, `DriverTypeWorker`, `DriverTypeScheduler`

- [ ] **Step 1: 编写 Driver 生命周期测试用例**

测试驱动在接收到 `Start(ctx)` 和 `Stop(ctx)` 信号时的平滑启动与退出状态。

- [ ] **Step 2: 编写 Driver 实现**

将 Gin HTTP Server、Asynq Worker Server、Asynq Scheduler 封装为标准 `core.Driver`，并在 `Apply(ctx)` 时挂载到 Context 驱动树。

- [ ] **Step 3: 运行驱动单元测试**

Run: `go test -v ./plugins/drivers/...`
Expected: PASS

- [ ] **Step 4: 提交 Task 3 代码**

```bash
git add plugins/drivers/
git commit -m "feat(plugins): implement runtime drivers for http, asynq worker, and cron"
```

---

### Task 4: 基础设施服务插件化 (`plugins/infra/`)

**Files:**
- Create: `plugins/infra/database/plugin.go` (提供 GORM DBService)
- Create: `plugins/infra/cache/plugin.go` (提供 RAM/Redis 三层缓存)
- Create: `plugins/infra/logger/plugin.go` (提供 Zap/Otel 结构化日志)
- Create: `plugins/infra/storage/plugin.go` (提供统一对象存储)
- Test: `plugins/infra/infra_test.go`

**Interfaces:**
- Produces: `contracts.DBService`, `contracts.CacheService`, `contracts.LoggerService`, `contracts.StorageService`

- [ ] **Step 1: 编写基础设施插件注入与提取测试**
- [ ] **Step 2: 实现 4 大基础设施插件并封装现有 pkg 与 infra 底座**
- [ ] **Step 3: 运行基础设施测试验证**

Run: `go test -v ./plugins/infra/...`
Expected: PASS

- [ ] **Step 4: 提交 Task 4 代码**

```bash
git add plugins/infra/
git commit -m "feat(plugins): package database, cache, logger, and storage as infra plugins"
```

---

### Task 5: 业务领域插件化重构 (`plugins/domain/`)

**Files:**
- Create: `plugins/domain/auth/` (认证、Session、Passkey、专属 migrations)
- Create: `plugins/domain/user/` (用户资料、角色权限、专属 migrations)
- Create: `plugins/domain/message_gateway/` (Bot网关、推送通道、Worker消费)
- Create: `plugins/domain/risk_control/` (IP限流、风控中间件)
- Create: `plugins/domain/admin/` (控制台、系统设置)
- Test: `plugins/domain/domain_test.go`

**Interfaces:**
- Consumes: `contracts.DBService`, `contracts.CacheService`, `contracts.LoggerService`
- Produces: `contracts.AuthService`, `contracts.UserService`

- [ ] **Step 1: 编写 Auth 与 User 插件业务装配与独立迁移测试**
- [ ] **Step 2: 将各业务模块迁移为扁平自包含插件，嵌入专属 Goose SQL 迁移**
- [ ] **Step 3: 运行业务插件集成测试**

Run: `go test -v ./plugins/domain/...`
Expected: PASS

- [ ] **Step 4: 提交 Task 5 代码**

```bash
git add plugins/domain/
git commit -m "feat(plugins): migrate auth, user, message_gateway, risk_control, admin to domain plugins"
```

---

### Task 6: 统一装配入口与运行时切面分发器 (`core/app.go`, `cmd/`)

**Files:**
- Create: `core/app.go`
- Modify: `internal/cmd/root.go`
- Modify: `internal/cmd/api.go`
- Modify: `internal/cmd/worker.go`
- Modify: `internal/cmd/scheduler.go`
- Modify: `internal/cmd/all.go`
- Test: `core/app_test.go`

**Interfaces:**
- Consumes: `core.App`, `core.Plugin`, `core.Driver`
- Produces: 统一 CLI 启动与优雅停机流程

- [ ] **Step 1: 编写 App 生命周期与 Profile 调度测试**
- [ ] **Step 2: 实现 `core.App` 编排引擎，无缝接入 `wavelet api / worker / schedule / all`**
- [ ] **Step 3: 运行启动与角色切面集成验证**

Run: `go test -v ./core -run TestAppProfileDispatch`
Expected: PASS

- [ ] **Step 4: 提交 Task 6 代码**

```bash
git add core/ internal/cmd/
git commit -m "feat(core): implement app profile lifecycle dispatcher and wire cli commands"
```

---

### Task 7: 下游脚手架、自定义示例插件与端到端验证

**Files:**
- Create: `downstream/custom_plugins/order/plugin.go`
- Create: `downstream/main.go`
- Create: `downstream/config.yaml`
- Test: `downstream/e2e_test.go`

- [ ] **Step 1: 编写下游自定义业务插件并在下游 `main.go` 组装启动**
- [ ] **Step 2: 执行全量 E2E 测试，验证数据迁移、HTTP 路由访问、Worker 任务消费与平滑停机**
- [ ] **Step 3: 运行全局质量门禁检查**

Run:
```bash
make test
make code-check
make format
```
Expected: 全部 PASS，0 lint 报错。

- [ ] **Step 4: 提交 Task 7 代码**

```bash
git add downstream/
git commit -m "feat(downstream): add starter scaffold, example custom plugin, and e2e tests"
```

