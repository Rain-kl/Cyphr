# Transcribe 智能转录 SaaS 平台分步实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 完整构建并交付基于 Wavelet/Cordis 的 Transcribe SaaS 平台，包括 Controller 服务端插件、Python 异步推理 Agent、纯净独立 CLI 二进制工具（`bin/transcribe`），端到端跑通从音视频提取、任务调度、多任务并发转录到 SSE 实时日志流回传的全流程。

**Architecture:** 控制端与推理端遵循控制面（WebSocket 信令心跳与调度）与数据面（HTTP 音频拉取与批量日志上报）分离原则；CLI 工具使用独立轻量编译入口剥离服务端依赖；Agent 使用 Python 异步并发模型与全局异常盾牌；数据持久化采用双方言 Goose 迁移与 `t_` 前缀表设计。

**Tech Stack:** Go 1.24+, Gin, GORM, Gorilla WebSocket, Goose SQL (Postgres/SQLite3), Python 3.12+, uv, websockets, httpx, psutil, Cobra CLI, ffmpeg.

## Global Constraints

- 遵循 Cordis 微内核架构：插件间严禁非法直接 import 内部私有实现，面向 contracts 编程。
- 遵循单一所有者原则：数据表使用 `t_` 前缀（`t_models`, `t_nodes`, `t_jobs`, `t_job_logs`），由 Controller 插件独占维护。
- 禁止 GORM AutoMigrate，数据表结构统一由 Goose SQL 迁移维护并嵌入二进制。
- API 路由规范：彻底移除 `/transcribe` 前缀；管理端使用 `/controller/` 命名空间；转录接口对齐 OpenAI `/v1/audio/transcriptions` 规范。
- CLI 独立纯净构建：`make build-cli` 输出 `bin/transcribe`，严禁打包服务端重度依赖。
- 提交规范：每个功能点完成后执行 Git commit，遵循 Conventional Commits：`<type>(<scope>): <subject>`，禁止 push 远程。

---

### Task 1: Controller 数据持久层与双方言 Goose 迁移

**Files:**
- Create: `backend/transcribe/plugins/svr/migrations/postgres/00001_init_transcribe.sql`
- Create: `backend/transcribe/plugins/svr/migrations/sqlite/00001_init_transcribe.sql`
- Create: `backend/transcribe/plugins/svr/model/entity/model_entity.go`
- Create: `backend/transcribe/plugins/svr/model/entity/node_entity.go`
- Create: `backend/transcribe/plugins/svr/model/entity/job_entity.go`
- Create: `backend/transcribe/plugins/svr/model/entity/job_log_entity.go`
- Create: `backend/transcribe/plugins/svr/model/do/transcribe_dto.go`
- Create: `backend/transcribe/plugins/svr/consts/consts.go`
- Create: `backend/transcribe/plugins/svr/consts/errs.go`
- Create: `backend/transcribe/plugins/svr/dao/model_dao.go`
- Create: `backend/transcribe/plugins/svr/dao/node_dao.go`
- Create: `backend/transcribe/plugins/svr/dao/job_dao.go`
- Test: `backend/transcribe/plugins/svr/dao/dao_test.go`

**Interfaces:**
- Produces:
  - `entity.ModelEntity`: `TableName() string = "t_models"`
  - `entity.NodeEntity`: `TableName() string = "t_nodes"`
  - `entity.JobEntity`: `TableName() string = "t_jobs"`
  - `entity.JobLogEntity`: `TableName() string = "t_job_logs"`
  - `dao.ModelDAO`: `GetByName(ctx, name)`, `ListActive(ctx)`, `Create(ctx, model)`
  - `dao.NodeDAO`: `GetByTokenHash(ctx, hash)`, `GetByID(ctx, id)`, `ListAll(ctx)`, `Create(ctx, node)`, `UpdateLastSeen(ctx, id, ip)`
  - `dao.JobDAO`: `Create(ctx, job)`, `GetByID(ctx, id)`, `ListByUserID(ctx, uid, page, size, status)`, `UpdateStatus(ctx, id, status)`, `AppendLogs(ctx, jobID, logs)`

