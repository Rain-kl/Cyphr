# Cordis Architecture Alignment & Refactoring Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement Cordis spatiotemporal composability (scoped revertible effects and reactive fiber lifecycle state machine) in `backend/core`, and eliminate cross-plugin direct imports in domain repositories.

**Architecture:** 
1. Build scoped extension proxies on `core.Context` that automatically attach unregister callbacks to `ctx.OnDispose` in LIFO order upon registration.
2. Introduce `core/fiber.go` implementing the Fiber state machine (`PENDING -> LOADING -> ACTIVE -> UNLOADING -> DISPOSED`) with a reactive reconciler in `App`/`Container` ensuring dependency confluence.
3. Clean up defensive boundaries in `backend/plugins/domain/user` by removing direct `database.DB(ctx)` imports in favor of `contracts.DBService`.

**Tech Stack:** Go 1.24+, GORM, Gin, Asynq, Cordis micro-kernel paradigm.

## Global Constraints

- Strictly preserve `backend/pkg/util/` purity (no Gin/GORM imports).
- Zero physically hardcoded temp directories in tests (use `t.TempDir()`).
- All Go error returns and logging must adhere to project standards.
- Follow Conventional Commits (`feat(core): ...`, `refactor(user): ...`).

---

### Task 1: Scoped Revertible Effects for Core Extpoints

**Files:**
- Create: `backend/core/scoped_extpoints.go`
- Modify: `backend/core/context.go`
- Modify: `backend/core/extpoints/task.go`
- Modify: `backend/core/extpoints/schedule.go`
- Modify: `backend/core/extpoints/setting.go`
- Test: `backend/core/context_test.go`

**Interfaces:**
- Consumes: `core.Context`, `extpoints.RouterExtension`, `extpoints.TaskExtension`, `extpoints.ScheduleExtension`, `extpoints.SettingExtension`, `core.EventBus`
- Produces: Scoped extension methods on `Context` that automatically register LIFO disposers when routes, tasks, schedules, settings, and events are registered.

- [ ] **Step 1: Write the failing test for scoped extpoints automatic teardown**

In `backend/core/context_test.go`, add test cases verifying that registering routes, tasks, schedules, settings, and event listeners on a child context automatically registers unregister callbacks, and calling `childCtx.Dispose()` completely rolls them back:

```go
func TestContext_ScopedExtpoints_RevertibleEffects(t *testing.T) {
	root := NewContext(context.Background())
	child := root.Fork()

	// Register route, task, schedule, setting, event on child
	rd := child.Router().GET("/test-route", func() {})
	assert.Equal(t, 1, len(root.Router().Routes()))

	child.Events().On("test:event", func() {})
	assert.Equal(t, 1, root.Events().Listeners("test:event"))

	// Dispose child
	err := child.Dispose()
	assert.NoError(t, err)

	// All child effects should be revoked
	assert.Equal(t, 0, len(root.Router().Routes()))
	assert.Equal(t, 0, root.Events().Listeners("test:event"))
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./backend/core -run TestContext_ScopedExtpoints_RevertibleEffects`
Expected: FAIL (because current `Router().GET()` does not bind unregistration to `child.OnDispose`).

- [ ] **Step 3: Implement Scoped Extpoints and Context bindings**

1. In `backend/core/extpoints/task.go`, ensure `Unregister(taskType string) bool` exists.
2. In `backend/core/extpoints/schedule.go`, ensure `Unregister(name string) bool` exists.
3. In `backend/core/extpoints/setting.go`, ensure `Unregister(key string) bool` exists.
4. In `backend/core/scoped_extpoints.go` (or `context.go`), create scoped wrappers for `RouterExtension`, `TaskExtension`, `ScheduleExtension`, `SettingExtension` and `EventBus` that tie registrations to `ctx.OnDispose`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -v ./backend/core -run TestContext_ScopedExtpoints_RevertibleEffects`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/core/
git commit -m "feat(core): implement scoped revertible effects for context extpoints"
```

---

### Task 2: Plugin Fiber State Machine and Reactive Coeffects (Confluence)

