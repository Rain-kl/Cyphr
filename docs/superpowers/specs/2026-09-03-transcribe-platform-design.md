# Transcribe 智能转录 SaaS 平台架构设计说明书

## 1. 概述与设计目标

本项目是基于 Wavelet 微内核与 Cordis 插件体系构建的下游业务转录平台（Transcribe）。整体架构清晰解耦为三大核心组件：
1. **控制端 (Controller)**：运行于服务端的下游业务插件（`backend/transcribe/plugins/svr`），负责 Agent 节点管理、模型生命周期调度、转录任务状态机持久化、实时日志分发，对外暴露 OpenAI 兼容音频转录接口、用户作业查询端点及管理端节点调度端点。
2. **推理节点 (Python Agent)**：位于 `backend/agent/`，基于 Python 3.12+ 与 `uv` 包管理开发。主动向 Controller 建立 WebSocket 长连报告心跳（CPU/RAM/GPU 监控与已加载模型），接收任务派发与模型加载/卸载指令；通过专属 HTTP 端点流式下载音频，异步多任务并发执行转录（首版采用 Mock ASR 引擎），并通过 HTTP 端点分批上报执行日志与最终 OpenAI 格式结果。
3. **命令行工具 (CLI)**：独立的轻量 Go 二进制程序（`bin/transcribe`），剥离服务端数据库与 Redis 驱动，提供用户登录认证（`transcribe login`）、本地音视频格式智能检测与 `ffmpeg` 压缩、任务提交（`transcribe asr`）、实时 SSE 日志流式回传（支持 Ctrl+C 中断不影响后台任务）以及历史任务查看（`jobs ls`, `jobs log`）。

---

## 2. 整体架构与时序图

### 2.1 整体拓扑架构

```text
               +-------------------------------------------------------------+
               |                         Controller                          |
               |  +-------------------+  +---------------+  +-------------+  |
               |  | OpenAI API / Jobs |  |  Node Manager |  |  AgentHub   |  |
               |  +---------+---------+  +-------+-------+  +------+------+  |
               +------------|--------------------|-----------------|---------+
                            | (User Token)       | (Agent Token)   | (WS + HTTP)
                            ▼                    ▼                 ▼
                 +--------------------+                  +--------------------+
                 |    CLI Client      |                  |    Python Agent    |
                 | (bin/transcribe)   |                  |   (Inference Node) |
                 +--------------------+                  +--------------------+
```

### 2.2 核心作业时序流 (End-to-End Sequence)

```text
CLI (User)                   Controller (Server)                 Python Agent
    |                                 |                                |
    |                                 |<======== [WS Connect] =========| (带 agent_token 握手)
    |                                 |-------- 握手通过，记录节点 ------>|
    |                                 |<====== [WS Heartbeat] =========| (定时上报 CPU/RAM/模型)
    |                                 |                                |
    |--- 1. transcribe asr <file> --->|                                |
    |   (ffmpeg 本地提取转为 mp3)     |                                |
    |   (POST /v1/audio/transcriptions|                                |
    |    Header: X-Async: true)       |                                |
    |                                 |-- 2. 保存音频至 Storage        |
    |                                 |-- 3. 创建 Job (status=pending)-|
    |<-- 4. 返回 { "job_id": 10001 } -|                                |
    |                                 |                                |
    |--- 5. GET /jobs/10001/stream -->|                                |
    |   (SSE 长连监听日志)            |                                |
    |                                 |-- 6. 调度器匹配最优节点 -------|
    |                                 |====== [WS: dispatch_job] =====>| (下发任务元数据与下载路径)
    |                                 |                                |-- 7. 创建独立 asyncio.Task
    |                                 |<-- 8. GET /agent/jobs/1/media -| (HTTP 流式下载音频)
    |                                 |--- 9. 传输音频流 ------------->|
    |                                 |                                |-- 10. Mock ASR 引擎多阶段推理
    |                                 |<-- 11. POST /agent/jobs/1/logs-| (分批上报进度 30%, 60%)
    |<== 12. SSE 推送实时日志行 ======|                                |
    |    [16:00:01] Decoding 2/4...  |                                |
    |                                 |<-- 13. POST /agent/jobs/complete (结算 OpenAI 结果)
    |                                 |-- 14. 任务标记 completed ------|
    |<== 15. SSE 推送完成事件 ========|                                |
    |    (关闭 SSE 连接)              |                                |
    |-- 16. CLI 输出转录耗时与文本 ---|                                |
```

