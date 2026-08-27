# AGENTS.md

Behavioral guidelines to reduce common LLM coding mistakes. Merge with project-specific instructions as needed.

**Tradeoff:** These guidelines bias toward caution over speed. For trivial tasks, use judgment.

## 1. Think Before Coding

**Don't assume. Don't hide confusion. Surface tradeoffs.**

Before implementing:
- State your assumptions explicitly. If uncertain, ask.
- If multiple interpretations exist, present them - don't pick silently.
- If a simpler approach exists, say so. Push back when warranted.
- If something is unclear, stop. Name what's confusing. Ask.

## 2. Simplicity First

**Minimum code that solves the problem. Nothing speculative.**

- No features beyond what was asked.
- No abstractions for single-use code.
- No "flexibility" or "configurability" that wasn't requested.
- No error handling for impossible scenarios.
- If you write 200 lines and it could be 50, rewrite it.

Ask yourself: "Would a senior engineer say this is overcomplicated?" If yes, simplify.

## 3. Surgical Changes

**Touch only what you must. Clean up only your own mess.**

When editing existing code:
- Don't "improve" adjacent code, comments, or formatting.
- Don't refactor things that aren't broken.
- Match existing style, even if you'd do it differently.
- If you notice unrelated dead code, mention it - don't delete it.

When your changes create orphans:
- Remove imports/variables/functions that YOUR changes made unused.
- Don't remove pre-existing dead code unless asked.

The test: Every changed line should trace directly to the user's request.

## 4. Goal-Driven Execution

**Define success criteria. Loop until verified.**

Transform tasks into verifiable goals:
- "Add validation" → "Write tests for invalid inputs, then make them pass"
- "Fix the bug" → "Write a test that reproduces it, then make it pass"
- "Refactor X" → "Ensure tests pass before and after"

For multi-step tasks, state a brief plan:
```
1. [Step] → verify: [check]
2. [Step] → verify: [check]
3. [Step] → verify: [check]
```

Strong success criteria let you loop independently. Weak criteria ("make it work") require constant clarification.

---

**These guidelines are working if:** fewer unnecessary changes in diffs, fewer rewrites due to overcomplication, and clarifying questions come before implementation rather than after mistakes.

## Git 提交规范

遵循 Conventional Commits：`<type>(<scope>): <subject>`（例：`feat(auth): support email login`）。

## 务必阅读匹配的 Skill

| Skill | 何时使用 |
| :--- | :--- |
| `new-api` | 添加或修改自定义业务 API、Handler、服务层逻辑、自定义路由注册 |
| `new-async-task` | 添加或修改 Asynq 任务、定时任务、TaskHandler、任务元数据 |
| `new-setting` | 添加或修改系统/业务/公开设置、`/admin/system` 参数或 `/admin/settings` 图形化设置 |
| `database-migration` | 数据库表结构变更、goose SQL 迁移（PG/SQLite/ClickHouse）、seed 数据 |
| `logstore` | 日志/分析用途表、`internal/repository/logstore`、切换日志主库、PG/SQLite 回落 |
| `clickhouse-batchwriter` | ClickHouse 批量写入、`internal/infra/persistence/batchwriter` 接入、分析表异步 flush、背压与写入路径改造 |
| `file-upload` | 业务上传文件、Worker 程序化摄取、`upload.Ingest` 策略选型、文件访问与 `w_uploads` / 统计排查 |
| `cache-framework` | 新增或修改业务缓存（RAM/Redis/DB 三层读路径）、缓存失效、多节点 pub/sub 同步、评估高频读是否应接入缓存 |
| `push-notification` | 系统通知推送事件、统一触发器投递、带消息推送的业务功能 |
| `release-guide` | 根据自上一正式版本 Tag 以来的提交整理 Version Bump 提交信息以触发双语 Release |
| `shadcn` | 添加、修改或组合 shadcn/ui 组件 |

## 严格遵循事项 (Guardrails)

