# ASR 语音转录前端交互与全接口对齐设计说明书

**日期**: 2026-09-03  
**状态**: Approved  
**版本**: 1.0.0  

---

## 1. 设计目标与产品定位

为 Transcribe 语音转录 SaaS 平台构建一套专业、直观、高可用且对所有后端接口 100% 对齐的前端应用。
严格区分普通用户与系统管理员角色，遵循 Wavelet 前端规范、shadcn/ui 设计语言与 `next-intl` 国际化体系。

### 1.1 核心原则与防线
1. **接口 100% 深度对齐**：服务端暴露的所有 Controller 管理接口与数据接口，必须在前端有完整、可操作的交互实体，禁止悬空无用接口；
2. **零硬编码与完全国际化**：所有 UI 提示、字段、按钮与空状态文案统一存放于 `frontend/messages/zh-CN.json` 与 `en.json`；
3. **物理子组件拆分**：严格遵循单文件代码小于 600 行规范，多 Tab 与复杂对话框按就近原则拆分为独立组件存放在 `components/` 局部子目录中；
4. **WCAG AA 与色彩体系**：使用项目主题 CSS 变量与 shadcn/ui 组件，支持亮暗色模式，所有无文字图标按钮显式添加 `aria-label`。

---

## 2. 路由结构与菜单布局

| 路由路径 | 页面定位 | 权限要求 | 侧边栏入口 |
| :--- | :--- | :--- | :--- |
| `/asr` | **用户端 ASR 转录大盘** | 全体登录用户 | 主导航组 (`navMainItems`)，菜单名 `ASR`，图标 `AudioWaveform` |
| `/asr/jobs/[id]` | **用户端作业详情与音字联动工作台** | 任务所有者或管理员 | 通过 `/asr` 表格行点击或提交任务后自动重定向直达 |
| `/admin/asr` | **管理员 ASR 集群与全景管控中心** | 仅系统管理员 (`is_admin`) | 管理组 (`adminItems`)，菜单名 `ASR 管理`，图标 `Cpu` |

---

## 3. 后端接口完整映射表 (100% 对齐矩阵)

| 后端 API 路由 | 请求方法 | 功能说明 | 对应前端界面操作点 |
| :--- | :--- | :--- | :--- |
| `POST /api/v1/audio/transcriptions` | POST (Multipart) | 创建转录任务 (带 `X-Async: true`) | `/asr` 的「新建转录」弹窗提交 |
| `GET /api/v1/jobs` | GET | 分页查询当前用户任务列表 | `/asr` 的任务大盘数据表格 |
| `GET /api/v1/jobs/:id` | GET | 查询单任务详情与转录文本 | `/asr/jobs/[id]` 详情与管理员详情抽屉 |
| `GET /api/v1/jobs/:id/stream` | GET (SSE) | 实时消费任务日志与进度事件流 | `/asr/jobs/[id]` 实时控制台终端与运行中状态 |
| `GET /api/v1/models` | GET | 获取所有启用的模型清单 | 「新建转录」下拉框、模型管理总览 |
| `GET /api/v1/controller/nodes` | GET | 获取全部 Agent 节点与实时心跳指标 | `/admin/asr` 的「节点管理」卡片与表格 |
| `POST /api/v1/controller/nodes` | POST | 注册新节点并生成一次性 Token | `/admin/asr` 的「新增节点」弹窗向导 |
| `GET /api/v1/controller/nodes/:id` | GET | 获取单节点的深度硬件与任务运行详情 | `/admin/asr` 点击「节点详情」侧边抽屉 |
| `DELETE /api/v1/controller/nodes/:id` | DELETE | 注销/移除特定 Agent 节点 | `/admin/asr` 节点操作菜单中的「删除节点」确认框 |
| `POST /api/v1/controller/nodes/:id/load-model` | POST | 向指定在线节点下发加载模型指令 | `/admin/asr` 节点卡片上的「加载模型」操作弹窗 |
| `POST /api/v1/controller/nodes/:id/unload-model` | POST | 向指定在线节点下发卸载模型指令 | `/admin/asr` 已加载模型标签上的快捷「卸载」操作 |
| `GET /api/v1/controller/models` | GET | 管理员获取全量模型列表（含停用） | `/admin/asr` 的「模型管理」标签页数据表 |
| `PUT /api/v1/controller/models/:id/status` | PUT | 切换模型的全局启用/停用状态 | `/admin/asr` 模型表格中的 Switch 开关组件 |
| `GET /api/v1/controller/jobs` | GET | 管理员穿透全平台所有用户作业大盘 | `/admin/asr` 的「ASR 任务全景」标签页 |
| `GET /api/v1/agent/jobs/:id/media` | GET | 音频媒体流下载与回放播放 | `/asr/jobs/[id]` 的 HTML5 同步音频播放器 |

