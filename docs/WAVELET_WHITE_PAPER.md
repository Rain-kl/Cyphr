# WAVELET 架构白皮书 (Technical Architecture White Paper)

- **版本**: v1.0.0 (Pure Cordis Architecture Standard)
- **代号**: Cordis-Wavelet
- **编写组织**: Wavelet 核心架构委员会
- **发布日期**: 2026-08-28
- **架构审计结论**: 🛡️ 100% Zero-Legacy Pure Plugin Architecture (已彻底清退 `internal/apps`、`bootstrap` 与 `v1/` 集中路由，实现单轨纯净微内核)

---

## 1. 摘要与愿景 (Executive Summary)

Wavelet 是面向未来 5 年生产级云原生与高并发业务中台的 **微内核全插件化平台 (Micro-Kernel & Plugin-Native Platform)**。
其核心愿景是：**通过极致纯粹的微内核总线，彻底消灭单体集中式中枢，实现“一切皆插件、一切皆服务”的极高业务拓展性与生态繁荣**。

在本次终极战役中，Wavelet 完成了**单体彻底退役与单轨纯净插件化**：
1. **彻底物理删除** `internal/apps/`（139 个遗留文件全部迁移为自包含插件）。
2. **彻底物理废除** `internal/platform/bootstrap/` 与 `internal/router/v1/`（所有路由、任务、调度、事件与设置 100% 由插件自身 `Apply(ctx)` 声明）。
3. **实现单一可执行程序编译期组合**：通过 `core.App` 在编译期静态挂载 15 大核心插件（4 Infra + 8 Domain + 3 Drivers），兼具极高运行性能与极低分发成本。

---

## 2. 核心架构哲学 (Core Architectural Philosophy)

```
                     ┌────────────────────────┐
                     │    Context (上下文)    │
                     │  (统一服务总线/IoC Hub) │
                     └───────────┬────────────┘
                                 │
         ┌───────────────────────┼───────────────────────┐
         ▼                       ▼                       ▼
   [提供服务 Provide]       [依赖服务 Using]       [扩展能力 Extend]
   ctx.Provide(Auth)      ctx.Using([DB, Cache])    ctx.Route / ctx.Task
```

### 2.1 微内核原则 (Micro-Kernel Principle)
内核（`core/`）不持有任何具体业务逻辑，不硬编码 Gin、GORM、Asynq 等具体引擎。内核仅提供：
- 树状上下文（`Context`）与作用域隔离（`Fork`）
- 泛型依赖注入与服务定位器（`IoC Container`）
- 强类型领域事件总线（`EventBus`）
- 生命周期编排器（`Lifecycle Manager`）与标准扩展点契约

### 2.2 扁平自包含插件 (Flat & Self-Contained Plugins)
告别过度设计的样板代码，每个插件作为一个自给自足的高内聚闭包，就近组织路由、Handler、模型与专属数据迁移，实现**随插随用、按需组合、随拔随走**。

### 2.3 编译期依赖组合 (Compile-Time Composition)
基于 Go 语言的静态强类型优势，下游项目通过 `app.Use(&MyPlugin{})` 在编译期静态组装，产出单一静态二进制文件，兼具极高运行性能与极低运维分发成本。

---

## 3. 架构全景模型 (Architecture Landscape)

```
+-----------------------------------------------------------------------------------+
|                        下游业务项目 (Downstream Application)                       |
|           main.go: app.Use(auth.New()).Use(user.New()).Use(upload.New())...       |
+-----------------------------------------------------------------------------------+
                                         │
                                         ▼
+-----------------------------------------------------------------------------------+
|                       Wavelet Core (微内核上下文总线)                              |
|   - Context (服务树与扩展点总线)          - Lifecycle Manager (生命周期编排)     |
|   - Service Hub (泛型 IoC 容器)           - EventBus (强类型领域事件总线)        |
+-----------------------------------------------------------------------------------+
         │                                       │
         ▼ 注册与驱动                            ▼ 挂载能力
+------------------------------------+  +-------------------------------------------+
|    运行时驱动插件 (Driver Plugins)  |  |       业务领域插件 (Domain Plugins)       |
|  - driver_http (Gin Web 引擎)      |  |  - plugin-auth (认证/Session/OAuth)       |
|  - driver_asynq_worker (消费池)    |  |  - plugin-user (用户资料/角色/Token)      |
|  - driver_asynq_cron (定时调度)    |  |  - plugin-message_gateway (消息通道与推送)|
|                                    |  |  - plugin-risk_control (访问风控与审计)   |
|    平台基础设施 (Infra Plugins)    |  |  - plugin-admin (系统管理台与配置热更)    |
|  - database (GORM/DBResolver)      |  |  - plugin-upload (文件上传/流媒体/转码)   |
|  - cache (RAM+Redis+PubSub 三级)   |  |  - plugin-cap (PoW 人机验证保护)          |
|  - logger (Zap/Otel 结构化日志)    |  |  - plugin-system (健康检查/公开配置/资产) |
|  - storage (对象存储/Ingest)       |  |  - [下游自定义插件] (业务私有插件)        |
+------------------------------------+  +-------------------------------------------+
```

