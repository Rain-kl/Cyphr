# Cordis Architecture Refactor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Refactor Wavelet backend to strictly conform to Cordis meta-framework principles: full revertible effects, space composability via per-plugin scoped context fork, 4-semantic typed event bus, elimination of package-level infra globals, and strict single-owner principle across domain plugins.

**Architecture:** 
1. Microkernel core (`core/`): Implement `Waterfall`, `Parallel`, `Serial` event dispatch, per-plugin Scoped Context (`ctx.Fork()`), revertible extension points (`extpoints`), and `ctx.DB()` / `ctx.Cache()` contract helpers.
2. Contracts (`core/contracts/`): Expand `UserService` & `AuthService` with administration and token revocation interfaces; define typed domain events.
3. Domain Plugins (`plugins/domain/`): Implement user/auth contract additions; refactor `admin` plugin to completely remove cross-plugin internal package imports and direct SQL operations on other plugins' tables.

**Tech Stack:** Go 1.23+, GORM, Gin, Goose, Cordis Paradigm.

## Global Constraints

- No direct cross-package imports between plugins (`plugins/domain/A` must NEVER import `plugins/domain/B` or `plugins/drivers/*`).
- Single Owner Principle: Every database table is owned and operated exclusively by its owner plugin.
- Microkernel purity: `core/` and `core/contracts/` must never import `gin`, `gorm`, `asynq`.
- Tests must pass with `-race` enabled; temporary directories must use `t.TempDir()`.
- Quality gates: `make code-check`, `make format`, `make swagger`.

---

### Task 1: Core EventBus 4 Dispatch Semantics

**Files:**
- Modify: `backend/core/events.go`
- Test: `backend/core/events_test.go`

**Interfaces:**
- Produces:
  - `(b *EventBus) Emit(ctx context.Context, topic string, payload any) error`
  - `(b *EventBus) Waterfall(ctx context.Context, topic string, initialPayload any) (any, error)`
  - `(b *EventBus) Parallel(ctx context.Context, topic string, payload any) error`
  - `(b *EventBus) Serial(ctx context.Context, topic string, payload any) error`

- [ ] **Step 1: Write tests for Waterfall, Parallel, and Serial dispatch semantics**
- [ ] **Step 2: Run tests to verify they fail**
- [ ] **Step 3: Implement Waterfall, Parallel, Serial methods on EventBus**
- [ ] **Step 4: Run tests to verify they pass**
- [ ] **Step 5: Commit**

---

### Task 2: Core Scoped Context, Revertible ExtPoints & Contract Helpers

**Files:**
- Modify: `backend/core/context.go`
- Modify: `backend/core/app.go`
- Modify: `backend/core/extpoints/router.go`
- Modify: `backend/core/extpoints/task.go`
- Modify: `backend/core/extpoints/schedule.go`
- Modify: `backend/core/extpoints/setting.go`
- Modify: `backend/core/extpoints/migration.go`
- Test: `backend/core/context_test.go`
- Test: `backend/core/app_test.go`
- Test: `backend/core/extpoints/extpoints_test.go`

**Interfaces:**
- Produces:
  - `(c *Context) DB() contracts.DBService`
  - `(c *Context) Cache() contracts.CacheService`
  - `(r *RouterRegistry) Unregister(id uint64) bool`
  - `RouterExtension.Handle(...) Disposer` / `RouteDefinition` with disposer tracking
  - `App.ApplyPlugins()` forks scoped context per plugin: `p.Apply(a.ctx.Fork())`

- [ ] **Step 1: Write tests for Scoped Context Fork, LIFO Disposer, and Router Unregister**
- [ ] **Step 2: Run tests to verify failure**
- [ ] **Step 3: Implement Scoped Fork, Disposers, and Context DB/Cache helpers**
- [ ] **Step 4: Update App.ApplyPlugins to fork a context for each plugin**
- [ ] **Step 5: Run tests and verify all core tests pass**
- [ ] **Step 6: Commit**

---

### Task 3: Expand Service Contracts & Domain Events

**Files:**
- Modify: `backend/core/contracts/user.go`
- Modify: `backend/core/contracts/auth.go`
- Modify: `backend/core/contracts/events.go`