- [ ] **Step 1: 编写 PostgreSQL 与 SQLite3 迁移脚本**
  包含 `t_models`, `t_nodes`, `t_jobs`, `t_job_logs` 的建表与索引。在 `t_models` 中预置初始模型 `mock-whisper-base`。
- [ ] **Step 2: 编写 Entity 与 DAO 结构体及对应方法**
  实现防注入查询（`util.EscapeLike`），定义标准状态常量（`StatusPending`, `StatusRunning`, `StatusCompleted`, `StatusFailed`）。
- [ ] **Step 3: 编写 DAO 单元测试并在内存 SQLite 下执行验证**
  验证模型的创建、查询，节点的创建与 Token 匹配，Job 状态变更与日志批次写入。
- [ ] **Step 4: 运行测试确保 PASS**
  `cd backend && go test -v ./transcribe/plugins/svr/dao/...`
- [ ] **Step 5: Git Commit**
  `git add backend/transcribe/plugins/svr/migrations backend/transcribe/plugins/svr/model backend/transcribe/plugins/svr/consts backend/transcribe/plugins/svr/dao`
  `git commit -m "feat(transcribe): add database entities, goose migrations and dao layer"`

---

### Task 2: Controller 内存枢纽 AgentHub 与调度器 Scheduler

**Files:**
- Create: `backend/transcribe/plugins/svr/service/hub/session.go`
- Create: `backend/transcribe/plugins/svr/service/hub/agent_hub.go`
- Create: `backend/transcribe/plugins/svr/service/scheduler/scheduler.go`
- Create: `backend/transcribe/plugins/svr/service/log_broker.go`
- Create: `backend/transcribe/plugins/svr/service/node_service.go`
- Create: `backend/transcribe/plugins/svr/service/job_service.go`
- Test: `backend/transcribe/plugins/svr/service/service_test.go`

**Interfaces:**
- Consumes: Task 1 的 DAO 层与 Entity
- Produces:
  - `hub.AgentHub`: `RegisterSession(sess)`, `UnregisterSession(nodeID)`, `GetSession(nodeID)`, `ListActiveSessions()`, `BroadcastToNode(nodeID, msg)`
  - `scheduler.Scheduler`: `SchedulePendingJobs(ctx)`
  - `service.LogBroker`: `Subscribe(jobID) (<-chan LogMessage, cancelFunc)`, `Publish(jobID, msg)`
  - `service.NodeService`: `CreateNode(ctx, name) (*NodeDTO, rawToken, error)`, `VerifyNodeToken(ctx, rawToken) (*NodeDTO, error)`
  - `service.JobService`: `CreateJob(ctx, req) (*JobDTO, error)`, `GetJobDetail(ctx, id) (*JobDTO, error)`, `ListJobs(ctx, uid, page, size)`

- [ ] **Step 1: 编写 LogBroker 与 AgentHub 长连管理器**
  实现线程安全的 Session 映射、心跳更新、掉线检查与任务重置机制；实现 SSE 内存事件广播器。
- [ ] **Step 2: 编写 Scheduler 调度器与 NodeService / JobService**
  实现派发算法：优先匹配已加载模型且当前任务数最少的节点；未匹配则向空闲节点发送 `load_model` 指令；若无节点则保留 `pending`。
- [ ] **Step 3: 编写 Service 层单元测试**
  模拟 Agent 连接注册、心跳上报、任务派发信令与 SSE 订阅/发布广播。
- [ ] **Step 4: 运行测试确保 PASS**
  `cd backend && go test -v ./transcribe/plugins/svr/service/...`
- [ ] **Step 5: Git Commit**
  `git add backend/transcribe/plugins/svr/service`
  `git commit -m "feat(transcribe): implement agenthub, scheduler and logbroker services"`

---

### Task 3: Controller HTTP/WebSocket 控制器层与插件装配

