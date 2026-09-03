# ASR 语音转录前端与全接口对齐实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 构建专业、直观且 100% 对齐后端管理接口的 ASR 语音转录前端系统，包含用户端转录工作台与音字联动校验、管理员端 Agent 节点集群管控、模型启停分发控制台与全平台作业大盘。

**Architecture:** 
- 前端基于 Next.js 15 App Router 与 React 19，采用 shadcn/ui 组件库与 `next-intl` 国际化系统；
- 服务层统一继承 `BaseService` 封装标准请求与 SSE (`EventSource`) 长连；
- 后端轻量扩展补齐 Controller 管理接口（删除节点、节点详情、启停模型、全量任务大盘），确保前后端接口 100% 深度对齐。

**Tech Stack:** Next.js, React 19, TypeScript, Tailwind CSS, shadcn/ui, Lucide Icons, next-intl, Go (Gin, GORM, Cordis Microkernel).

## Global Constraints

- **接口 100% 对齐**: 服务端暴露的所有 Controller 接口必须在前端具备可操作的交互实体。
- **严禁删除 node_modules**: 严格遵循项目 Guardrails，禁止删除 `frontend/node_modules`。
- **完全国际化 (i18n)**: 统一在 `frontend/messages/zh-CN.json` 与 `en.json` 挂载文案，禁止硬编码 UI 字符串。
- **页面规范**: 页面根容器全宽 `w-full`，最外层间距 `py-6 px-1`；标题使用 `<h1 className="text-2xl font-semibold tracking-tight">`；图标使用 Lucide 组件 `size-5 text-primary`。
- **组件拆分**: 单文件代码小于 600 行，复杂区块与 Tab 按就近原则拆分至同级 `components/` 目录。
- **质量门禁**: 完成开发后依次通过 `make code-check` (Cordis 架构检查 0 violations, golangci-lint 0 issues, eslint 0 warnings) 与 `make format`。
- **Git 规范**: 每个任务完成后提交 Conventional Commit，严禁推送至远程仓库。

---

### Task 1: 后端管理接口补全与对齐 (Backend Admin APIs Alignment)

**Files:**
- Modify: `backend/transcribe/plugins/svr/dao/node_dao.go` (新增 `Delete`, `GetByID`)
- Modify: `backend/transcribe/plugins/svr/dao/model_dao.go` (新增 `ListAll`, `UpdateStatus`)
- Modify: `backend/transcribe/plugins/svr/dao/job_dao.go` (增强 `ListByUserID` 支持 `uid=0` 查全量)
- Modify: `backend/transcribe/plugins/svr/service/node_service.go` (新增 `DeleteNode`, `GetNodeDetail`)
- Modify: `backend/transcribe/plugins/svr/service/model_service.go` 或扩展 DAO (支持启停模型)
- Modify: `backend/transcribe/plugins/svr/controller/controller_node_handler.go` (挂载 `DELETE /nodes/:id`, `GET /nodes/:id`)
- Modify: `backend/transcribe/plugins/svr/controller/model_handler.go` (挂载 `GET /controller/models`, `PUT /controller/models/:id/status`)
- Modify: `backend/transcribe/plugins/svr/controller/job_handler.go` (挂载 `GET /api/v1/controller/jobs`)
- Modify: `backend/transcribe/plugins/svr/controller/controller.go` (注册路由)
- Test: `backend/transcribe/plugins/svr/controller/controller_test.go`

**Interfaces:**
- Produces:
  - `DELETE /api/v1/controller/nodes/:id`: 注销/删除节点
  - `GET /api/v1/controller/nodes/:id`: 获取节点详细监控与任务信息
  - `GET /api/v1/controller/models`: 管理员获取全量模型列表（含禁用）
  - `PUT /api/v1/controller/models/:id/status`: 启用/停用模型
  - `GET /api/v1/controller/jobs`: 管理员查询全平台作业列表（支持用户、节点、状态、关键词筛选）