---

## 4. 前端交互与视图组件细化设计

### 4.1 用户端：ASR 任务列表大盘 (`/asr`)
- **指标卡片栏 (Metric Stats)**：
  - 4 个卡片：总任务数、正在转录、已完成、失败异常。
- **操作工具栏 (Toolbar)**：
  - 关键词搜索框（按文件名搜索）；
  - 状态筛选器下拉框（全部、等待中、转录中、已完成、已失败）；
  - 刷新按钮与「新建任务」Primary 按钮。
- **任务数据表格 (`components/jobs-table.tsx`)**：
  - 列：`ID`、`文件名`（带音视频类型图标）、`模型`、`状态`（多彩 Badge）、`进度`（进度条）、`时长`、`创建时间`、`操作`（查看详情按钮、查看终端日志）；
  - 交互：支持行点击跳转 `/asr/jobs/[id]`。
- **新建任务对话框 (`components/new-job-dialog.tsx`)**：
  - 拖拽/点击上传：支持 `.mp3, .wav, .m4a, .mp4, .mkv, .mov, .flv, .webm`，直观显示文件名称与大小；
  - 模型选择下拉框：默认选中 `mock-whisper-base`，实时拉取可用列表；
  - 语言选择下拉框：自动检测 (Auto) / 简体中文 / English / 日语 等；
  - 任务类型选择：语音转录 (Transcribe) / 语音翻译 (Translate)；
  - 高级选项折叠栏：提示词 (Prompt)、采样温度 (Temperature 0~1)；
  - 提交后按钮展示 Loading Spinner，成功后 Toast 提示并自动导航至 `/asr/jobs/[id]`。

### 4.2 用户端：任务详情与音字联动工作台 (`/asr/jobs/[id]`)
- **顶部面包屑与状态头 (`components/job-header.tsx`)**：
  - 面包屑：`ASR / 作业 #10001`；
  - 文件名、模型名、总时长、创建时间、当前状态 Badge；
  - 右侧操作群组：一键复制文本、多格式导出下拉菜单（TXT / SRT / VTT / JSON）。
- **音频同步播放器 (`components/audio-player.tsx`)**：
  - 嵌入式 HTML5 音频控制器，音频源直连 `/api/v1/agent/jobs/:id/media`（或 storage）；
  - 具备播放/暂停、音量调节、快进/快退 5 秒、当前播放秒数精确提示；
  - 暴露 `currentTime` 状态供字幕段落高亮联动，并提供 `seekTo(seconds)` 方法。
- **转录结果面板 (`components/transcript-viewer.tsx`)**：
  - 支持 **「时间轴段落 (Timeline Segments)」** 与 **「纯文本 (Full Text)」** 双视图切换；
  - **时间轴段落视图**：
    - 展示每个段落的起止时间戳 `[00:02.500 --> 00:08.200]`；
    - 随着音频播放，当前播放区间段落实时高亮滚动聚焦；
    - 点击任意段落的时间戳标签，音频播放器自动 seek 到对应秒数开始播放，提供极致听对校对体验。
- **实时日志终端 (`components/live-terminal.tsx`)**：
  - 黑色背景（等宽代码字体），流式打印服务端通过 SSE (`/api/v1/jobs/:id/stream`) 推送的实时日志；
  - 顶部带日志条数统计、自动滚屏开关 (Auto Scroll)、一键清屏/全屏切换；
  - 任务转录中时默认展开显示，转录完成后可折叠收起。

---

### 4.3 管理员端：ASR 集群管理中心 (`/admin/asr`)
采用三 Tab 结构：

#### 4.3.1 Tab 1: 节点管理 (Node Management)
- **集群顶层指标**：
  - 在线节点数 / 总节点数、活跃任务总并发、集群 CPU 平均占用率；
  - 右上角「新增节点」按钮。