- 切勿删除 `frontend/node_modules`。
- 保持 `internal/util/` 绝对纯净，禁止导入 Gin、GORM、sessions 等 Web/数据库框架包。
- 测试用例禁止硬编码相对路径创建临时目录，统一使用 Go 内置 `t.TempDir()`。
- 所有 HTTP 路由仅在 `internal/router/router.go` 中作为高层分发注册。
- 修改 API Handler 后运行 `make swagger`，完成代码开发后必须依次运行 `make code-check` 与 `make format`。
- 业务模块必须复用平台缓存/文件服务：文件摄取统一用 `upload.Ingest`，删除用 `upload.Remove`/`upload.RemoveOwned`；禁止直接写 `w_uploads` 或绕过 upload 域直接操作 `infra/objectstore`。
- 禁止在 `init()` 中注册跨模块集成（任务 Handler、推送事件、域事件监听器等），统一在 `internal/platform/bootstrap` 显式装配并在 `internal/cmd` 入口调用。
- 核心业务模块（`oauth`、`user`）禁止直接 import `push` 或 `custom_events` 触发通知，须通过 `internal/listener` 发射域事件。
- API 错误响应必须通过 `response.Abort*` 中断请求，由 `ErrorHandlerMiddleware` 统一写出 JSON 并记录 Trace；禁止在 Handler/中间件中直接 `c.JSON(status, response.Err(...))` 或 `200` 返回 `error_msg`。
- **分层**：`apps → repository → model`，`repository → infra/persistence`；禁止 `model → repository`。
  - `model`：实体、表名、配置 key、查询 DTO、无 IO 规则。禁止 `db.DB` / Redis / CH；禁止 `import repository`。GORM hook 仅可 mutate 自身字段，禁止在 hook 内再查 DB/缓存。
  - `repository`：唯一持久化入口。apps/logics 禁止为业务 CRUD 直调 `db.DB`（管理端 SQL 控制台、infra 内部等例外保留）。禁止新增 `model.Get/List/Create/...` 类数据访问 API。
- 日志/分析表（访问日志、审计流水、可观测时序）走 `internal/repository/logstore`，禁止 apps 直连 `repository/analytics` 或 `db.ChConn`/`db.ChDB`。判定与接入步骤见 `logstore` skill。

## 技术栈与项目目录结构

### 技术栈
- **后端**：Go 1.25+、Gin、GORM、PostgreSQL、可选 ClickHouse、Redis、Asynq、Cobra、Viper、Swaggo、OpenTelemetry、Zap、AWS SDK v2。
- **前端**：Next.js (App Router)、TypeScript、Tailwind CSS、pnpm、shadcn/ui。

## 后端开发规范

### API 响应规范
- **统一信封**：`{ "error_msg": "", "data": ... }`
- **成功**：HTTP 200，写出 `c.JSON(http.StatusOK, response.OK(data))` 或 `response.OKNil()`。
- **失败**：使用 `internal/shared/response` 的 `Abort*` 系列函数（如 `AbortBadRequest`、`AbortUnauthorized`、`AbortNotFound`、`AbortInternal`）中断请求。
- **错误文案**：使用模块内 `errs.go` 中的 camelCase 字符串常量（如 `errBindParamsFailed`），禁止暴露底层数据库/系统错误细节给客户端。
- **Logics 分工**：`logics.go` 只接受 `context.Context`，返回 `(result, error)`，严禁依赖 `*gin.Context` 或调用 `c.JSON`/`Abort*`。
- **错误日志**：底层错误在 Handler/Logic 边界用 `pkg/logger` 打印日志，禁止使用 `_ = ...` 静默吞掉关键错误。

### 数据库操作
- 平台域（user、auth_source、access_token、schedule、task_execution）的持久化必须走 `internal/repository`，禁止在 `internal/model` 中调用 `db.DB` / Redis。
- 管理员代码推荐使用 `db.DB(ctx)`（`internal/infra/persistence`，包名 `db`）保证 Trace 链路透传。
- 禁止在 Handler 写复杂 SQL；迁移文件位于 `internal/infra/persistence/migrator/goose/`（禁止 GORM AutoMigrate）。
- 不创建物理外键（显式建索引）；Go 模型零值需与数据库默认值匹配。
- **SQL LIKE 查询防注入与转义**：所有含用户输入的模糊查询必须调用 `pkg/util.EscapeLike` 转义通配符，并显式指定 `ESCAPE '\\'` 语法（如 `Where("username LIKE ? ESCAPE '\\'", util.EscapeLike(keyword)+"%")`），同时兼容 PostgreSQL 与 SQLite 方言并杜绝通配符注入攻击。

