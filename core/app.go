// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	defaultShutdownTimeout = 10 * time.Second
)

// AppOption configures an App instance during construction.
type AppOption func(*App)

// WithContext sets a custom root Context for the App.
func WithContext(ctx *Context) AppOption {
	return func(a *App) {
		if ctx != nil {
			a.ctx = ctx
		}
	}
}

// WithProfile sets the runtime profile for the App.
func WithProfile(profile Profile) AppOption {
	return func(a *App) {
		a.profile = normalizeProfile(profile)
	}
}

// WithPlugins registers initial plugins for the App.
func WithPlugins(plugins ...Plugin) AppOption {
	return func(a *App) {
		a.Use(plugins...)
	}
}

// WithMigrationEngine sets the database migration engine for the App.
func WithMigrationEngine(engine MigrationEngine) AppOption {
	return func(a *App) {
		a.migrationEngine = engine
	}
}

// WithMigrationRunner sets the migration runner function for the App.
func WithMigrationRunner(runner MigrationRunner) AppOption {
	return func(a *App) {
		a.migrationEngine = runner
	}
}

// WithShutdownTimeout sets the fallback timeout for graceful application shutdown.
func WithShutdownTimeout(timeout time.Duration) AppOption {
	return func(a *App) {
		if timeout > 0 {
			a.shutdownTimeout = timeout
		}
	}
}

// App is the unified assembly entrypoint and runtime aspect dispatcher of the Cordis micro-kernel.
// It manages plugin collection, dependency mounting, migration execution, profile-based driver startup,
// and graceful signal-driven LIFO shutdown.
type App struct {
	mu              sync.RWMutex
	ctx             *Context
	profile         Profile
	plugins         []Plugin
	pluginMap       map[string]Plugin
	applied         bool
	running         bool
	startedDrivers  []Driver
	migrationEngine MigrationEngine
	shutdownTimeout time.Duration
}

// NewApp creates a new Cordis application instance with default options.
func NewApp(opts ...AppOption) *App {
	app := &App{
		ctx:             NewContext(context.Background()),
		profile:         ProfileAll,
		pluginMap:       make(map[string]Plugin),
		shutdownTimeout: defaultShutdownTimeout,
	}

	for _, opt := range opts {
		if opt != nil {
			opt(app)
		}
	}

	return app
}

// Context returns the root micro-kernel Context of the application.
func (a *App) Context() *Context {
	return a.ctx
}

// Profile returns the current runtime profile of the application.
func (a *App) Profile() Profile {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.profile
}

// WithProfile sets the application runtime profile and returns the App for fluent chaining.
func (a *App) WithProfile(profile Profile) *App {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.profile = normalizeProfile(profile)
	return a
}

// SetProfile sets the application runtime profile.
func (a *App) SetProfile(profile Profile) *App {
	return a.WithProfile(profile)
}

// Use registers one or more plugins into the application in registration order.
// Duplicate plugins (by Name) update existing registrations in-place to preserve order.
func (a *App) Use(plugins ...Plugin) *App {
	a.mu.Lock()
	defer a.mu.Unlock()

	for _, p := range plugins {
		if p == nil {
			continue
		}
		name := p.Name()
		if name == "" {
			continue
		}

		if _, exists := a.pluginMap[name]; exists {
			for i, existing := range a.plugins {
				if existing.Name() == name {
					a.plugins[i] = p
					break
				}
			}
		} else {
			a.plugins = append(a.plugins, p)
		}
		a.pluginMap[name] = p
	}

	return a
}

// Plugins returns a copy of all registered plugins in registration order.
func (a *App) Plugins() []Plugin {
	a.mu.RLock()
	defer a.mu.RUnlock()

	res := make([]Plugin, len(a.plugins))
	copy(res, a.plugins)
	return res
}

// Plugin retrieves a registered plugin by its unique name.
func (a *App) Plugin(name string) (Plugin, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	p, ok := a.pluginMap[name]
	return p, ok
}

// SetMigrationEngine sets the migration engine for the application.
func (a *App) SetMigrationEngine(engine MigrationEngine) *App {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.migrationEngine = engine
	return a
}

// SetMigrationRunner sets the migration runner function for the application.
func (a *App) SetMigrationRunner(runner MigrationRunner) *App {
	return a.SetMigrationEngine(runner)
}