- [ ] **Step 1: 在 DAO 与 Service 层扩展接口**
- [ ] **Step 2: 在 Controller 层实现 Handler 并注册路由**
- [ ] **Step 3: 编写单元测试验证各接口返回格式**
- [ ] **Step 4: 运行 `go test -v ./transcribe/plugins/svr/...` 确保 PASS**
- [ ] **Step 5: Git Commit**
  `git commit -m "feat(transcribe): add admin node delete, model status toggle and all jobs endpoints"`

---

### Task 2: 前端服务契约、侧边栏导航与国际化字典 (Frontend Services, Nav & i18n)

**Files:**
- Create: `frontend/lib/services/transcribe/types.ts`
- Create: `frontend/lib/services/transcribe/transcribe-service.ts`
- Create: `frontend/lib/services/transcribe/admin-transcribe-service.ts`
- Modify: `frontend/lib/services/index.ts`
- Modify: `frontend/components/layout/sidebar.tsx`
- Modify: `frontend/messages/zh-CN.json`
- Modify: `frontend/messages/en.json`

**Interfaces:**
- Produces:
  - `services.transcribe`: `submitTranscription`, `listMyJobs`, `getJobDetail`, `streamJobLogs`, `listModels`
  - `services.adminTranscribe`: `listNodes`, `getNodeDetail`, `createNode`, `deleteNode`, `loadModel`, `unloadModel`, `listAllModels`, `toggleModelStatus`, `listAllJobs`
  - 侧边栏主导航增加 `/asr`（`ASR`），管理组增加 `/admin/asr`（`ASR 管理`）

- [ ] **Step 1: 编写 TypeScript 强类型定义与 DTO 契约**
- [ ] **Step 2: 编写继承 `BaseService` 的 TranscribeService 与 AdminTranscribeService**
- [ ] **Step 3: 在 `frontend/lib/services/index.ts` 注册单例服务**
- [ ] **Step 4: 在 `AppSidebar` 挂载 `/asr` 与 `/admin/asr` 路由**
- [ ] **Step 5: 在 `zh-CN.json` 与 `en.json` 补充完整的多语言翻译命名空间**
- [ ] **Step 6: Git Commit**
  `git commit -m "feat(frontend): add transcribe services, sidebar navigation and i18n dictionaries"`

---

### Task 3: 用户端 ASR 任务列表大盘与新建弹窗 (`/asr`)

**Files:**
- Create: `frontend/app/(main)/asr/page.tsx`
- Create: `frontend/app/(main)/asr/components/stats-cards.tsx`
- Create: `frontend/app/(main)/asr/components/jobs-table.tsx`
- Create: `frontend/app/(main)/asr/components/jobs-filter.tsx`
- Create: `frontend/app/(main)/asr/components/new-job-dialog.tsx`

**Interfaces:**
- Consumes: `services.transcribe.listMyJobs`, `services.transcribe.listModels`, `services.transcribe.submitTranscription`
- Produces: 用户端 ASR 任务大盘页面，支持快速检索、状态筛选、音视频拖拽上传与提交

- [ ] **Step 1: 编写指标统计卡片 `stats-cards.tsx` 与过滤器 `jobs-filter.tsx`**
- [ ] **Step 2: 编写任务表格 `jobs-table.tsx`（支持状态徽章、进度条、分页与行点击路由跳转）**
- [ ] **Step 3: 编写新建转录弹窗 `new-job-dialog.tsx`（拖拽音视频上传、动态模型选择、语言设置、提交跳转）**
- [ ] **Step 4: 装配主页面 `frontend/app/(main)/asr/page.tsx`**
- [ ] **Step 5: Git Commit**
  `git commit -m "feat(asr): implement user transcription dashboard and job submission dialog"`

---

### Task 4: 用户端 ASR 任务详情与音字联动工作台 (`/asr/jobs/[id]`)

**Files:**
- Create: `frontend/app/(main)/asr/jobs/[id]/page.tsx`
- Create: `frontend/app/(main)/asr/jobs/[id]/components/job-header.tsx`
- Create: `frontend/app/(main)/asr/jobs/[id]/components/audio-player.tsx`
- Create: `frontend/app/(main)/asr/jobs/[id]/components/transcript-viewer.tsx`
- Create: `frontend/app/(main)/asr/jobs/[id]/components/live-terminal.tsx`
- Create: `frontend/app/(main)/asr/jobs/[id]/components/export-menu.tsx`