---

## 3. 双轨凭据与鉴权隔离模型

系统设计两套完全正交隔离的凭据体系，保障权限最小化与安全性：

### 3.1 用户侧凭据 (User Access Token)
- **生成途径**：通过既有的管理系统「个人设置 -> Access Token」界面生成。
- **验证机制**：基于 `contracts.AuthService.VerifyToken` 解析出当前操作的 `UserDTO`。
- **作用范围**：
  - `CLI 工具`：`transcribe login` 输入并保存在本地配置。
  - 用户接口：`POST /v1/audio/transcriptions`、`GET /api/v1/jobs`、`GET /api/v1/jobs/:id`、`GET /api/v1/jobs/:id/stream`、`GET /api/v1/models`。

### 3.2 节点侧凭据 (Agent Node Token)
- **生成途径**：管理员在管理控制台（`/api/v1/controller/nodes`）点击「新增节点」时由服务端生成高熵密钥（格式为 `agt_<32位随机16进制>`）。
- **存储机制**：数据库 `t_nodes` 仅存储该 Token 的 SHA256 哈希值 `token_hash` 与前缀 `token_prefix`（例如 `agt_a1b2c3d4...`），明文仅在创建瞬间向管理员展示一次。
- **验证机制**：Controller 编写专属中间件 `RequireAgentAuth()`，从 Query 参数 `?token=`（针对 WebSocket 握手）或 Header `Authorization: Bearer <agent_token>`（针对 HTTP 上报）提取 Token，哈希后与 `t_nodes` 比对校验，确认节点激活状态并绑定 `node_id` 到请求上下文。
- **作用范围**：
  - 仅限 Agent 节点调用：`GET /api/v1/agent/ws`、`GET /api/v1/agent/jobs/:id/media`、`POST /api/v1/agent/jobs/:id/logs`、`POST /api/v1/agent/jobs/:id/complete`。
  - 严格禁止 Agent Token 访问用户作业或系统配置，避免权限横向越权。

---

## 4. 完整的 API 接口与数据契约规范

所有接口去除冗余的 `/transcribe` 前缀；管理端接口统一使用 `/controller/` 命名空间。

### 4.1 OpenAI 兼容转录接口 (`POST /v1/audio/transcriptions`)
- **请求方法**：`POST`
- **鉴权**：User Access Token (`Authorization: Bearer <user_token>`)
- **Content-Type**：`multipart/form-data`
- **请求入参**：
  - `file` (File, 必需)：待转录音频或视频文件（支持 mp3, wav, m4a, webm, flac, mp4 等）。
  - `model` (String, 必需)：指定转录模型名称（如 `mock-whisper-base`）。
  - `language` (String, 可选)：音频语言代码（ISO-639-1，如 `zh`, `en`）。
  - `prompt` (String, 可选)：模型前导提示文本。
  - `response_format` (String, 可选)：返回格式，支持 `json`（默认）、`verbose_json`、`text`、`srt`、`vtt`。
  - `temperature` (Float, 可选)：采样温度（0.0 ~ 1.0）。
- **请求头扩展**：
  - `X-Async: true`：声明以异步方式创建 Job，不阻塞等待。