- **节点网格/表格 (`components/nodes-tab.tsx`, `components/node-card.tsx`)**：
  - 节点名称、主机 IP、在线状态指示灯（在线绿色脉冲，掉线灰色离线）；
  - 硬件占用率：CPU 百分比进度条、内存进度条（如 `3.8 GB / 16.0 GB`）、GPU 百分比；
  - 运行中任务数 (`Running Jobs`)；
  - 当前已加载模型徽章列表，附带快捷「卸载 (`X`)」操作；
  - 卡片操作项：
    - 「加载模型」按钮：打开弹窗选择待下发模型，下发 `load-model` 指令；
    - 「查看详情」按钮：呼出右侧抽屉展示详尽硬件监控遥测与实时会话属性；
    - 「删除节点」按钮：二次确认警告框，执行 `DELETE /api/v1/controller/nodes/:id`。
- **新增节点向导 (`components/create-node-dialog.tsx`)**：
  - 输入节点标识名称；
  - 提交成功后弹出一键启动指引卡片：
    - 显示该节点专属的 `agent_token`（附带一键复制按钮）；
    - 显示预制好的 Python 启动命令行，方便运维快速复制部署：
      ```bash
      CONTROLLER_URL=http://your-server:8080 AGENT_TOKEN=atk_xxx NODE_NAME=node-1 uv run python -m src.main
      ```

#### 4.3.2 Tab 2: 模型管理 (Model Management)
- **平台模型数据表格 (`components/models-tab.tsx`)**：
  - 列：模型唯一标识（`name`）、任务类型（`task_type`）、模型描述（`description`）、创建时间、启用开关（`Switch`）；
  - 点击 Switch 开关实时调用 `PUT /api/v1/controller/models/:id/status` 更新模型可用性；
  - 顶部快速操作台：支持向任意在线节点批量或指定下发加载/卸载模型指令。

#### 4.3.3 Tab 3: ASR 任务全景 (All Jobs Panorama)
- **穿透租户的全平台作业表格 (`components/all-jobs-tab.tsx`)**：
  - 复合筛选栏：按状态、所属用户 UID、执行节点 ID、模型名称检索；
  - 数据表展示用户 ID、任务 ID、文件名、节点、进度、状态、耗时、时间；
  - 操作栏：**「查看全景详情」** 呼出右侧深度检查抽屉。
- **全景深度检查抽屉 (`components/job-deep-inspector.tsx`)**：
  - 提交用户 UID 与用户画像信息；
  - 执行节点详细信息与分配历史；
  - 底层音频存储物理路径 (`audio_storage_path`) 与一键直链下载；
  - 性能分解：音频时长、总转录耗时、实时速率比；
  - 异常诊断：失败任务直显底层调用栈与系统 `error_msg`；
  - 完整执行日志回放面板；
  - 原始 OpenAI JSON 响应报文（带格式化高亮与一键复制）。

---

## 5. 前端 API 服务层与数据协议 (`frontend/lib/services/transcribe`)

继承 `BaseService` 规范构建：
1. `TranscribeService` (`frontend/lib/services/transcribe/transcribe-service.ts`):
   - `submitTranscription(formData)`: 发起 multipart 转录任务；
   - `listMyJobs(params)`: 查询个人作业；
   - `getJobDetail(id)`: 查询作业详情；
   - `streamJobLogs(id, onMessage, onFinish, onError)`: 原生 `EventSource` 包装；
   - `listModels()`: 获取可用模型列表。
2. `AdminTranscribeService` (`frontend/lib/services/transcribe/admin-transcribe-service.ts`):
   - `listNodes(keyword)`: 查询所有 Agent 节点及其实时指标；
   - `getNodeDetail(id)`: 查询单节点详细监控；
   - `createNode(name)`: 创建节点获取 Token；
   - `deleteNode(id)`: 删除节点；
   - `loadModel(nodeId, modelName)`: 向节点下发加载模型；
   - `unloadModel(nodeId, modelName)`: 向节点下发卸载模型；
   - `listAllModels(keyword)`: 管理员获取全部模型；
   - `toggleModelStatus(id, isActive)`: 切换模型启停状态；
   - `listAllJobs(params)`: 管理员全平台任务大盘查询。
3. 服务注册：在 `frontend/lib/services/index.ts` 导出并挂载至统一单例。