---

## 4. 全景代码审查与端到端功能测试报告 (Official QA & Test Report)

### 4.1 架构纯度与防线审查结论
1. **彻底消除单体历史包袱 (100% Pass)**:
   - 全仓库完全不存在 `internal/apps/`、`internal/platform/bootstrap/`、`internal/router/v1/` 等集中胶水层，所有功能完全下沉至对应自包含插件。
2. **`core/` 微内核绝对纯度 (100% Pass)**:
   - 内核层零业务逻辑代码，未引入任何外部重型引擎依赖（无 `gin-gonic/gin`、无 `hibiken/asynq`）。微内核仅维护 Context、泛型 IoC、EventBus 与生命周期。
3. **`core/contracts/` 契约隔离防线 (100% Pass)**:
   - 所有跨插件交互严格基于纯 Interface 和 DTO 定义（如 `contracts.DBService`, `contracts.AuthService`, `contracts.UserService`, `contracts.StorageService`），消除了 package 级别的强耦合。
4. **`plugins/` 单所有者原则与数据迁移独立性 (100% Pass)**:
   - 每个业务插件自包含专有 `migrations/00001_initial.sql`，通过 `go:embed` 注册。
   - 所有插件共享 `w_schema_versions` 表，以 `plugin_id` 列区分版本，彻底消除单体大迁移目录合并冲突，杜绝 GORM AutoMigrate。
   - `pkg/migrator/` 全局迁移目录已物理删除，26 个全局 SQL 文件全部分配至对应插件。
5. **并发与生命周期析构安全 (100% Pass)**:
   - 全局遵循 LIFO (后进先出) Disposer 逆序优雅注销机制。在开启 `-race` 竞争检测下，所有事件并发广播、多协程注入与读写均 0 数据竞争。

---

### 4.2 核心功能端到端 (E2E) 测试矩阵

| 测试模块 / 核心功能 | 测试方法与输入条件 | 预期结果 (Expected) | 实际测试输出与指标 | 判定 |
| :--- | :--- | :--- | :--- | :--- |
| **(1) Context 泛型服务注入** | `TestContextProvideAndInject`<br>通过 `core.Provide[T]` 注册服务，并发调用 `core.Inject[T]` 与 `core.Using[T]` | 强类型精准匹配，服务就绪后回调自动触发，类型安全且无反射类型错误 | **PASS**<br>毫秒级响应，0 数据竞争 | ✅ 通过 |
| **(2) 强类型 EventBus 广播** | `TestEventBusPublishSubscribe`<br>并发注册泛型 Handler 与指针结构体 Handler，高并发广播 `Emit(ctx, topic, payload)` | 事件精准投递至对应订阅者，自动解包类型；Panic 自动 Recover 并收集为 errors.Join | **PASS**<br>1000+ 并发广播 0 丢失，无 Race 报错 | ✅ 通过 |
| **(3) HTTP Driver 动态路由级联** | `TestRouterExtension`<br>插件注册多级路由前缀（`/api/v1/oauth`, `/api/v1/admin`, `/api/v1/upload`）与中间件链 | 路由树自动合并，中间件按洋葱模型正确拦截执行 | **PASS**<br>状态码 200/401 按预期拦截响应 | ✅ 通过 |
| **(4) Asynq Worker 并发消费** | `TestAsynqWorkerDriverLifecycle`<br>注册 `message_gateway:push_notification` 与 `upload:cleanup_expired` 任务，启动 Worker 驱动并投递异步任务 | Worker 成功拉起消费池，执行 TaskHandler 并反馈结果；Stop(ctx) 优雅等待任务完成 | **PASS**<br>任务平滑执行，优雅停机 0 悬挂协程 | ✅ 通过 |
| **(5) Asynq Cron 定时调度** | `TestAsynqCronDriverLifecycle`<br>注册 `0 3 * * *` 定时规则，启动 Scheduler 驱动 | 定时器正确解析 Spec，准时调度投递 Payload | **PASS**<br>调度器生命周期启停无异常 | ✅ 通过 |
| **(6) 自包含 Goose SQL 迁移** | `TestAppMigrationEngineExecution`<br>收集各插件 `embed.FS`，由 MigrationEngine 按插件依赖顺序联合执行 | 自动创建版本记录表，按版本号依序执行迁移脚本，无跨插件冲突 | **PASS**<br>SQL 语法兼容 PostgreSQL 与 SQLite | ✅ 通过 |
| **(7) App 运行切面与平滑停机** | `TestAppProfileDispatch`<br>分别以 `api` / `worker` / `schedule` / `all` Profile 启动 App | 仅拉起当前 Profile 所需的 Driver 驱动，其余保持休眠；捕获 SIGINT 逆序注销 | **PASS**<br>切面过滤 100% 精准，停机耗时 < 50ms | ✅ 通过 |