- **响应体**：
  - **当携带 `X-Async: true` 时 (CLI 与异步调用模式)**：
    ```json
    {
      "error_msg": "",
      "data": {
        "job_id": 10001,
        "status": "pending",
        "model": "mock-whisper-base",
        "created_at": "2026-09-03T16:40:00Z"
      }
    }
    ```
  - **未携带 `X-Async` 时 (标准同步 OpenAI SDK 调用模式)**：
    - Controller 同步阻塞等待 Agent 执行完毕（默认超时 120s），直接写出 OpenAI 标准格式：
    - `response_format=json`:
      ```json
      {
        "text": "今天天气非常晴朗，欢迎使用语音转录服务。"
      }
      ```
    - `response_format=verbose_json`:
      ```json
      {
        "task": "transcribe",
        "language": "chinese",
        "duration": 8.5,
        "text": "今天天气非常晴朗，欢迎使用语音转录服务。",
        "segments": [
          {
            "id": 0,
            "seek": 0,
            "start": 0.0,
            "end": 4.0,
            "text": "今天天气非常晴朗，",
            "tokens": [],
            "temperature": 0.0,
            "avg_logprob": -0.2,
            "compression_ratio": 1.1,
            "no_speech_prob": 0.01
          },
          {
            "id": 1,
            "seek": 400,
            "start": 4.0,
            "end": 8.5,
            "text": "欢迎使用语音转录服务。",
            "tokens": [],
            "temperature": 0.0,
            "avg_logprob": -0.15,
            "compression_ratio": 1.1,
            "no_speech_prob": 0.01
          }
        ]
      }
      ```

---

### 4.2 用户作业接口 (`/api/v1/jobs`)

#### 1) `GET /api/v1/jobs` (查询任务列表)
- **鉴权**：User Access Token
- **查询参数**：
  - `page` (int, 默认 1)
  - `page_size` (int, 默认 20)
  - `status` (string, 可选：`pending`, `running`, `completed`, `failed`)
- **响应示例**：
  ```json
  {
    "error_msg": "",
    "data": {
      "items": [
        {
          "id": 10001,
          "model": "mock-whisper-base",
          "status": "completed",
          "progress": 100,
          "duration": 8.5,
          "original_file_name": "meeting_recording.mp3",
          "created_at": "2026-09-03T16:40:00Z"
        }
      ],
      "total": 1,
      "page": 1,
      "page_size": 20
    }
  }
  ```

#### 2) `GET /api/v1/jobs/:id` (获取任务详情)
- **鉴权**：User Access Token
- **响应示例**：
  ```json
  {
    "error_msg": "",
    "data": {
      "id": 10001,
      "model": "mock-whisper-base",
      "status": "completed",
      "progress": 100,
      "duration": 8.5,
      "original_file_name": "meeting_recording.mp3",
      "result_text": "今天天气非常晴朗，欢迎使用语音转录服务。",
      "openai_response": { ... },
      "error_msg": "",
      "created_at": "2026-09-03T16:40:00Z",
      "completed_at": "2026-09-03T16:40:09Z"
    }
  }
  ```

#### 3) `GET /api/v1/jobs/:id/stream` (SSE 实时日志流)
- **鉴权**：User Access Token (`Authorization: Bearer <user_token>` 或 Query `?token=<user_token>`)
- **协议**：`text/event-stream`，长连接
- **数据帧格式**：
  - 增量日志输出：
    ```text
    event: log
    data: {"seq": 1, "progress": 20, "message": "[INFO] Audio decoded, extracting features..."}

    event: log
    data: {"seq": 2, "progress": 60, "message": "[INFO] Transcribing segment 2/3..."}
    ```
  - 任务完成事件：
    ```text
    event: finish
    data: {"status": "completed", "duration": 8.5, "result_text": "今天天气非常晴朗，欢迎使用语音转录服务。"}
    ```
  - 任务失败事件：
    ```text
    event: finish
    data: {"status": "failed", "error_msg": "Audio stream corrupted."}
    ```
- **机制**：客户端连接建立后，Controller 自动重放该 Job 历史日志，随后实时输出后续增量；收到 `finish` 事件后服务端主动关闭该 SSE 连接。

#### 4) `GET /api/v1/models` (可用模型列表)
- **鉴权**：User Access Token
- **响应示例**：
  ```json
  {
    "error_msg": "",
    "data": [
      {
        "name": "mock-whisper-base",
        "task_type": "asr",
        "description": "Mock ASR base model for testing and demonstration",
        "is_active": true
      }
    ]
  }
  ```

