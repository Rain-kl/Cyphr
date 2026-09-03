# Cyphr

🚀 现代化、生产级分布式智能语音识别与音频转录平台

[English](./README.md) · [简体中文](./README_zh.md)

[![License: Apache-2.0](https://img.shields.io/badge/License-Apache--2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8.svg?logo=go)](https://golang.org/)
[![Next.js](https://img.shields.io/badge/Next.js-16-black.svg?logo=next.js)](https://nextjs.org/)
[![React](https://img.shields.io/badge/React-19-61DAFB.svg?logo=react)](https://reactjs.org/)
[![Python](https://img.shields.io/badge/Python-3.12+-3776AB.svg?logo=python)](https://python.org/)
[![Tailwind CSS](https://img.shields.io/badge/Tailwind-4.0-38B2AC.svg?logo=tailwind-css)](https://tailwindcss.com/)

---

## 📖 平台简介

**Cyphr**（原 Transcribe）是一套专为高性能、大规模生产环境打造的**分布式智能语音识别 (ASR) 与音视频转录 SaaS 平台**。

项目采用清晰解耦的三层架构设计：服务端控制中心（Go + Gin + GORM + Cordis 插件体系）、推理计算节点（Python + uv + 智能硬件感知）、现代化管理与操作控制台（Next.js 16 + React 19 + Shadcn UI），并配套独立的轻量级跨平台命令行工具（Cyphr CLI）。

平台天然兼容 **OpenAI 音频转录 API 标准**，能够作为企业自建私有化转录集群的核心引擎，实现对 Whisper、FunASR、Qwen-ASR 等主流开源或专有语音模型的统一纳管、弹性调度与高并发作业处理。

---

## ✨ 核心特性

- 🎙️ **完全兼容 OpenAI 转录标准**  
  原生提供 `/api/v1/audio/transcriptions` 接口，入参与出参严格对齐 OpenAI 规范（支持 `json`、`verbose_json`、`text` 格式），现有基于 OpenAI SDK 的应用无缝平滑迁移。

- ⚡ **分布式推理调度与拓扑弹性伸缩**  
  控制面与算力面彻底解耦。支持任意数量的 Python 推理节点（Agent）主动通过 WebSocket 建立长连集群，动态感知 CPU、内存、GPU（NVIDIA VRAM）实时利用率，配合内置调度器实现智能路由与最优负载均衡。

- 📡 **双向实时通道与流式响应**  
  - **控制通道**：基于 WebSocket 的双向心跳、作业派发、模型热挂载与卸载信令机制；
  - **日志通道**：基于 Server-Sent Events (SSE) 的实时转录进度及分块文本流式广播，告别轮询开销。

- 🖥️ **现代化全功能管理控制台 (Web Console)**  
  基于 Next.js 16 App Router、Tailwind CSS 4 与 Shadcn UI 打造。提供：
  - **ASR 作业大厅**：平台全局转录任务实时多维过滤检索与状态机视图；
  - **算力节点驾驶舱**：硬件实时遥测抽屉（CPU、RAM、GPU 占用及显存监控）；
  - **模型热管控**：模型一键启停、节点模型定向挂载与动态卸载；
  - **深度作业分析器 (Job Deep Inspector)**：单作业原声音频在线播放与下载、转录分词段落、原始结果 JSON 深度透视。

- 💻 **全能且独立的命令行客户端 (Cyphr CLI)**  
  独立的 Go 二进制程序（`bin/cyphr`，兼容别名 `transcribe`）：
  - 本地智能音视频探测与自动 `ffmpeg` 无损抽流压缩；
  - 异步任务一键提交并附带终端优雅加载条；
  - 实时流式日志查看（支持 Ctrl+C 中断不影响后台任务执行）；
  - 历史作业查询与转录文件本地导出。

- 🔒 **严格的双轨正交安全隔离体系**  
  - **用户凭据 (User Access Token)**：管理常规用户业务接口与 CLI 操作；
  - **节点凭据 (Agent Node Token)**：控制面生成的强熵一次性密钥，服务端仅存 SHA-256 哈希，物理级杜绝横向越权。

- 📦 **企业级通用文件对象存储 (Platform Ingest)**  
  音频文件统一受控托管于平台级存储引擎（支持本地存储与各类 S3 兼容对象存储），杜绝孤岛临时文件落地，提供完整的存储生命周期管理与去重治理。

---

## 🏗️ 整体架构与流程

### 2.1 拓扑架构

```text
┌──────────────────────────────────────────────────────────────────┐
│                        Cyphr 控制面集群                           │
│                                                                  │
│   ┌─────────────────────┐   ┌────────────────────────────────┐   │
│   │  OpenAI API / Jobs  │   │     Node & Model Scheduler     │   │
│   └──────────┬──────────┘   └───────────────┬────────────────┘   │
│              │ (User Token)                 │ (Agent Token)      │
└──────────────┼──────────────────────────────┼────────────────────┘
               │                              │ (WebSocket + HTTP)
       ┌───────┴────────┐                     ▼
       │  用户接入层     │          ┌──────────────────────┐
       │                │          │  Python Worker Nodes │
       │ • Web Console  │          │                      │
       │ • Cyphr CLI    │          │ • Node 01 (GPU A10)  │
       │ • API Client   │          │ • Node 02 (GPU 4090) │
       │ • OpenAI SDK   │          │ • Node 03 (CPU Only) │
       └────────────────┘          └──────────────────────┘
```

### 2.2 核心作业时序图 (End-to-End Sequence)

```text
客户端 (Web / CLI)                 控制端 (Controller)                 推理节点 (Agent)
     │                                     │                                  │
     │                                     │<========= [WS Connect] ==========│ (节点凭据鉴权长连)
     │                                     │<========= [WS Heartbeat] ========│ (实时上报 CPU/GPU/模型)
     │                                     │                                  │
     │── 1. 提交音频 (POST /api/v1/...) ───>│                                  │
     │                                     │── 2. 受控入库 (Platform Storage) ─│
     │                                     │── 3. 创建 Job (status=pending) ───│
     │<── 4. 返回 Job ID ──────────────────│                                  │
     │                                     │── 5. 调度器匹配就绪节点 ─────────│
     │── 6. 订阅日志 (GET /jobs/:id/stream) >│                                  │
     │                                     │════════ [WS: dispatch_job] ═════>│ (下发作业参数与媒体地址)
     │                                     │                                  │── 7. 流式下载音频媒体
     │                                     │<── 8. 分批上报日志 (POST /logs) ─│── 8. 启动模型多阶段推理
     │<══ 9. SSE 实时推送日志与进度 ═══════│                                  │
     │                                     │<── 10. 任务结算 (POST /complete) ─│── 10. 生成 OpenAI 结果
     │                                     │── 11. 状态流转为 completed ──────│
     │<══ 12. SSE 收到完成信号与最终文本 ══│                                  │
```

---

## 🛠️ 技术栈清单

| 层次 | 核心技术 | 说明 |
| :--- | :--- | :--- |
| **后端核心** | Go 1.25+, Gin, GORM | 高并发服务端、RESTful API、数据持久化 |
| **微内核架构** | Cordis 插件体系, Go Submodules | 模块化解耦，面向 `contracts` 接口编程 |
| **前端控制台** | Next.js 16 (Turbopack), React 19, TypeScript | 现代化响应式 Web 控制台与管理系统 |
| **样式与组件** | Tailwind CSS 4, Shadcn UI, Motion | 精致现代的主题设计与动态流式交互 |
| **国际化** | next-intl | 完整的中文 / 英文双语即时切换 |
| **推理 Agent** | Python 3.12+, uv, psutil, pynvml | 异步高性能长连 Worker、系统硬件动态感知 |
| **多媒体编解码** | FFmpeg / ffprobe | 音视频文件探测、无损重采样与格式压缩 |
| **CLI 命令行** | Cobra, Viper | 独立轻量级跨平台二进制客户端 |
| **数据与存储** | PostgreSQL, SQLite, S3 Compatible Store | 业务数据存储与分布式对象存储适配 |

---

## 🚀 快速上手

### 1. 环境准备

- **Go** >= 1.25
- **Node.js** >= 18.0 & **pnpm** >= 9.0
- **Python** >= 3.12 & [uv](https://docs.astral.sh/uv/) 包管理器
- **FFmpeg**（可选，CLI 本地音频压缩转码必需）

### 2. 启动控制中心服务端与前端

```bash
# 1. 克隆代码仓库
git clone https://github.com/Rain-kl/Cyphr.git
cd Cyphr

# 2. 配置环境变量
cp .env.example .env

# 3. 安装前端依赖
cd frontend && pnpm install && cd ..

# 4. 同时启动后端与前端开发服务
make dev
```

服务启动后，打开浏览器访问：
- **Web 控制台**：`http://localhost:3000`
- **默认管理员账号**：`admin` / `admin123`
- **后台 API 接口**：`http://localhost:8080`
- **Swagger API 文档**：`http://localhost:8080/swagger/index.html`

### 3. 部署并启动推理节点 (Agent)

在 GPU 计算服务器或本地机器运行推理 Agent：

```bash
# 进入 Agent 目录
cd backend/agent

# 使用 uv 快速同步环境与依赖
uv sync

# 配置 Agent 连接地址与节点 Token (在 Web 控制台「ASR 管理 -> 节点管理」中点击「创建节点」获取)
export AGENT_SERVER_URL="http://localhost:8080"
export AGENT_TOKEN="agt_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
export AGENT_NODE_NAME="worker-node-01"

# 启动推理 Agent
uv run python -m src.main
```
Agent 启动后会自动向控制中心发起 WebSocket 连接并上报硬件遥测信息。

### 4. 使用 Cyphr 命令行客户端 (CLI)

构建独立的 CLI 二进制文件：

```bash
# 编译命令行工具到 bin/cyphr
make build-cli

# 登录控制中心并保存访问凭据 (在个人设置页获取 User Access Token)
./bin/cyphr login --url http://localhost:8080 --token usr_your_personal_token

# 查看集群可用模型
./bin/cyphr models

# 一键转录音视频文件（自动检测、压制并实时打印 SSE 日志）
./bin/cyphr asr /path/to/meeting.mp4 --model mock-whisper-base

# 查看历史转录作业
./bin/cyphr jobs ls
```

---

## 🔌 OpenAI 兼容接口调用示例

Cyphr 服务端支持标准的 OpenAI 音频转录调用方式：

### 示例 1：使用 cURL

```bash
curl -X POST http://localhost:8080/api/v1/audio/transcriptions \
  -H "Authorization: Bearer <YOUR_USER_ACCESS_TOKEN>" \
  -F "file=@/path/to/audio.mp3" \
  -F "model=mock-whisper-base" \
  -F "response_format=verbose_json"
```

### 示例 2：使用官方 OpenAI Python SDK

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://localhost:8080/api/v1",
    api_key="<YOUR_USER_ACCESS_TOKEN>"
)

with open("meeting.wav", "rb") as audio_file:
    transcript = client.audio.transcriptions.create(
        model="mock-whisper-base",
        file=audio_file,
        response_format="verbose_json"
    )

print(transcript.text)
```

---

## 📋 常用工程指令 (Makefile)

项目根目录提供了全面的 Makefile 指令：

```bash
make dev             # 并行启动前端与后端开发服务
make dev-b           # 单独启动后端服务 (go run main.go all)
make dev-f           # 单独启动前端服务 (pnpm dev)
make build-cli       # 构建独立的命令行客户端 bin/cyphr
make build-backend   # 编译纯后端二进制 bin/cyphr
make build-embedded  # 将前端静态产物整体内嵌打包入单一可执行文件 bin/cyphr
make code-check      # 执行代码质量与架构合规性门禁 (Cordis 隔离性 + golangci-lint + tsc + eslint)
make format          # 格式化所有后端 Go 代码与前端代码
make swagger         # 根据后端注释重新生成 Swagger API 文档
```

---

## 🗂️ 目录结构说明

```text
Cyphr/
├── backend/                  # 后端工程根目录
│   ├── cmd/                  # 命令行服务入口与子命令
│   ├── core/                 # 微内核契约与插件运行时上下文
│   ├── plugins/              # 核心业务域与基础设施插件
│   ├── transcribe/           # Transcribe 专有业务域插件
│   │   ├── plugins/svr/      # 控制中心插件 (Controller, Hub, Scheduler, DAO)
│   │   ├── plugins/cli/      # 命令行工具实现代码 (Cobra commands, client)
│   │   └── tests/            # 平台端到端 (E2E) 测试套件
│   └── agent/                # 分布式推理节点实现 (Python 3.12+ / uv)
├── frontend/                 # 前端工程根目录
│   ├── app/                  # Next.js 16 App Router (用户工作台与管理员控制台)
│   ├── components/           # UI 组件库 (Shadcn UI & 业务自定义组件)
│   ├── lib/services/         # 前端 API 统一封装
│   └── messages/             # 多语言国际化字典 (zh / en)
├── docs/                     # 架构规格说明书与设计白皮书
├── manifest/                 # 容器构建与部署配置文件 (Docker, Kubernetes)
└── scripts/                  # 架构合规性检查与自动化工具脚本
```

---

## 📄 开源许可证

本项目基于 [Apache 2.0 License](./LICENSE) 协议开源。
