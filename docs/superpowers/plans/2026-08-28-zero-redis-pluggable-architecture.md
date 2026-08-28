# Zero-Redis Pluggable Architecture Implementation Plan

> **Goal**: Extract Redis into optional plugins and introduce lightweight in-process equivalents (`cache_memory`, `driver_inproc_worker`, `driver_inproc_cron`), enabling zero-Redis monolithic and embedded deployment modes.

- **Architecture Spec**: [`docs/superpowers/specs/2026-08-28-zero-redis-pluggable-architecture-design.md`](file:///Users/ryan/Code/Go/Wavelet/docs/superpowers/specs/2026-08-28-zero-redis-pluggable-architecture-design.md)
- **Branch**: `main`

---

## Proposed Changes

### 1. In-Memory Cache Infrastructure Plugin (`backend/plugins/infra/cache_memory`)

#### [NEW] `backend/plugins/infra/cache_memory/plugin.go`
- Implements `core.Plugin` (`Name() == "cache_memory"`).
- Applies `contracts.CacheService` to the Context via `core.Provide[contracts.CacheService](ctx, memCacheSvc)`.

#### [NEW] `backend/plugins/infra/cache_memory/cache.go`
- Implements `contracts.CacheService` using `pkg/cache/ram`.
- Dispatches in-process invalidation notifications via `ctx.Events().Emit("cache:invalidate", key)`.

#### [NEW] `backend/plugins/infra/cache_memory/plugin_test.go`
- Unit tests for Get, Set, Delete, GetOrSet, TTL expiration, and event bus emission.

---

### 2. In-Process Async Worker Driver (`backend/plugins/drivers/driver_inproc_worker`)

#### [NEW] `backend/plugins/drivers/driver_inproc_worker/plugin.go`
- Implements `core.Plugin` & `core.Driver` (`Type() == core.DriverTypeWorker`).
- Scans and executes registered tasks from `ctx.Tasks().Tasks()`.

#### [NEW] `backend/plugins/drivers/driver_inproc_worker/executor.go`
- In-memory buffered channel queue and worker goroutine pool managed via `util.Go`.
- Supports execution timeout, retry with backoff, and graceful shutdown.

#### [NEW] `backend/plugins/drivers/driver_inproc_worker/plugin_test.go`
- Unit tests for in-process task execution, concurrency limit, retry on error, and graceful shutdown.

---

### 3. In-Process Cron Scheduler Driver (`backend/plugins/drivers/driver_inproc_cron`)

#### [NEW] `backend/plugins/drivers/driver_inproc_cron/plugin.go`
- Implements `core.Plugin` & `core.Driver` (`Type() == core.DriverTypeScheduler`).
- Reads `ctx.Schedules().Schedules()` and schedules jobs using `robfig/cron/v3`.

#### [NEW] `backend/plugins/drivers/driver_inproc_cron/scheduler.go`
- Handles Cron expression registration, job triggering, and graceful stopping.

#### [NEW] `backend/plugins/drivers/driver_inproc_cron/plugin_test.go`
- Unit tests verifying cron job scheduling, execution tracking, and stop behavior.

---

### 4. Admin Domain Decoupling from Redis

#### [MODIFY] `backend/plugins/domain/admin/repository.go`
- Introduce in-memory `RingBuffer` for task output streams when Redis is nil.
- Fallback task log lookups to `RingBuffer` and `w_task_executions` table.

#### [MODIFY] `backend/plugins/domain/admin/system_config_cache.go`
- Guard Redis PubSub listener so that when Redis is nil, it gracefully falls back to local event bus updates without spawning disconnected subscriber loops.

---

### 5. Application Assembly & Profile Switching

#### [MODIFY] `backend/cmd/app.go`
- Switch dynamically between Redis plugins (`cache`, `driver_asynq_worker`, `driver_asynq_cron`) and In-Process plugins (`cache_memory`, `driver_inproc_worker`, `driver_inproc_cron`) based on `config.Config.Redis.Enabled`.

#### [MODIFY] `backend/cmd/app_test.go`
- Add test verifying application bootstrap in both `Redis.Enabled = true` and `Redis.Enabled = false` states.

---

## Verification Plan

### Automated Tests
1. **In-Memory Cache Tests**: `go test -v ./backend/plugins/infra/cache_memory/...`
2. **In-Process Worker Tests**: `go test -v ./backend/plugins/drivers/driver_inproc_worker/...`
3. **In-Process Cron Tests**: `go test -v ./backend/plugins/drivers/driver_inproc_cron/...`
4. **Full Test Suite**: `cd backend && go test ./...`
5. **Quality Gate**: `make code-check && make format`
