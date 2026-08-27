# WAVELET 项目白皮书与开发指南 (White Paper & Official Developer Guide)

- **版本**: v1.0.0 (Cordis Architecture Standard)
- **维护者**: Wavelet 核心架构团队
- **更新策略**: 每一个核心 Task / Subagent 交付后实时同步更新
- **最新同步时间**: 2026-08-27 (Task 1 Complete)

---

## 1. 架构总览与设计哲学 (Architecture Overview & Philosophy)

Wavelet 是一套面向企业级高并发、多业务灵活拓展的 **微内核 + 全插件化 (Cordis-like Architecture)** 平台底座。

### 1.1 核心设计理念
1. **微内核 (Micro-Kernel)**: 内核仅维护统一上下文 (`Context`)、泛型服务定位与 IoC 容器、生命周期状态机与扩展点协议。
2. **一切皆插件 (All-in-Plugins)**: HTTP Server、Asynq Worker、Cron Scheduler、数据库、缓存、日志、鉴权、业务能力均为挂载在 Context 上的对等插件。
3. **扁平自包含 (Flat & Self-Contained)**: 插件内部就近组织路由、Handler、模型与专属数据迁移，拒绝臃肿冗余的 DDD 分层样板代码。
4. **编译期依赖组合 (Compile-time Composition)**: 下游通过 `app.Use(&plugin)` 自由裁剪组装，编译为单一静态高性能二进制。
5. **微服务就绪 (Monolith-First, Microservice-Ready)**: 模块间强类型接口解耦，单体下零开销内存调用，高并发下支持通过 RPC Client 插件无缝拆分为微服务。

---

## 2. 核心模块与最新建设进度 (Core Modules & Progress)

| 模块层级 | 模块名称 | 职责定位 | 当前状态 | 覆盖测试 / 成果 |
| :--- | :--- | :--- | :--- | :--- |
| **Core** | `core/` | 泛型 IoC 容器、Context 树形上下文、Disposer 级联销毁 | ✅ 已完成 (Task 1) | 覆盖率 96.2%，commit `a6ab158` |
| **Core** | `core/extpoints/` & `events.go` | 强类型 EventBus、6 大领域扩展点 (Router/Migration/Task/Schedule/Setting) | 🟡 构建中 (Task 2) | Subagent 并行开发中 |
| **Drivers** | `plugins/drivers/` | Gin HTTP Server、Asynq Worker、Asynq Scheduler 运行时驱动 | 🟡 构建中 (Task 3) | Subagent 并行开发中 |
| **Infra** | `plugins/infra/` | Database (GORM/DBResolver)、Cache (RAM/Redis/PubSub)、Logger、Storage | 🟡 构建中 (Task 4) | Subagent 并行开发中 |
| **Domain** | `plugins/domain/` | Auth、User、MessageGateway、RiskControl、Admin 自包含插件 | ⚪ 待构建 (Task 5) | - |
| **Runtime**| `core/app.go` & `cmd/` | 统一启动器与运行切面分发器 (API/Worker/Schedule/All) | ⚪ 待构建 (Task 6) | - |
| **Downstream**| `downstream/` | 下游二次开发项目模板与自定义业务插件脚手架 | ⚪ 待构建 (Task 7) | - |

---

## 3. 开发者实战快速指引 (Quick Start Guide)

*(伴随各 Task 的推进，此章节持续追加最新最全的实战代码示例)*