---

### 4.3 Agent 节点接口 (`/api/v1/agent/*`)

#### 1) `GET /api/v1/agent/ws?token=<agent_token>` (WebSocket 控制信令通道)
- **握手鉴权**：Query 参数 `token`，Controller 校验有效后将连接与该节点实例绑定入 `AgentHub`。
- **信令信封格式**：
  ```json
  {
    "type": "heartbeat | command | event",
    "action": "dispatch_job | load_model | unload_model | model_status | heartbeat_ack",
    "seq": 1001,
    "payload": {}
  }
  ```
- **Agent 上报心跳 (定时 5s)**：
  ```json
  {
    "type": "heartbeat",
    "seq": 2001,
    "payload": {
      "version": "0.1.0",
      "loaded_models": ["mock-whisper-base"],
      "running_jobs": 1,
      "system": {
        "cpu_percent": 14.2,
        "ram_used_mb": 2048,
        "ram_total_mb": 16384,
        "gpu_percent": 18.0,
        "gpu_memory_used_mb": 1024,
        "gpu_memory_total_mb": 8192
      }
    }
  }
  ```
- **Controller 下发任务派发指令 (`dispatch_job`)**：
  ```json
  {
    "type": "command",
    "action": "dispatch_job",
    "seq": 3001,
    "payload": {
      "job_id": 10001,
      "model_name": "mock-whisper-base",
      "task_type": "asr",
      "language": "zh",
      "media_path": "/api/v1/agent/jobs/10001/media"
    }
  }
  ```
- **Controller 下发模型加载/卸载指令 (`load_model` / `unload_model`)**：
  ```json
  {
    "type": "command",
    "action": "load_model",
    "seq": 3002,
    "payload": {
      "model_name": "mock-whisper-base"
    }
  }
  ```

#### 2) `GET /api/v1/agent/jobs/:id/media` (下载待转录音频)
- **鉴权**：Agent Node Token (`Authorization: Bearer <agent_token>`)
- **响应**：`Content-Type: audio/mpeg`（或实际类型），支持流式分块传输。

#### 3) `POST /api/v1/agent/jobs/:id/logs` (批量上报日志与进度)
- **鉴权**：Agent Node Token
- **请求体**：
  ```json
  {
    "progress": 50,
    "logs": [
      {
        "timestamp": "2026-09-03T16:40:02Z",
        "level": "info",
        "message": "Processing audio chunk 2/4..."
      }
    ]
  }
  ```
- **Controller 行为**：入库 `t_job_logs`，同时通过 Go Channel 广播至监听该 Job 的 SSE 协程。

#### 4) `POST /api/v1/agent/jobs/:id/complete` (任务完成结算)
- **鉴权**：Agent Node Token
- **请求体**：
  ```json
  {
    "status": "completed",
    "duration_seconds": 8.5,
    "result_text": "今天天气非常晴朗，欢迎使用语音转录服务。",
    "openai_response": {
      "task": "transcribe",
      "language": "chinese",
      "duration": 8.5,
      "text": "今天天气非常晴朗，欢迎使用语音转录服务。",
      "segments": [ ... ]
    },
    "error_msg": ""
  }
  ```
- **Controller 行为**：更新 `t_jobs` 状态为 `completed`（或 `failed`），记录文本与耗时，向 SSE 广播 `finish` 事件。

---

### 4.4 Controller 管理端端点 (`/api/v1/controller/*`)

统一采用 `/controller/` 前缀：

#### 1) `GET /api/v1/controller/nodes` (节点列表)
- **鉴权**：Admin 权限
- **响应示例**：
  ```json
  {
    "error_msg": "",
    "data": [
      {
        "id": 1,
        "name": "gpu-worker-node-1",
        "token_prefix": "agt_a1b2c3d4",
        "is_active": true,
        "is_online": true,
        "loaded_models": ["mock-whisper-base"],
        "running_jobs": 1,
        "system": {
          "cpu_percent": 14.2,
          "ram_percent": 35.0,
          "gpu_percent": 22.0
        },
        "last_seen_at": "2026-09-03T16:40:05Z"
      }
    ]
  }
  ```