**Interfaces:**
- Consumes: `services.transcribe.getJobDetail`, `services.transcribe.streamJobLogs`
- Produces: 任务详情与音字联动工作台，支持音频播放、点击段落时间戳跳转回听、实时 SSE 日志推流与多格式导出（TXT/SRT/VTT/JSON）

- [ ] **Step 1: 编写任务眉部组件 `job-header.tsx` 与导出菜单 `export-menu.tsx`**
- [ ] **Step 2: 编写 HTML5 音频播放器 `audio-player.tsx`（暴露播放状态与 seekTo 接口）**
- [ ] **Step 3: 编写时间轴段落与全文双视图组件 `transcript-viewer.tsx`（实现点击时间戳联动播放）**
- [ ] **Step 4: 编写等宽终端日志流组件 `live-terminal.tsx`（接入 SSE 流、自动滚屏、全屏）**
- [ ] **Step 5: 装配主页面 `frontend/app/(main)/asr/jobs/[id]/page.tsx`**
- [ ] **Step 6: Git Commit**
  `git commit -m "feat(asr): implement job detail studio with audio-transcript sync player and live sse terminal"`

---

### Task 5: 管理端 ASR 集群、模型与任务全景中心 (`/admin/asr`)

**Files:**
- Create: `frontend/app/(main)/admin/asr/page.tsx`
- Create: `frontend/app/(main)/admin/asr/components/nodes-tab.tsx`
- Create: `frontend/app/(main)/admin/asr/components/node-card.tsx`
- Create: `frontend/app/(main)/admin/asr/components/create-node-dialog.tsx`
- Create: `frontend/app/(main)/admin/asr/components/node-detail-drawer.tsx`
- Create: `frontend/app/(main)/admin/asr/components/load-model-dialog.tsx`
- Create: `frontend/app/(main)/admin/asr/components/models-tab.tsx`
- Create: `frontend/app/(main)/admin/asr/components/all-jobs-tab.tsx`
- Create: `frontend/app/(main)/admin/asr/components/job-deep-inspector.tsx`

**Interfaces:**
- Consumes: `services.adminTranscribe` 全量接口
- Produces: 三 Tab 管理中台（节点监控与模型热加载、模型库管理、全平台任务深度检查）

- [ ] **Step 1: 编写节点管理 Tab (`nodes-tab.tsx`, `node-card.tsx`, `create-node-dialog.tsx`, `node-detail-drawer.tsx`, `load-model-dialog.tsx`)**
- [ ] **Step 2: 编写模型管理 Tab (`models-tab.tsx`)，实现启停 Switch 与节点分发联动**
- [ ] **Step 3: 编写全平台任务全景 Tab (`all-jobs-tab.tsx`) 与深度检查抽屉 (`job-deep-inspector.tsx`)**
- [ ] **Step 4: 装配主页面 `frontend/app/(main)/admin/asr/page.tsx`**
- [ ] **Step 5: Git Commit**
  `git commit -m "feat(admin-asr): implement cluster node management, model switch and all-jobs inspector"`

---

### Task 6: 联调验证、质量门禁与全分支代码审查

**Files:**
- Verify: 前端页面构建与类型检查 (`pnpm tsc --noEmit && npx eslint .`)
- Verify: 后端测试 (`cd backend && go test -v ./transcribe/...`)
- Verify: 架构合规门禁 (`make code-check` 与 `make format`)

- [ ] **Step 1: 运行全量后端单元与集成测试确保 PASS**
- [ ] **Step 2: 运行前端 TypeScript 编译与 ESLint 检查确保 0 Warnings**
- [ ] **Step 3: 运行 `make code-check` 验证 Cordis 架构合规与零违规**
- [ ] **Step 4: 运行 `make format` 格式化代码**
- [ ] **Step 5: Git Commit**
  `git commit -m "chore(transcribe): pass full-stack quality gates and verification"`