// ApplyPlugins applies all registered plugins on the application Context.
// It is idempotent and only applies plugins once per App instance.
func (a *App) ApplyPlugins() error {
	a.mu.Lock()
	if a.applied {
		a.mu.Unlock()
		return nil
	}
	a.applied = true
	plugins := make([]Plugin, len(a.plugins))
	copy(plugins, a.plugins)
	a.mu.Unlock()

	for _, p := range plugins {
		if err := p.Apply(a.ctx); err != nil {
			return fmt.Errorf("core: apply plugin %q failed: %w", p.Name(), err)
		}
	}
	return nil
}

// RunMigrations dispatches migration execution across all registered plugin migration entries.
func (a *App) RunMigrations(ctx context.Context) error {
	entries := a.ctx.Migrations().Entries()
	if len(entries) == 0 {
		return nil
	}

	a.mu.RLock()
	engine := a.migrationEngine
	a.mu.RUnlock()

	if engine == nil {
		// Attempt to resolve from IoC container
		if resolved, err := Inject[MigrationEngine](a.ctx); err == nil && resolved != nil {
			engine = resolved
		}
	}

	if engine == nil {
		return nil
	}

	if err := engine.Migrate(ctx, entries); err != nil {
		return fmt.Errorf("core: migration failed: %w", err)
	}

	return nil
}

// Start executes the application boot pipeline:
// 1. Applies all registered plugins to populate services, routes, tasks, and drivers.
// 2. Dispatches database migrations via MigrationEngine.
// 3. Filters and starts drivers matching the active Profile.
// 4. Emits "app:ready" on the EventBus.
//
//nolint:contextcheck
func (a *App) Start(ctx ...context.Context) error {
	a.mu.Lock()
	if a.running {
		a.mu.Unlock()
		return ErrAppRunning
	}
	a.running = true
	a.mu.Unlock()

	var baseCtx context.Context
	switch {
	case len(ctx) > 0 && ctx[0] != nil:
		baseCtx = ctx[0]
	case a.ctx != nil:
		baseCtx = a.ctx.GoContext()
	default:
		baseCtx = context.Background()
	}

	// 1. Apply plugins
	if err := a.ApplyPlugins(); err != nil {
		a.mu.Lock()
		a.running = false
		a.mu.Unlock()
		return err
	}

	// 2. Run migrations
	if err := a.RunMigrations(baseCtx); err != nil {
		a.mu.Lock()
		a.running = false
		a.mu.Unlock()
		return err
	}

	// 3. Filter drivers matching active profile
	a.mu.RLock()
	prof := a.profile
	a.mu.RUnlock()

	allDrivers := a.ctx.Drivers()
	var driversToStart []Driver
	for _, d := range allDrivers {
		if matchesProfile(prof, d.Type()) {
			driversToStart = append(driversToStart, d)
		}
	}

	// 4. Start matching drivers
	for _, d := range driversToStart {
		if err := d.Start(baseCtx); err != nil {
			// Rollback already started drivers in reverse order
			a.mu.Lock()
			started := a.startedDrivers
			a.startedDrivers = nil
			a.running = false
			a.mu.Unlock()

			for i := len(started) - 1; i >= 0; i-- {
				_ = started[i].Stop(context.Background())
			}

			return fmt.Errorf("core: start driver %s failed: %w", d.Type(), err)
		}

		a.mu.Lock()
		a.startedDrivers = append(a.startedDrivers, d)
		a.mu.Unlock()
	}

	// 5. Emit app:ready event
	_ = a.ctx.Events().Emit(baseCtx, "app:ready", a)

	return nil
}