#### 2) `POST /api/v1/controller/nodes` (新增节点并颁发凭据)
- **鉴权**：Admin 权限
- **请求体**：`{ "name": "gpu-worker-node-2" }`
- **响应体**：
  ```json
  {
    "error_msg": "",
    "data": {
      "id": 2,
      "name": "gpu-worker-node-2",
      "agent_token": "agt_9f8e7d6c5b4a31201928374655443322",
      "token_prefix": "agt_9f8e7d6c",
      "created_at": "2026-09-03T16:40:10Z"
    }
  }
  ```
  *(注：`agent_token` 仅在此时展示一次)*

#### 3) `POST /api/v1/controller/nodes/:id/load-model` (手动触发加载模型)
- **鉴权**：Admin 权限
- **请求体**：`{ "model_name": "mock-whisper-base" }`
- **行为**：Controller 通过 WebSocket 向对应在线节点下发 `load_model` 指令。

#### 4) `POST /api/v1/controller/nodes/:id/unload-model` (手动触发卸载模型)
- **鉴权**：Admin 权限
- **请求体**：`{ "model_name": "mock-whisper-base" }`
- **行为**：Controller 通过 WebSocket 向对应在线节点下发 `unload_model` 指令。

---

## 5. 数据库表定义与双方言 Goose 迁移 (表前缀 `t_`)

针对 PostgreSQL 与 SQLite3 双方言编写独立的 SQL Goose 迁移脚本。

### 5.1 实体表 DDL 规约

```sql
-- 1. t_models: 注册的模型列表
CREATE TABLE IF NOT EXISTS t_models (
    id          BIGINT PRIMARY KEY,
    name        VARCHAR(64) UNIQUE NOT NULL,
    task_type   VARCHAR(32) NOT NULL DEFAULT 'asr',
    description TEXT,
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 2. t_nodes: 推理节点注册表
CREATE TABLE IF NOT EXISTS t_nodes (
    id           BIGINT PRIMARY KEY,
    name         VARCHAR(64) NOT NULL,
    token_hash   VARCHAR(64) UNIQUE NOT NULL,
    token_prefix VARCHAR(16) NOT NULL,
    is_active    BOOLEAN NOT NULL DEFAULT TRUE,
    last_ip      VARCHAR(45),
    last_seen_at TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 3. t_jobs: 转录任务主表
CREATE TABLE IF NOT EXISTS t_jobs (
    id                  BIGINT PRIMARY KEY,
    user_id             BIGINT NOT NULL,
    node_id             BIGINT,
    model_name          VARCHAR(64) NOT NULL,
    task_type           VARCHAR(32) NOT NULL DEFAULT 'asr',
    status              VARCHAR(32) NOT NULL DEFAULT 'pending',
    progress            INT NOT NULL DEFAULT 0,
    audio_storage_path  TEXT NOT NULL,
    original_file_name  VARCHAR(255) NOT NULL,
    duration_seconds    FLOAT NOT NULL DEFAULT 0,
    result_text         TEXT,
    result_json         TEXT,
    error_msg           TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    started_at          TIMESTAMPTZ,
    completed_at        TIMESTAMPTZ,
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_t_jobs_user_status ON t_jobs(user_id, status);

-- 4. t_job_logs: 任务运行日志流水表
CREATE TABLE IF NOT EXISTS t_job_logs (
    id         BIGINT PRIMARY KEY,
    job_id     BIGINT NOT NULL,
    seq        INT NOT NULL,
    progress   INT NOT NULL DEFAULT 0,
    message    TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_t_job_logs_job_id_seq ON t_job_logs(job_id, seq);
```

*(SQLite3 方言中，`TIMESTAMPTZ` 自动替换为 `DATETIME`)*

---

## 6. Controller 插件内部架构与调度核心实现

目录位置：`backend/transcribe/plugins/svr`