**Files:**
- Create: `backend/transcribe/plugins/svr/controller/middleware.go` (Agent Token 鉴权中间件)
- Create: `backend/transcribe/plugins/svr/controller/openai_handler.go` (`POST /v1/audio/transcriptions`)
- Create: `backend/transcribe/plugins/svr/controller/job_handler.go` (`GET /api/v1/jobs`, `GET /api/v1/jobs/:id`, `GET /api/v1/jobs/:id/stream`)
- Create: `backend/transcribe/plugins/svr/controller/agent_handler.go` (`GET /api/v1/agent/ws`, `GET /api/v1/agent/jobs/:id/media`, `POST /api/v1/agent/jobs/:id/logs`, `POST /api/v1/agent/jobs/:id/complete`)
- Create: `backend/transcribe/plugins/svr/controller/controller_node_handler.go` (`GET /api/v1/controller/nodes`, `POST /api/v1/controller/nodes`, `POST /api/v1/controller/nodes/:id/load-model`, `POST /api/v1/controller/nodes/:id/unload-model`)
- Create: `backend/transcribe/plugins/svr/controller/model_handler.go` (`GET /api/v1/models`)
- Modify: `backend/transcribe/plugins/svr/plugin.go`
- Modify: `backend/cmd/app.go` (注册 `controller.New()`)
- Test: `backend/transcribe/plugins/svr/controller/controller_test.go`

**Interfaces:**
- Consumes: Task 1 & 2 的 Service/DAO，`contracts.AuthService`, `contracts.DBService`, `contracts.StorageService`
- Produces: 完整的 Controller 插件 HTTP & WebSocket 路由端点

- [ ] **Step 1: 编写 Agent Token 鉴权中间件与 WebSocket 升级处理**
  支持 `?token=` 与 `Authorization: Bearer <token>` 统一解析校验。
- [ ] **Step 2: 编写各 Handler**
  - `openai_handler.go`: 处理 multipart 文件上传，存储音频，根据 `X-Async: true` 决定返回 `job_id` 还是同步等待；
  - `job_handler.go`: 列表、详情、SSE 流式回传；
  - `agent_handler.go`: WS 消息分发（heartbeat, model_status）、音频分块下载、日志批量入库、任务结算；
  - `controller_node_handler.go`: 节点管理与手动模型加载/卸载控制指令。
- [ ] **Step 3: 更新 `plugin.go` 挂载路由与 Goose 迁移，并在 `cmd/app.go` 装配插件**
- [ ] **Step 4: 编写 API 接口测试（利用 httptest / gin.TestMode）确保 PASS**
  `cd backend && go test -v ./transcribe/plugins/svr/controller/...`
- [ ] **Step 5: Git Commit**
  `git add backend/transcribe/plugins/svr backend/cmd/app.go`
  `git commit -m "feat(transcribe): assemble controller routes, openai handler and agent ws endpoint"`

---

### Task 4: Python Agent 异步核心与 Mock ASR 引擎

**Files:**
- Create: `backend/agent/pyproject.toml`
- Create: `backend/agent/config.yaml`
- Create: `backend/agent/src/config.py`
- Create: `backend/agent/src/monitor.py`
- Create: `backend/agent/src/models/base.py`
- Create: `backend/agent/src/models/registry.py`
- Create: `backend/agent/src/models/mock_asr.py`
- Create: `backend/agent/src/reporter.py`
- Create: `backend/agent/src/job_runner.py`
- Create: `backend/agent/src/ws_client.py`
- Modify: `backend/agent/main.py`
- Test: `backend/agent/tests/test_agent.py`

**Interfaces:**
- Consumes: Controller 暴露的 `/api/v1/agent/ws` 与 HTTP 端点
- Produces: 可独立运行的 Python 异步推理节点，支持多任务并发与防崩溃保护

- [ ] **Step 1: 配置 `pyproject.toml` 与依赖**
  依赖 `websockets`, `httpx`, `psutil`, `pydantic`, `pytest`, `pytest-asyncio`。
- [ ] **Step 2: 编写系统指标监控 `monitor.py` 与 Mock ASR 引擎 `mock_asr.py`**
  实现硬件状态采集（CPU/RAM 利用率）；实现多阶段耗时模拟（音频加载、分块推理、时间戳对齐）并输出 OpenAI `verbose_json` 标准格式。
- [ ] **Step 3: 编写 HTTP 通信器 `reporter.py` 与任务执行池 `job_runner.py`**
  流式下载待转录音频；独立 `asyncio.Task` 执行；全局 `try-except` 异常捕获与失败结算上报。