**Files:**
- Create: `backend/core/fiber.go`
- Create: `backend/core/fiber_test.go`
- Modify: `backend/core/app.go`
- Modify: `backend/core/types.go`
- Modify: `backend/core/container.go`

**Interfaces:**
- Consumes: `core.Plugin`, `core.Context`, `core.Container`
- Produces: `core.DependentPlugin`, `core.Fiber`, `core.FiberState`, `App.Reconcile()`

- [ ] **Step 1: Write the failing test for Fiber state machine and out-of-order registration confluence**

In `backend/core/fiber_test.go`:

```go
func TestFiber_ConfluenceAndReactiveActivation(t *testing.T) {
	app := NewApp()

	// Plugin B depends on contracts.DBService, but is registered BEFORE DatabasePlugin (Plugin A)
	pluginB := &mockDependentPlugin{
		name: "plugin-b",
		deps: []reflect.Type{reflect.TypeFor[contracts.DBService]()},
	}
	pluginA := &mockDBPlugin{name: "database"}

	app.Use(pluginB, pluginA)

	err := app.Start(context.Background())
	assert.NoError(t, err)

	// Verify both plugins reached FiberActive state and B executed Apply successfully after A provided DBService
	assert.True(t, pluginB.applied)
	assert.True(t, pluginA.applied)

	_ = app.Stop()
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./backend/core -run TestFiber_ConfluenceAndReactiveActivation`
Expected: FAIL (because current `app.ApplyPlugins()` applies in static slice order without dependency reconciliation).

- [ ] **Step 3: Implement Fiber State Machine and Reconciler**

1. In `backend/core/types.go`, declare:
```go
type DependentPlugin interface {
	Plugin
	Inject() []reflect.Type
}
```
2. In `backend/core/fiber.go`, implement `Fiber` with states (`FiberPending`, `FiberLoading`, `FiberActive`, `FiberUnloading`, `FiberDisposed`), child scoped context, and state transition methods.
3. In `backend/core/app.go`, integrate Fibers into `App` and implement iterative dependency reconciliation during `ApplyPlugins` and on dynamic `Provide`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -v ./backend/core`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add backend/core/
git commit -m "feat(core): implement plugin fiber state machine and reactive dependency reconciler"
```

---

### Task 3: Domain Plugin Isolation & Boundary Enforcement

**Files:**
- Modify: `backend/plugins/domain/user/repository.go`
- Modify: `backend/plugins/domain/user/service.go`
- Modify: `backend/plugins/domain/user/handlers.go`
- Modify: `backend/plugins/domain/user/plugin.go`
- Test: `backend/plugins/domain/user/plugin_test.go`

**Interfaces:**
- Consumes: `contracts.DBService` via `core.Inject` / `ctx.DB()`
- Produces: Decoupled User repository without direct `Wavelet/plugins/infra/database` imports.

- [ ] **Step 1: Write/update test verifying User repository works with injected DBService**

In `backend/plugins/domain/user/plugin_test.go`, test user CRUD operations resolving `contracts.DBService` through Context.

- [ ] **Step 2: Run test to verify current state**

Run: `go test -v ./backend/plugins/domain/user/...`

- [ ] **Step 3: Refactor user repository to eliminate direct `plugins/infra/database` imports**

In `backend/plugins/domain/user/repository.go`:
- Remove `import "Wavelet/plugins/infra/database"`.
- Obtain `*gorm.DB` via `ctx` (e.g. from context using `contracts.DBService` or context value).

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -v ./backend/plugins/domain/user/...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/plugins/domain/user/
git commit -m "refactor(user): decouple repository from direct database infra import"
```

---

### Task 4: Full Suite Verification & Quality Gate

**Files:**
- Entire repository

- [ ] **Step 1: Run all backend tests**

Run: `cd backend && go test -v ./...`
Expected: ALL PASS

- [ ] **Step 2: Run code-check and format**

Run: `make code-check && make format`
Expected: 0 lint errors, clean formatting.

- [ ] **Step 3: Commit any formatting or lint fixes**

```bash
git commit -am "chore: format and verify code quality"
```
