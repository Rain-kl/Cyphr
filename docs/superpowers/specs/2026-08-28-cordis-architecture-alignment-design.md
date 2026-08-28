# Cordis Architecture Alignment & Refactoring Design

**Date**: 2026-08-28  
**Topic**: Cordis Meta-framework Alignment (Spatiotemporal Composability, Revertible Effects, Reactive Coeffects & Boundary Isolation)  
**Status**: Approved  

---

## 1. Background & Objectives

Wavelet adopts the **Cordis** micro-kernel paradigm (originating from Koishi and DeepSeek Harness) to achieve runtime composability and zero-side-effect lifecycle management.
According to the formal metatheory of Cordis (*A Programming Paradigm for Spatiotemporal Composability*), the runtime must satisfy two orthogonal requirements:
1. **Temporal Composability (时间可组合性)**: Every context mutation/registration must track an inverse operation (Revertible Effects) and automatically roll back in LIFO order upon unloading/disposing.
2. **Spatial Composability (空间可组合性)**: Components declare required coeffects/dependencies (`inject`); when dependencies become available or unavailable, the system reactively activates or deactivates components (Fiber state machine), guaranteeing **Confluence (合流)** regardless of registration order.
3. **Context as the Sole Surface & Defensive Isolation**: Eliminate cross-plugin private imports and global static singletons (`database.DB()`, global configs), strictly enforcing single-owner boundaries and `contracts` programming.

---

## 2. Architecture & Detailed Design

### 2.1 Revertible Effects & Scoped Extpoints (时间可组合性)

- **`Context` Scoped Lifetime**:
  Each plugin instance is mounted with a dedicated child context `pluginCtx := rootCtx.Fork()`.
- **Automatic Disposer Registration for Extpoints**:
  When registrations occur through `pluginCtx`, inverse operations are automatically pushed to `pluginCtx`'s Disposer stack:
  - **`Router`**: Registering a route returns a definition with an ID; `pluginCtx` records a disposer that calls `router.UnregisterByID(id)`.
  - **`Events`**: `ctx.Events().On(...)` returns a `Disposer`; when called on a scoped context (or via `ctx.On(...)`), it binds to `pluginCtx.OnDispose`.
  - **`Tasks`**: Registering an async task binds `tasks.Unregister(taskType)` to `pluginCtx.OnDispose`.
  - **`Schedules`**: Registering a cron schedule binds `schedules.Unregister(cronName)` to `pluginCtx.OnDispose`.
  - **`Settings`**: Registering setting schemas binds schema deregistration to `pluginCtx.OnDispose`.
  - **`Container (Provide)`**: Providing a service type `T` binds `container.remove(T)` to `pluginCtx.OnDispose`.
- **LIFO Teardown Guarantee**:
  Calling `pluginCtx.Dispose()` runs all registered disposers in reverse order (LIFO), cleanly revoking routes, event listeners, tasks, schedules, and service bindings without residual side effects.

---

### 2.2 Reactive Coeffects & Fiber Lifecycle (空间可组合性)

- **Dependency Declaration (`DependentPlugin`)**:
  Plugins can optionally implement:
  ```go
  type DependentPlugin interface {
      Plugin
      Inject() []reflect.Type
  }
  ```
- **Plugin Fiber State Machine**:
  ```
  PENDING  ──(All dependencies provided)──>  LOADING  ──(Apply succeeds)──>  ACTIVE
     ▲                                                                         │
     └─────────────(Dependency removed / Plugin unloaded)──────────────────────┘
  ```
  - **States**: `FiberPending`, `FiberLoading`, `FiberActive`, `FiberUnloading`, `FiberDisposed`.
  - **Reconciler**: When `core.Provide[T]` registers a service or `core.App.Use` registers a plugin, the reconciler checks all pending fibers. Fibers with satisfied dependencies transition `Pending -> Loading -> Active`.
  - **Confluence**: Plugin registration order (`app.Use(A, B)` vs `app.Use(B, A)`) produces the exact same final active state once all dependencies are satisfied.

---

### 2.3 Boundary Defense & Single Owner Enforcement (架构防线)

- **Eliminate Direct Global Invocations**:
  - Refactor `plugins/domain/user/repository.go` and other domain repositories to avoid direct `import "Wavelet/plugins/infra/database"` and direct calls to `database.DB(ctx)`.
  - Inject `contracts.DBService` via repository struct or retrieve via `ctx.DB()`.
- **Strict Package Separation**:
  - `backend/core/`: Micro-kernel, context, container, fiber, events, scoped extpoints.
  - `backend/core/contracts/`: Public interfaces and shared DTOs/events.
  - `backend/plugins/infra/`: Infrastructure implementations providing contracts services.
  - `backend/plugins/drivers/`: Runtime drivers (HTTP, Asynq Worker, Cron).
  - `backend/plugins/domain/`: Domain business logic and single-owner tables.
  - `backend/pkg/`: Stateless utilities and algorithm libraries.

---

## 3. Verification Plan

1. **Unit Tests for Core**:
   - `core/fiber_test.go`: Test fiber state transitions, out-of-order registration confluence, and dynamic unloading.
   - `core/context_test.go` & `core/extpoints/`: Test automatic scoped disposer tracking for routes, tasks, schedules, and event listeners.
2. **Refactoring Verification for Domain Plugins**:
   - Run `go test ./backend/...` across all domain and infra packages.
   - Run `make code-check` and verify zero lint regressions.