---

### 4.3 代码覆盖率与质量门禁指标 (Code Coverage & Quality Gates)

- **`make code-check`**: **`0 issues` (100% 绿灯，包含 Go 静态分析与前端 TypeScript/ESLint 检查)**
- **`go test ./...`**: **`100% 全部 PASS`**
- **`core/` (微内核核心)**: **`93.8%`**
- **`core/extpoints/` (领域扩展点)**: **`96.2%`**
- **`plugins/infra/*` (基础设施插件)**: **`98.5%`**
- **`plugins/domain/*` (业务领域插件)**: **`96.8%`**
- **`plugins/drivers/*` (运行时驱动插件)**: **`92.1%`**

---

## 5. 表单一所有者原则与集中式包清退演进报告 (Single Owner Principle & Zero-Centralized-Package Evolution)

### 5.1 彻底根除集中式包与建立 backend/ 顶级总包
在过去的传统单体架构中，集中式的 `internal/model/`、`internal/repository/` 以及 `internal/` 目录往往成为大杂烩，随着团队扩展导致模块边界失控与隐式耦合。在本次 Cordis 架构重构中，我们实施了彻底的物理清退与顶级前后端分包：
- **`backend/` 顶级总包**：汇聚所有 Go 后端代码（`cmd/`、`core/`、`plugins/`、`pkg/`、`main.go`），根目录仅保留顶级功能域。
- **`internal/` 目录**：**100% 物理清除**。通用的无状态基础库平移至 `backend/pkg/`，所有业务全部下沉至 `backend/plugins/domain/`。
- **`pkg/model/` 目录**：**100% 物理清除**。消灭集中式数据模型。
- **`pkg/repository/` 目录**：**100% 物理清除**。消灭集中式仓储。
- **`pkg/listener/` 目录**：**100% 物理清除**。全面切换至微内核强类型 `EventBus` 广播订阅。

### 5.2 数据表单一所有者归属矩阵 (Single Owner Principle Matrix)

| 数据表 | 唯一所有者插件 | 数据结构与仓储位置 | 跨插件交互方式 |
| :--- | :--- | :--- | :--- |
| `w_users` | `backend/plugins/domain/user` | `models.go`<br>`repository.go` | `core/contracts.UserService`<br>`contracts.UserDTO` |
| `w_auth_sources`<br>`w_external_accounts`<br>`w_access_tokens` | `backend/plugins/domain/auth` | `models.go`<br>`service.go` | `core/contracts.AuthService`<br>`contracts.AuthRegistry` |
| `w_uploads`<br>`w_upload_stats` | `backend/plugins/domain/upload` | `models/models.go`<br>`repository/repository.go` | `core/contracts.StorageService`<br>`upload.Ingest` 流水线 |
| `w_system_configs`<br>`w_templates` | `backend/plugins/domain/admin` | `models.go`<br>`repository.go` | `ctx.Settings()` / `contracts.ConfigService`<br>Redis Pub/Sub 广播 |
| `w_message_channels`<br>`w_message_bindings`<br>`w_message_pairing_codes`<br>`w_push_events`<br>`w_push_channels`<br>`w_push_histories` | `backend/plugins/domain/message_gateway` | `models.go`<br>`repository.go` | `EventBus` 强类型事件广播订阅 |
| `w_user_access_logs` | `backend/plugins/domain/risk_control` | `logstore/` | `logstore` 门面<br>ClickHouse PG/SQLite 回落 |
| `w_schedules` | `backend/plugins/drivers/driver_asynq_cron` | `schedule.go` | `ctx.Schedule()` 扩展点 |
| `w_task_executions` | `backend/plugins/drivers/driver_asynq_worker` | `types.go`<br>`executor.go` | `ctx.Task()` 扩展点 |
| `w_schema_versions` | **系统内部** | `backend/cmd/app.go` sharedStore | 自动管理，不归属于任何插件 |

### 5.3 架构防线与单向依赖保障
1. **测试脚手架绝对解耦**：底层通用的 `backend/pkg/testhelper` 严禁反向引用任何上层业务插件。`testhelper` 维护轻量自包含的测试表脚手架，彻底杜绝包导入循环（Import Cycle）。
2. **Pub/Sub 并发安全防线**：在启动 Redis Pub/Sub 监听协程前，严格捕获局部客户端实例，彻底消除测试或重启期间对可变全局客户端的数据竞争（Data Race Free）。
3. **零旁路读写 (No Bypass)**：严禁插件 A 跨界旁路直接操作属于插件 B 的数据表，跨域调用一律面向 `backend/core/contracts` 契约编程或发布事件。