### 6.1 代码分层结构
```text
controller/
├── plugin.go                   # 插件入口，实现 core.Plugin (Name & Apply)
├── consts/
│   ├── consts.go               # 状态枚举 (StatusPending, StatusRunning 等)
│   └── errs.go                 # 驼峰式错误常量 (errInvalidToken, errNodeOffline 等)
├── controller/
│   ├── admin_node_handler.go   # /api/v1/controller/nodes 相关 Handler
│   ├── agent_handler.go        # /api/v1/agent/ws 及 HTTP 日志/完成 Handler
│   ├── job_handler.go          # /api/v1/jobs 列表与 SSE /stream Handler
│   └── openai_handler.go       # /v1/audio/transcriptions 兼容 Handler
├── service/
│   ├── hub/
│   │   ├── agent_hub.go        # 内存长连管理池，心跳与保活看门狗
│   │   └── session.go          # 单个在线 Agent 实例上下文与 WS 读写锁
│   ├── scheduler/
│   │   └── scheduler.go        # 核心任务调度器 (选点、自动加载模型、派发)
│   ├── job_service.go          # Job 创建、媒体保存、结果落库
│   ├── log_broker.go           # SSE 发布订阅通道管理器
│   └── node_service.go         # 节点增删、Token 校验与状态更新
├── dao/
│   ├── job_dao.go              # t_jobs 与 t_job_logs CRUD (带 EscapeLike)
│   ├── node_dao.go             # t_nodes 操作
│   └── model_dao.go            # t_models 操作
├── model/
│   ├── entity/                 # GORM 映射实体
│   │   ├── model_entity.go     # TableName() = "t_models"
│   │   ├── node_entity.go      # TableName() = "t_nodes"
│   │   ├── job_entity.go       # TableName() = "t_jobs"
│   │   └── job_log_entity.go   # TableName() = "t_job_logs"
│   └── do/                     # DTO 与 OpenAI 响应模型
└── migrations/
    ├── postgres/               # 00001_init_transcribe.sql
    └── sqlite/                 # 00001_init_transcribe.sql
```

### 6.2 AgentHub 与长连会话管理
- **连接池结构**：使用 `sync.RWMutex` 保护 `map[uint64]*AgentSession`。
- **Session 上下文**：记录 `NodeID`, `WSConn`, `LoadedModels []string`, `RunningJobs int`, `LastHeartbeat time.Time`, `SystemStats`.
- **心跳与掉线看门狗**：后台独立 goroutine（`pkg/util.Go`），每 5 秒巡检一次：
  - 若 `time.Since(LastHeartbeat) > 15s`，判定掉线，关闭旧连接，从连接池移除；
  - 将该节点正在执行的未完成 Job 状态由 `running` 重置回 `pending`，触发重新调度。

### 6.3 调度器算法 (Scheduler Algorithm)
1. 触发时机：
   - 用户成功提交新 Job 入库后；
   - 在线 Agent 发送心跳汇报当前完成任务空闲时；
   - 新 Agent 上线注册就绪时。
2. 调度执行：
   - 提取所有状态为 `pending` 的 Job；
   - 检查在线节点池：
     - **第一优选**：节点已加载该 `model_name`，且 `RunningJobs` 最小；
     - **第二优选**：无节点加载，但存在空闲节点（CPU/RAM 负载最低），向其发送 `load_model` 指令，等待回执后派发；
     - **排队等待**：无任何可用在线节点时，Job 保持 `pending`，等待节点接入。
   - 派发执行：更新 Job 状态为 `running`，绑定 `node_id`，通过 WebSocket 下发 `dispatch_job` 信令。

### 6.4 SSE 实时日志广播实现
- 每个处于 `running` 状态的 Job 在内存中分配一个订阅者列表 `[]chan LogMessage`。
- Agent 通过 `POST /api/v1/agent/jobs/:id/logs` 上报时：
  1. 批量插入数据库 `t_job_logs`；
  2. 遍历订阅者 Channel，采用非阻塞发送 `select { case ch <- msg: default: }` 避免拖垮上报流程。