- [ ] **Step 4: 编写 WebSocket 客户端 `ws_client.py` 与 `main.py`**
  自动重连循环、心跳发送协程、`dispatch_job` 与 `load_model` 指令派发。
- [ ] **Step 5: 编写 Python 单元测试并执行验证**
  `cd backend/agent && uv run pytest`
- [ ] **Step 6: Git Commit**
  `git add backend/agent`
  `git commit -m "feat(agent): implement python async agent with mock asr engine and fault tolerance"`

---

### Task 5: 纯净 CLI 独立工具 (`bin/transcribe`)

**Files:**
- Create: `backend/transcribe/plugins/cli/config/config.go`
- Create: `backend/transcribe/plugins/cli/cmd/root.go`
- Create: `backend/transcribe/plugins/cli/cmd/login.go`
- Create: `backend/transcribe/plugins/cli/cmd/asr.go`
- Create: `backend/transcribe/plugins/cli/cmd/jobs.go`
- Create: `backend/transcribe/plugins/cli/media/ffmpeg.go`
- Create: `backend/transcribe/plugins/cli/client/client.go`
- Create: `backend/cmd/transcribe/main.go`
- Modify: `Makefile` (新增 `build-cli` 编译指令)
- Test: `backend/transcribe/plugins/cli/media/ffmpeg_test.go`
- Test: `backend/transcribe/plugins/cli/cmd/cmd_test.go`

**Interfaces:**
- Consumes: Controller 暴露的用户/作业端点
- Produces: 单独分发的可执行文件 `bin/transcribe`

- [ ] **Step 1: 编写 CLI 配置读写 `config.go` 与媒体检查/转换 `ffmpeg.go`**
  配置保存于 `~/.transcribe/config.yaml`；媒体模块探测系统 `ffmpeg`，针对视频执行单声道 16kHz 提取并压缩为临时 `.mp3`。
- [ ] **Step 2: 编写 HTTP 客户端与 SSE 流消费解析器**
  支持长连读取 `text/event-stream`，打印实时日志行，优雅监听 SIGINT（`Ctrl+C`）中断并友好提示。
- [ ] **Step 3: 编写各子命令 (`login`, `asr`, `jobs ls`, `jobs log`)**
  - `transcribe login`: 校验凭证并保存配置；
  - `transcribe asr`: 预处理文件、上传提交任务、实时打印日志与最终转录文本；
  - `transcribe jobs ls`: 终端表格美化输出；
  - `transcribe jobs log`: 打印历史日志或 `-f` 实时追踪。
- [ ] **Step 4: 编写 `cmd/transcribe/main.go` 与 `Makefile` 的 `build-cli` 目标**
- [ ] **Step 5: 编写单元测试并执行验证**
  `cd backend && go test -v ./transcribe/plugins/cli/...`
- [ ] **Step 6: 执行 `make build-cli` 验证独立二进制输出**
  `make build-cli && ./bin/transcribe --help`
- [ ] **Step 7: Git Commit**
  `git add backend/transcribe/plugins/cli backend/cmd/transcribe Makefile`
  `git commit -m "feat(cli): implement standalone transcribe cli tool with ffmpeg extraction and sse streaming"`

---

### Task 6: 联调验证、质量门禁与端到端交付

**Files:**
- Test: `backend/transcribe/tests/e2e_test.go`

- [ ] **Step 1: 运行项目架构检查与代码风格门禁**
  `make code-check && make format`
- [ ] **Step 2: 运行全量后端测试**
  `cd backend && go test -v ./transcribe/...`
- [ ] **Step 3: 执行端到端集成联调流程**
  1. 启动 Controller 服务；
  2. 调用 Controller 接口创建节点并获取 `agent_token`；
  3. 配置并启动 Python Agent，观察 WS 连通与心跳日志；
  4. 使用 `bin/transcribe login` 配置连接；
  5. 准备测试音频/视频文件，运行 `bin/transcribe asr sample.mp4`；
  6. 验证终端实时输出 SSE 进度日志并在完成时输出格式化转录文本；
  7. 运行 `bin/transcribe jobs ls` 与 `bin/transcribe jobs log` 验证状态持久化。
- [ ] **Step 4: Git Commit**
  `git add .`
  `git commit -m "test(transcribe): verify e2e transcription pipeline and pass quality gate"`