### 并发与安全防护规范
- **Goroutine 安全**：禁止直接使用裸 `go func()`；统一使用 `pkg/util.Go`，确保具备未捕获 panic 恢复和调用栈日志记录能力。
- **Pub/Sub 监听并发安全**：启动 Redis Pub/Sub 订阅监听前，必须捕获局部客户端实例（如 `redisClient := db.Redis`），禁止在 goroutine 闭包中直读可变全局 `db.Redis`；提供 `Stop*Listener` 时必须维护 `done` 通道等待 goroutine 完整退出后再重置状态，消除测试或重连时的数据竞争。
- **Session 固定攻击防御**：用户登录/授权成功后，必须调用 `oauth.SetLoginSession`（内部执行 Session ID 轮换），防止 Session 固定攻击。
- **防账户枚举与时序攻击**：
  - 登录失败统一返回模糊报错；当查询用户不存在时，必须调用 `pkg/util.DummyCheckPassword` 执行同等开销的 bcrypt 哈希计算，彻底消除时序侧信道攻击。
  - 验证码、签名 Token 等敏感字符串比对必须使用 `crypto/subtle.ConstantTimeCompare` 常量时间比对。
- **敏感端点限流**：登录尝试、OAuth 授权发起等敏感接口必须接入基于 Redis 的滑动窗口限流机制，防止暴力破解与缓存资源耗尽。

## 前端开发规范

- 新特性开发前参考 Next.js 文档与 `frontend/app/(main)/admin/demo` 示例代码。
- **页面容器与标题栏**：
    - 页面根容器统一使用全宽 `w-full`，最外层统一用 `py-6` 或 `py-6 px-1` 对齐边距。
    - 标题容器统一 `flex items-center gap-2`（带操作按钮用 `justify-between`）。
    - 图标直接使用 Lucide 组件（`size-5 text-primary`），禁止包裹背景小卡片或装饰边框。
    - 标题文字统一使用 `<h1 className="text-2xl font-semibold tracking-tight">`。
- **无障碍语义与色彩规范 (a11y & WCAG)**：
    - **标题层级规范 (Heading Hierarchy)**：页面中非顶级结构化标题（如空状态提示、加载提示、卡片眉题/卡片标题、抽屉区块名）严禁滥用 `<h3>`/`<h4>`，统一使用 `<p>` 配合样式，保证屏幕阅读器感知的标题层级连续。
    - **无文本控件无障碍**：所有仅包含图标的按钮（如仅有 Icon 的 Button、Switch、无文本的 SelectTrigger）必须显式添加 `aria-label`。
    - **色彩对比度**：正文、提示、徽章等小字颜色在亮色/暗色模式下必须满足 WCAG AA（对比度 ≥ 4.5:1）。
- **组件拆分与维护**：
    - 物理路由页面 `page.tsx` 仅维护高级骨架与布局。
    - 单文件超过 600 行或含多 Tab/大复杂区块时，必须按就近原则拆分为子组件存放在路由同级的 `components/` 局部目录中（参考 `/admin/database` 的模块化拆分结构）。
- **样式与服务**：
    - 优先使用 shadcn/ui 的 `variant` 和全局 CSS 变量，不要在业务代码中硬编码颜色/背景。
    - 前端请求统一在 `frontend/lib/services/<name>/` 中继承 `BaseService` 编写并在 `index.ts` 注册。
- **国际化 (i18n)**：
    - 使用 `next-intl`（**无 URL locale 前缀** / non-routing provider 模式），兼容 `NEXT_STANDALONE_EXPORT` 静态导出。
    - 支持语言：`zh-CN`、`en`；默认 `zh-CN`。
    - 解析优先级：cookie `NEXT_LOCALE`（用户显式选择）→ 浏览器语言 → 默认 `zh-CN`。
    - 文案统一放在 `frontend/messages/{locale}.json`，按命名空间嵌套（`common` / `layout` / `auth` / `settings` / 业务域）。
    - 组件内用户可见文案必须通过 `useTranslations()` / `getTranslations()` 读取；**禁止**新增中英硬编码 UI 字符串（后端返回的 `error_msg`、日志、调试信息除外）。
    - key 使用 camelCase 分层（如 `auth.login.submit`）；完整短语作为 value，禁止在组件内拼接句子。
    - 新增或修改文案时必须**同步**更新 `zh-CN.json` 与 `en.json`，保持 key 树一致。
    - 语言选项展示用自称：`中文` / `English`（不随当前 UI 语言翻译）。
    - 日期/数字格式化使用 locale 感知 helper（如 `formatDateTime`），禁止写死 `'zh-CN'` / `date-fns` 的 `zhCN`（除非该路径尚未迁移且不在本次改动范围）。
    - 设计说明见 `docs/superpowers/specs/2026-07-24-frontend-i18n-design.md`。