- CLI 连接 `GET /api/v1/jobs/:id/stream`：
  1. 先从 `t_job_logs` 取出已存在的历史日志全部写出；
  2. 注册 Channel 接收后续增量，持续以 SSE 格式写出直至任务 `finish`；
  3. 客户端断开连接时，注销并关闭对应的 Channel。

---

## 7. Python Agent 内部架构与 Mock 引擎

目录位置：`backend/agent`

### 7.1 文件布局
```text
backend/agent/
├── pyproject.toml             # uv 项目依赖配置
├── README.md                  # 运行说明
├── config.yaml                # 本地配置 (CONTROLLER_URL, AGENT_TOKEN, NODE_NAME)
└── src/
    ├── __init__.py
    ├── config.py              # 读取 yaml 或环境变量
    ├── monitor.py             # psutil 获取 CPU/RAM，try import pynvml 获取显存
    ├── ws_client.py           # 核心长连循环、自动重连、心跳发送与指令分发
    ├── job_runner.py          # 任务调度器 (asyncio.Task 任务池与全局 try-except 防崩溃)
    ├── reporter.py            # httpx.AsyncClient 负责音频下载与日志/结果上报
    ├── models/
    │   ├── __init__.py
    │   ├── base.py            # 抽象基类 BaseEngine (load, unload, transcribe)
    │   ├── registry.py        # 模型实例字典管理
    │   └── mock_asr.py        # v1 Mock ASR 引擎实现
    └── main.py                # 程序入口
```

### 7.2 核心并发与防崩溃设计
1. **主事件循环与重连保护**：
   `ws_client.py` 运行在 `asyncio` 主循环中，连接断开后以指数退避重试（1s -> 2s -> 5s，最大 30s），保障即使 Controller 重启，Agent 也会自动重连归队。
2. **多 Job 并发隔离**：
   收到 `dispatch_job` 时，调用 `asyncio.create_task(job_runner.handle_job(payload))`，绝不在 WS 接收循环中同步执行任务。
3. **全局异常盾牌 (Exception Shielding)**：
   每个 Job 完整包裹在：
   ```python
   try:
       # 1. 报告开始，下载音频
       # 2. 调取 mock ASR 推理
       # 3. 提交 complete 结算
   except Exception as e:
       logger.exception(f"Job {job_id} failed with error: {e}")
       await reporter.report_complete(job_id, status="failed", error_msg=str(e))
   finally:
       # 清理本地临时下载的音频文件
   ```
   绝不允许单个任务的失败导致整个 Agent 进程退出或断线。
4. **Mock ASR 引擎行为**：
   - 模拟真实的语音识别阶段耗时：
     - Step 1 (0.5s, 进度 20%): 加载音频与特征提取；
     - Step 2 (1.0s, 进度 50%): 分块声学模型推理；
     - Step 3 (1.0s, 进度 80%): 语言模型解码与时间戳对齐；
     - Step 4 (0.5s, 进度 100%): 生成最终转录文本。
   - 返回符合 OpenAI `verbose_json` 格式的标准字幕结构。

---

## 8. CLI 独立工具设计与交互体验

编译产物：`bin/transcribe`，构建入口：`backend/cmd/transcribe/main.go`。

### 8.1 纯净编译构建
- 在项目根目录 `Makefile` 中提供：
  ```makefile
  build-cli:
  	cd backend && go build -ldflags "-s -w" -o ../bin/transcribe cmd/transcribe/main.go
  ```
- 二进制仅引用 Cobra、标准 HTTP 客户端及轻量工具，零服务端冗余依赖，体积小、秒级启动。

### 8.2 本地配置管理
- 配置文件路径：`~/.transcribe/config.yaml`
- 内容：
  ```yaml
  controller_url: "http://127.0.0.1:8000"
  access_token: "wavelet_pat_xxxx"
  default_model: "mock-whisper-base"
  ```

### 8.3 命令详细实现

