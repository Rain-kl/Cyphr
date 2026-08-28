# Zero-Redis Pluggable Architecture Design

**Date**: 2026-08-28  
**Topic**: Decoupling Redis via Cordis Pluggable Infrastructure and In-Process Drivers (Zero-Redis Monolith Mode)  
**Status**: Approved  

---

## 1. Background & Objectives

Currently, the Wavelet platform has direct or indirect couplings with Redis across four areas:
1. **Cache Layer (`infra/cache`)**: Hardcoded initialization of Redis client and L2 cache lookup.
2. **Background Worker & Cron Drivers (`drivers/driver_asynq_*`)**: Asynq requires Redis as message queue and timer broker.
3. **Cross-Node Invalidation (Pub/Sub)**: Invalidation messages directly interact with Redis channels.
4. **Task Execution Log Stream**: Real-time worker output writes directly to Redis pipelines in `admin/repository.go`.

**Goal**:
In accordance with Cordis's "Everything is a Plugin" and "Single Owner Principle", extract Redis into dedicated optional plugins and provide lightweight in-process equivalents (`cache_memory`, `driver_inproc_worker`, `driver_inproc_cron`) so that standalone monolith deployments, embedded scenarios, and local development can run with zero external Redis dependency.

---

## 2. Architecture & Detailed Design

### 2.1 Cache Infrastructure Split (`backend/plugins/infra/`)

`contracts.CacheService` remains the sole contract for caching. Two alternative plugins implement this contract:

1. **`plugins/infra/cache_memory` (Default for Monolith / Zero-Redis)**:
   - Encapsulates `pkg/cache/ram` for fast in-process TTL caching.
   - Cache invalidations emit `cache:invalidate` events via `ctx.Events()` locally.
   - Provides `core.Provide[contracts.CacheService](ctx, memCacheSvc)`.
2. **`plugins/infra/cache_redis` (Distributed Cluster Mode)**:
   - Provides full multi-tier caching: L1 Local RAM + L2 Remote Redis + Redis Pub/Sub invalidation.
   - Implements `core.DependentPlugin` (declares dependencies on database / configuration).
   - Provides `core.Provide[contracts.CacheService](ctx, redisCacheSvc)`.

---

### 2.2 In-Process Worker & Scheduler Drivers (`backend/plugins/drivers/`)

Domain plugins register tasks and schedules only against `ctx.Tasks()` and `ctx.Schedules()` extension points, completely oblivious to the underlying runner.

1. **`plugins/drivers/driver_inproc_worker` (In-Process Worker Driver)**:
   - Implements `core.Driver` with `Type() == core.DriverTypeWorker`.
   - Maintains an in-memory buffered channel queue and worker goroutine pool (managed via `util.Go` with panic recovery).
   - Supports task concurrency limits, exponential backoff retries, and context execution timeouts.
2. **`plugins/drivers/driver_inproc_cron` (In-Process Cron Scheduler Driver)**:
   - Implements `core.Driver` with `Type() == core.DriverTypeScheduler`.
   - Uses `robfig/cron/v3` to poll and trigger entries in `ctx.Schedules().Schedules()`.
3. **`plugins/drivers/driver_asynq_*` (Distributed Cluster Drivers)**:
   - Retains Asynq worker and cron drivers for Redis-backed distributed workloads.

---

### 2.3 Task Execution Log Stream & Event Bus Decoupling

1. **Task Stream Logs**:
   - Provide an in-memory `RingBuffer` (e.g. recent 500 lines per execution).
   - When Redis is disabled, logs stream into the `RingBuffer` and flush to `w_task_executions` upon completion.
2. **System Config & Invalidation Broadcast**:
   - In single-node mode, `ctx.Events()` in-process event bus handles all notifications immediately.
   - In multi-node mode, `cache_redis` bridges events across instances via Redis Pub/Sub.

---

### 2.4 Application Assembly (`backend/cmd/app.go`)

In `cmd/app.go`, the application declaratively selects the plugin suite based on configuration:

```go
if config.Config.Redis.Enabled {
    app.Use(
        cache_redis.New(),
        driver_asynq_worker.New(),
        driver_asynq_cron.New(),
    )
} else {
    app.Use(
        cache_memory.New(),
        driver_inproc_worker.New(),
        driver_inproc_cron.New(),
    )
}
```

---

## 3. Verification Plan

1. **Unit Tests**:
   - `plugins/infra/cache_memory/plugin_test.go`: Test in-memory cache operations, TTL expiry, and `contracts.CacheService` compliance.
   - `plugins/drivers/driver_inproc_worker/plugin_test.go`: Test in-process task dispatch, concurrency, retry, and cancellation.
   - `plugins/drivers/driver_inproc_cron/plugin_test.go`: Test in-process cron schedule execution.
2. **Integration Verification**:
   - Verify that running the application with `config.Database.Enabled = false` and `config.Redis.Enabled = false` boots cleanly into `all`, `api`, `worker`, and `scheduler` profiles with zero connection errors.
3. **Quality Gate**:
   - Run `go test ./...`, `make code-check`, and `make format`.