**Interfaces:**
- Produces:
  - `AdminListUsersRequest`, `AdminCreateUserRequest`, `AdminUpdateUserRequest`
  - `UserService` admin methods (`AdminListUsers`, `AdminGetUser`, `AdminCreateUser`, `AdminUpdateUser`, `AdminUpdateUserStatus`, `AdminDeleteUser`)
  - `AuthService` token management methods (`RevokeToken`, `RevokeUserTokens`, `InvalidateCachedUser`, `InvalidateCachedToken`)
  - Standard event definitions (`EventUserUpdated`, `EventUserDeleted`, `EventUserStatusChanged`, `EventTokenRevoked`)

- [ ] **Step 1: Declare extended contracts and DTO types in core/contracts/**
- [ ] **Step 2: Declare typed event constants and structs in core/contracts/events.go**
- [ ] **Step 3: Verify core and core/contracts compile cleanly**
- [ ] **Step 4: Commit**

---

### Task 4: Implement Expanded Contracts in User & Auth Domain Plugins

**Files:**
- Modify: `backend/plugins/domain/user/service.go`
- Modify: `backend/plugins/domain/user/plugin.go`
- Modify: `backend/plugins/domain/user/user_test.go`
- Modify: `backend/plugins/domain/auth/service.go`
- Modify: `backend/plugins/domain/auth/plugin.go`
- Modify: `backend/plugins/domain/auth/plugin_test.go`

**Interfaces:**
- Implements: `contracts.UserService` full methods in `user` plugin.
- Implements: `contracts.AuthService` full methods in `auth` plugin.
- Subscribes: `auth` plugin subscribes to `EventUserStatusChanged` / `EventUserDeleted` to invalidate cache and revoke tokens.

- [ ] **Step 1: Write unit tests for new UserService admin methods and AuthService revocation methods**
- [ ] **Step 2: Run tests to verify failure**
- [ ] **Step 3: Implement the methods in user and auth domain packages**
- [ ] **Step 4: Run user and auth plugin tests and verify they pass**
- [ ] **Step 5: Commit**

---

### Task 5: Refactor Admin Plugin (Eliminate Cross-Plugin Direct Imports & Table Ownership Violations)

**Files:**
- Modify: `backend/plugins/domain/admin/handlers_user.go`
- Modify: `backend/plugins/domain/admin/handlers_auth_source.go`
- Modify: `backend/plugins/domain/admin/handlers_config.go`
- Modify: `backend/plugins/domain/admin/handlers_logs.go`
- Modify: `backend/plugins/domain/admin/handlers_status.go`
- Modify: `backend/plugins/domain/admin/handlers_tasks.go`
- Modify: `backend/plugins/domain/admin/repository.go`
- Modify: `backend/plugins/domain/admin/system_config_cache.go`
- Modify: `backend/plugins/domain/admin/plugin.go`
- Modify: `backend/plugins/domain/admin/plugin_test.go`

**Interfaces:**
- Consumes: `contracts.UserService`, `contracts.AuthService`, `contracts.DBService`, `contracts.CacheService`, `ctx.DB()`, `ctx.Cache()`
- Zero imports of `plugins/domain/auth`, `plugins/domain/risk_control`, `plugins/domain/cap`, `plugins/drivers/*`, `plugins/infra/database`

- [ ] **Step 1: Write integration tests for Admin handlers using mocked/injected contracts**
- [ ] **Step 2: Refactor admin handlers to delegate user/auth operations to contracts**
- [ ] **Step 3: Remove all cross-plugin direct package imports and illegal SQL DML**
- [ ] **Step 4: Run admin plugin tests to verify passing**
- [ ] **Step 5: Commit**

---

### Task 6: Clean up Domain & Infra Plugins Database / Cache Injections

**Files:**
- Modify: `backend/plugins/domain/cap/...`
- Modify: `backend/plugins/domain/message_gateway/...`
- Modify: `backend/plugins/domain/risk_control/...`
- Modify: `backend/plugins/domain/upload/...`
- Modify: `backend/plugins/domain/system/...`

- [ ] **Step 1: Audit and replace direct `database.DB(ctx)` calls with `ctx.DB()` / injected `contracts.DBService`**
- [ ] **Step 2: Audit and replace direct `cache.Client()` calls with `ctx.Cache()` / injected `contracts.CacheService`**
- [ ] **Step 3: Run domain plugins test suite**
- [ ] **Step 4: Commit**

---

### Task 7: Full Verification & Quality Gates

**Files:**
- All backend files

- [ ] **Step 1: Run full test suite with race detector: `go test -v -race ./backend/...`**
- [ ] **Step 2: Run `make code-check`**
- [ ] **Step 3: Run `make format`**
- [ ] **Step 4: Run `make swagger`**
- [ ] **Step 5: Final commit**