#### 1) `transcribe login`
- 语法：`transcribe login [--url <url>] [--token <token>]`
- 交互：未传参时终端提示输入 Controller 地址与 Access Token；
- 探测：向 Controller 发起 `GET /api/v1/models` 请求，校验凭证有效性；成功后写入 `~/.transcribe/config.yaml` 并打印登录成功提示。

#### 2) `transcribe asr [--model <name>] <filepath>`
1. **格式检查与 ffmpeg 预处理**：
   - 校验本地输入文件是否存在；
   - 检测扩展名：
     - 若为纯音频格式（`.mp3`, `.wav`, `.flac`, `.m4a`, `.aac`, `.ogg`），直接使用；
     - 若为视频格式（`.mp4`, `.mkv`, `.avi`, `.mov`, `.webm`, `.flv` 等）：
       - 调用 `exec.LookPath("ffmpeg")` 探测本地是否存在 `ffmpeg`；
       - 若不存在，输出明确指引并退出；
       - 若存在，执行：
         `ffmpeg -i <input> -vn -ac 1 -ar 16000 -c:a libmp3lame -b:a 48k <tmp.mp3>`
         将视频转为单声道 16kHz 优化的临时音频文件，极大降低网络带宽与上传耗时。
2. **任务上传与提交**：
   - 构造 multipart 表单，设置 Header `X-Async: true`，发起 `POST /v1/audio/transcriptions`；
   - 获得 Controller 返回的 `job_id`，控制台打印：
     `✓ 任务已成功提交！Job ID: 10001`
     `正在连接实时日志流... (可随时按 Ctrl+C 退出，后台任务不会中断)`
3. **实时流追踪与安全退出**：
   - 建立 SSE 长连 `GET /api/v1/jobs/10001/stream`；
   - 监听系统中断信号（`os.Interrupt` / `SIGTERM`）：
     若用户在转录过程中按下 `Ctrl+C`，捕获信号打印友好提示：
     `已退出日志追踪。任务仍在后台运行，可运行 transcribe jobs log 10001 查看进度。`
     进程优雅退出，后台任务正常推进。
   - 收到 `finish` 事件后，打印总耗时并高亮输出转录出的文本内容。

#### 3) `transcribe jobs ls`
- 语法：`transcribe jobs ls`
- 调用 `GET /api/v1/jobs`，使用终端格式化制表输出：
  ```text
  JOB ID   MODEL               STATUS      PROGRESS  DURATION  CREATED AT
  10001    mock-whisper-base   completed   100%      8.5s      2026-09-03 16:40:00
  10002    mock-whisper-base   running     60%       -         2026-09-03 16:42:15
  ```

#### 4) `transcribe jobs log <job_id> [-f / --follow]`
- 语法：`transcribe jobs log <job_id> [-f]`
- 默认直接读取任务详情打印已有日志；若携带 `-f` 则接入 SSE 流实时追随输出直至任务结束。

---

## 9. 质量门禁与测试计划

1. **静态代码与架构红线检查**：
   - 运行 `make code-check`：验证没有跨插件非法直接依赖，保证 `backend/pkg` 纯净度。
   - 运行 `make format`：保证后端 Go 代码风格对齐。
2. **单元测试与集成测试**：
   - Controller 测试：覆盖 Token 鉴权中间件、Job 状态机流转、SSE 广播逻辑。
   - CLI 测试：覆盖本地参数解析、配置持久化、音视频文件扩展名探测与 ffmpeg 命令构造。
   - Python Agent 测试：覆盖 `monitor.py` 系统指标采集、Mock ASR 状态流转与全局异常捕获。
3. **全链路联调验收**：
   - 启动 Controller 服务；
   - Admin 在管理端调用接口新增节点并获取 `agent_token`；
   - 配置并启动 Python Agent，确认 WS 建立长连并上报心跳；
   - 运行 `transcribe login` 完成 CLI 认证绑定；
   - 运行 `transcribe asr sample.mp4`，验证 ffmpeg 本地压缩转音频、任务派发至 Agent、SSE 实时流式吐出日志，并在终端最终完整展示识别文本。
