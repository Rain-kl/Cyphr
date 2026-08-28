# Autoresearch: Cordis 架构合规 + Bug 修复 + 性能 + 代码质量

## Objective
Wavelet 后端（Go, Cordis 插件化微内核架构）全面质量提升：修复违反 Cordis 设计原则的地方、修复 bug、消除潜在性能问题、减少代码重复（dupl）、提升可维护性。主指标为 golangci-lint 问题总数，逐实验递减，且不得以作弊手段（nolint / 弱化配置 / 删测试）达成。

## Metrics
- **Primary**: lint_issues (count, lower is better) — `golangci-lint run` 报告的问题总数（基线 45）
- **Secondary**: dup_issues (dupl 专项计数), tests_passed (go test 通过包数，不得下降)

## How to Run
`./.auto/measure.sh` — 输出 `METRIC lint_issues=N` / `METRIC dup_issues=N` / `METRIC tests_passed=N`。

## Files in Scope
- `backend/core/` 微内核（context.go/container.go/events.go/app.go/extpoints/contracts）— 只允许更纯粹，禁止引入框架依赖
- `backend/plugins/{domain,infra,drivers}/` 所有插件 — 重复代码消除、bug 修复
- `backend/pkg/` 基础库
- `scripts/check_cordis_architecture.sh` 架构守门脚本（只可增强、不可弱化）
- `.auto/*` 会话文件

## Off Limits（作弊红线）
- ❌ 禁止修改 `.golangci.yml` 弱化 lint（禁阈值调高、禁关 linter、禁 excludes）
- ❌ 禁止 `//nolint` 注释压制问题
- ❌ 禁止删除测试或功能来消除告警（删除死代码需先证明确实无人引用）
- ❌ 禁止违反 `backend/pkg/util/` 纯净性、`backend/core/` 微内核纯净性、contracts 纯抽象
- ❌ 修复行为 bug 必须带回归测试或明确论证；tests_passed 不得下降
- ✅ 允许：重构提取共享 helper（插件内）、加注释、常量化字符串、拆复杂嵌套、修 gosec 权限、优化 SQL/锁/分配

## Constraints
- `.auto/checks.sh` 必须通过：`go build` + `go test ./...` + Cordis 架构检查全绿
- 插件间严禁跨包 import，只能走 `core/contracts` + EventBus
- 裸 `go func()` 禁止，统一 `util.Go`；SQL LIKE 必须 `util.EscapeLike` + `ESCAPE '\\'`
- API Handler 改动后跑 `make swagger`（若改了 handler 签名/路由）

## What's Been Tried
### 基线状态（2026-08-28, 45 lint issues）
- **nilerr 真 bug 候选**: `backend/plugins/domain/admin/repository.go:507` err!=nil 却 return nil
- **contextcheck**: `driver_inproc_cron/plugin.go:70`、`driver_inproc_worker/plugin.go:133` 未传 context
- **dupl 重复块**: core/extpoints{migration,setting,schedule,task} 四处同构注册代码；message_gateway{admin_handlers:55-67,115-130,143-158 ↔ push_handlers:61-74,77-93 ↔ push_channels:218-231,270-286}; message_gateway/repository.go:222-240↔332-350; driver_asynq_worker/plugin.go:371-390↔420-439
- **nestif 深嵌套**: admin/handlers_config.go:442(complexity 6), upload/filesrv/file_server.go:330(6), driver_asynq_cron/plugin.go:142(9), driver_asynq_cron/scheduler.go:119(5)
- **gosec**: upload/shared/test_helpers.go:133(G301),134(G306),152(G304); infra/database/postgres.go:49(G301)
- **goconst**: "sqlite"(admin/handlers_db.go:150,handlers_status.go:179), "upload"/"default"(upload/task/*)
- **revive**: storage.go:53 缺注释, db_helper.go{admin:118,125; message_gateway:25; upload/storage/migration.go:40} 未用参数, handlers_config.go:510 未用参数
- **mnd**: pkg/cache/disk/cache.go:100 魔法数 10
- **gofumpt**: upload/stats/{category,stats_counter}.go, driver_http/db_helper.go 格式错误
- 前端 eslint/tsc 已全绿；架构检查脚本 0 违规

### 教训
- （空 — 随实验更新）