// Stop gracefully shuts down the application:
// 1. Emits "app:stopping" on the EventBus.
// 2. Stops all started drivers in LIFO (reverse) order.
// 3. Disposes the Context (running registered OnDispose callbacks in LIFO order).
// 4. Emits "app:stopped" on the EventBus.
//
//nolint:contextcheck
func (a *App) Stop(ctx ...context.Context) error {
	a.mu.Lock()
	if !a.running {
		a.mu.Unlock()
		return nil
	}
	a.running = false
	started := a.startedDrivers
	a.startedDrivers = nil
	timeout := a.shutdownTimeout
	a.mu.Unlock()

	var shutdownCtx context.Context
	if len(ctx) > 0 && ctx[0] != nil {
		shutdownCtx = ctx[0]
	} else {
		var cancel context.CancelFunc
		shutdownCtx, cancel = context.WithTimeout(context.Background(), timeout)
		defer cancel()
	}

	_ = a.ctx.Events().Emit(shutdownCtx, "app:stopping", a)

	var errs []error

	// 1. Stop drivers in reverse order
	for i := len(started) - 1; i >= 0; i-- {
		d := started[i]
		if err := d.Stop(shutdownCtx); err != nil {
			errs = append(errs, fmt.Errorf("core: stop driver %s failed: %w", d.Type(), err))
		}
	}

	// 2. Dispose context
	if a.ctx != nil && !a.ctx.IsDisposed() {
		if err := a.ctx.Dispose(); err != nil {
			errs = append(errs, fmt.Errorf("core: dispose context failed: %w", err))
		}
	}

	_ = a.ctx.Events().Emit(shutdownCtx, "app:stopped", a)

	return errors.Join(errs...)
}

// Run starts the application and blocks until an OS signal (SIGINT, SIGTERM) or context cancellation is received,
// then executes graceful shutdown.
//
//nolint:contextcheck
func (a *App) Run(ctx ...context.Context) error {
	var parent context.Context
	switch {
	case len(ctx) > 0 && ctx[0] != nil:
		parent = ctx[0]
	case a.ctx != nil:
		parent = a.ctx.GoContext()
	default:
		parent = context.Background()
	}

	sigCtx, stopSignals := signal.NotifyContext(parent, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	defer stopSignals()

	if err := a.Start(sigCtx); err != nil {
		return err
	}

	// Wait for OS signal or context cancellation
	<-sigCtx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), a.shutdownTimeout)
	defer cancel()

	return a.Stop(shutdownCtx)
}

// IsRunning returns whether the application is currently running.
func (a *App) IsRunning() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.running
}

// StartedDrivers returns a copy of currently running drivers.
func (a *App) StartedDrivers() []Driver {
	a.mu.RLock()
	defer a.mu.RUnlock()

	res := make([]Driver, len(a.startedDrivers))
	copy(res, a.startedDrivers)
	return res
}

// ExecuteCLI parses CLI arguments to configure the profile and runs the application.
func (a *App) ExecuteCLI(args ...string) error {
	var ctx context.Context
	if a.ctx != nil {
		ctx = a.ctx.GoContext()
	} else {
		ctx = context.Background()
	}
	return a.ExecuteCLIWithContext(ctx, args...)
}

// ExecuteCLIWithContext parses CLI arguments, configures the profile, and runs the application with the given context.
//
//nolint:contextcheck
func (a *App) ExecuteCLIWithContext(ctx context.Context, args ...string) error {
	cliArgs := args
	if len(cliArgs) == 0 {
		cliArgs = os.Args[1:]
	}

	profile := ProfileAll
	if len(cliArgs) > 0 {
		first := strings.TrimSpace(cliArgs[0])
		switch {
		case strings.HasPrefix(first, "--profile="):
			profile = Profile(strings.TrimPrefix(first, "--profile="))
		case strings.HasPrefix(first, "-p="):
			profile = Profile(strings.TrimPrefix(first, "-p="))
		case !strings.HasPrefix(first, "-"):
			profile = Profile(first)
		}
	}

	a.WithProfile(profile)
	return a.Run(ctx)
}

func matchesProfile(profile Profile, dt DriverType) bool {
	norm := normalizeProfile(profile)
	switch norm {
	case ProfileAll, "":
		return true
	case ProfileAPI:
		return dt == DriverTypeHTTP
	case ProfileWorker:
		return dt == DriverTypeWorker
	case ProfileSchedule:
		return dt == DriverTypeScheduler
	default:
		return string(norm) == string(dt)
	}
}

func normalizeProfile(p Profile) Profile {
	switch strings.ToLower(strings.TrimSpace(string(p))) {
	case "api", "http":
		return ProfileAPI
	case "worker":
		return ProfileWorker
	case "schedule", "scheduler", "cron":
		return ProfileSchedule
	case "all", "fused", "full", "":
		return ProfileAll
	default:
		return p
	}
}
