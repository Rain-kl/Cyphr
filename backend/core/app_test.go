// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package core_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Rain-kl/Wavelet/core"
	"github.com/Rain-kl/Wavelet/core/extpoints"
)

// appMockDriver is a test driver tracking its start/stop lifecycle.
type appMockDriver struct {
	mu          sync.Mutex
	driverType  core.DriverType
	startCalled bool
	stopCalled  bool
	startErr    error
	stopErr     error
}

func newAppMockDriver(dt core.DriverType) *appMockDriver {
	return &appMockDriver{driverType: dt}
}

func (m *appMockDriver) Type() core.DriverType {
	return m.driverType
}

func (m *appMockDriver) Start(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.startErr != nil {
		return m.startErr
	}
	m.startCalled = true
	return nil
}

func (m *appMockDriver) Stop(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.stopErr != nil {
		return m.stopErr
	}
	m.stopCalled = true
	return nil
}

func (m *appMockDriver) isStarted() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.startCalled
}

func (m *appMockDriver) isStopped() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stopCalled
}

// appMockPlugin is a test plugin.
type appMockPlugin struct {
	name    string
	applyFn func(ctx *core.Context) error
}

func (p *appMockPlugin) Name() string {
	return p.name
}

func (p *appMockPlugin) Apply(ctx *core.Context) error {
	if p.applyFn != nil {
		return p.applyFn(ctx)
	}
	return nil
}

func TestAppNewAndConfiguration(t *testing.T) {
	customCtx := core.NewContext(context.Background())
	p1 := &appMockPlugin{name: "plugin1"}
	p2 := &appMockPlugin{name: "plugin2"}

	app := core.NewApp(
		core.WithContext(customCtx),
		core.WithProfile(core.ProfileAPI),
		core.WithPlugins(p1, p2),
		core.WithShutdownTimeout(5*time.Second),
	)

	assert.Equal(t, customCtx, app.Context())
	assert.Equal(t, core.ProfileAPI, app.Profile())
	assert.Len(t, app.Plugins(), 2)

	retrieved, ok := app.Plugin("plugin1")
	assert.True(t, ok)
	assert.Equal(t, p1, retrieved)

	_, ok = app.Plugin("non_existent")
	assert.False(t, ok)

	// Update existing plugin in-place
	p1Updated := &appMockPlugin{name: "plugin1"}
	app.Use(p1Updated, nil)
	assert.Len(t, app.Plugins(), 2)
	retrieved, ok = app.Plugin("plugin1")
	assert.True(t, ok)
	assert.Equal(t, p1Updated, retrieved)

	// Test SetProfile
	app.SetProfile(core.ProfileWorker)
	assert.Equal(t, core.ProfileWorker, app.Profile())
}

func TestAppProfileDispatch(t *testing.T) {
	tests := []struct {
		name           string
		profile        core.Profile
		expectedHTTP   bool
		expectedWorker bool
		expectedCron   bool
		expectedCustom bool
	}{
		{
			name:           "ProfileAPI only starts HTTP driver",
			profile:        core.ProfileAPI,
			expectedHTTP:   true,
			expectedWorker: false,
			expectedCron:   false,
			expectedCustom: false,
		},
		{
			name:           "ProfileWorker only starts Worker driver",
			profile:        core.ProfileWorker,
			expectedHTTP:   false,
			expectedWorker: true,
			expectedCron:   false,
			expectedCustom: false,
		},
		{
			name:           "ProfileSchedule only starts Schedule driver",
			profile:        core.ProfileSchedule,
			expectedHTTP:   false,
			expectedWorker: false,
			expectedCron:   true,
			expectedCustom: false,
		},
		{
			name:           "Profile 'scheduler' alias starts Schedule driver",
			profile:        core.Profile("scheduler"),
			expectedHTTP:   false,
			expectedWorker: false,
			expectedCron:   true,
			expectedCustom: false,
		},
		{
			name:           "ProfileAll starts all drivers",
			profile:        core.ProfileAll,
			expectedHTTP:   true,
			expectedWorker: true,
			expectedCron:   true,
			expectedCustom: true,
		},
		{
			name:           "Custom profile starts custom driver",
			profile:        core.Profile("custom_rpc"),
			expectedHTTP:   false,
			expectedWorker: false,
			expectedCron:   false,
			expectedCustom: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpD := newAppMockDriver(core.DriverTypeHTTP)
			workerD := newAppMockDriver(core.DriverTypeWorker)
			cronD := newAppMockDriver(core.DriverTypeScheduler)
			customD := newAppMockDriver(core.DriverType("custom_rpc"))

			p := &appMockPlugin{
				name: "drivers_plugin",
				applyFn: func(ctx *core.Context) error {
					_ = ctx.RegisterDriver(httpD)
					_ = ctx.RegisterDriver(workerD)
					_ = ctx.RegisterDriver(cronD)
					_ = ctx.RegisterDriver(customD)
					return nil
				},
			}

			app := core.NewApp(
				core.WithProfile(tt.profile),
				core.WithPlugins(p),
			)

			err := app.Start(context.Background())
			require.NoError(t, err)

			assert.Equal(t, tt.expectedHTTP, httpD.isStarted(), "HTTP driver start mismatch")
			assert.Equal(t, tt.expectedWorker, workerD.isStarted(), "Worker driver start mismatch")
			assert.Equal(t, tt.expectedCron, cronD.isStarted(), "Cron driver start mismatch")
			assert.Equal(t, tt.expectedCustom, customD.isStarted(), "Custom driver start mismatch")

			err = app.Stop(context.Background())
			require.NoError(t, err)
		})
	}
}

func TestAppLifecycleStartStop(t *testing.T) {
	var stopOrder []string
	var stopOrderMu sync.Mutex

	httpD := newAppMockDriver(core.DriverTypeHTTP)
	workerD := newAppMockDriver(core.DriverTypeWorker)

	httpD.stopErr = nil
	workerD.stopErr = nil

	// Wrap stop to record order
	origHttpStop := httpD.Stop
	_ = origHttpStop

	p := &appMockPlugin{
		name: "test_plugin",
		applyFn: func(ctx *core.Context) error {
			_ = ctx.RegisterDriver(httpD)
			_ = ctx.RegisterDriver(workerD)

			ctx.OnDispose(func() error {
				stopOrderMu.Lock()
				stopOrder = append(stopOrder, "ctx_disposer")
				stopOrderMu.Unlock()
				return nil
			})
			return nil
		},
	}

	app := core.NewApp(
		core.WithProfile(core.ProfileAll),
		core.WithPlugins(p),
	)

	var readyReceived, stoppingReceived, stoppedReceived bool
	app.Context().Events().On("app:ready", func() {
		readyReceived = true
	})
	app.Context().Events().On("app:stopping", func() {
		stoppingReceived = true
	})
	app.Context().Events().On("app:stopped", func() {
		stoppedReceived = true
	})

	err := app.Start(context.Background())
	require.NoError(t, err)
	assert.True(t, app.IsRunning())
	assert.Len(t, app.StartedDrivers(), 2)
	assert.True(t, readyReceived)

	err = app.Stop(context.Background())
	require.NoError(t, err)
	assert.False(t, app.IsRunning())
	assert.Empty(t, app.StartedDrivers())
	assert.True(t, stoppingReceived)
	assert.True(t, stoppedReceived)

	assert.True(t, httpD.isStopped())
	assert.True(t, workerD.isStopped())
	assert.True(t, app.Context().IsDisposed())

	stopOrderMu.Lock()
	assert.Contains(t, stopOrder, "ctx_disposer")
	stopOrderMu.Unlock()
}

func TestAppStartDriverFailureRollback(t *testing.T) {
	driver1 := newAppMockDriver(core.DriverTypeHTTP)
	driver2 := newAppMockDriver(core.DriverTypeWorker)
	driver2.startErr = errors.New("worker listen port conflict")
	driver3 := newAppMockDriver(core.DriverTypeScheduler)

	p := &appMockPlugin{
		name: "fail_driver_plugin",
		applyFn: func(ctx *core.Context) error {
			_ = ctx.RegisterDriver(driver1)
			_ = ctx.RegisterDriver(driver2)
			_ = ctx.RegisterDriver(driver3)
			return nil
		},
	}

	app := core.NewApp(
		core.WithProfile(core.ProfileAll),
		core.WithPlugins(p),
	)

	err := app.Start(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "worker listen port conflict")
	assert.False(t, app.IsRunning())

	// Driver 1 was started then rolled back (stopped)
	assert.True(t, driver1.isStarted())
	assert.True(t, driver1.isStopped())

	// Driver 3 was never started
	assert.False(t, driver3.isStarted())
}

func TestAppMigrationEngineExecution(t *testing.T) {
	var migratedEntries []extpoints.MigrationEntry
	runner := core.MigrationRunner(func(ctx *core.Context, entries []extpoints.MigrationEntry) error {
		migratedEntries = entries
		return nil
	})

	sqlFS := fstest.MapFS{
		"migrations/001_init.sql": &fstest.MapFile{Data: []byte("CREATE TABLE users(id int);")},
	}

	p := &appMockPlugin{
		name: "auth",
		applyFn: func(ctx *core.Context) error {
			ctx.Migrations().Register("auth", sqlFS)
			return nil
		},
	}

	app := core.NewApp(
		core.WithProfile(core.ProfileAll),
		core.WithPlugins(p),
		core.WithMigrationRunner(runner),
	)

	err := app.Start(context.Background())
	require.NoError(t, err)
	defer func() { _ = app.Stop(context.Background()) }()

	require.Len(t, migratedEntries, 1)
	assert.Equal(t, "auth", migratedEntries[0].PluginID)
}

func TestAppMigrationEngineFromIoCContainer(t *testing.T) {
	var executed bool
	runner := core.MigrationRunner(func(ctx *core.Context, entries []extpoints.MigrationEntry) error {
		executed = true
		return nil
	})

	sqlFS := fstest.MapFS{
		"migrations/001_init.sql": &fstest.MapFile{Data: []byte("CREATE TABLE logs(id int);")},
	}

	p := &appMockPlugin{
		name: "logstore",
		applyFn: func(ctx *core.Context) error {
			ctx.Migrations().Register("logstore", sqlFS)
			core.Provide[core.MigrationEngine](ctx, runner)
			return nil
		},
	}

	app := core.NewApp(
		core.WithProfile(core.ProfileAll),
		core.WithPlugins(p),
	)

	err := app.Start(context.Background())
	require.NoError(t, err)
	defer func() { _ = app.Stop(context.Background()) }()

	assert.True(t, executed)
}

func TestAppRunContextCancellation(t *testing.T) {
	d := newAppMockDriver(core.DriverTypeHTTP)
	p := &appMockPlugin{
		name: "http_plugin",
		applyFn: func(ctx *core.Context) error {
			return ctx.RegisterDriver(d)
		},
	}

	app := core.NewApp(
		core.WithProfile(core.ProfileAPI),
		core.WithPlugins(p),
		core.WithShutdownTimeout(1*time.Second),
	)

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- app.Run(ctx)
	}()

	// Wait briefly then cancel
	time.Sleep(50 * time.Millisecond)
	assert.True(t, app.IsRunning())
	assert.True(t, d.isStarted())

	cancel()

	select {
	case err := <-errCh:
		assert.NoError(t, err)
		assert.False(t, app.IsRunning())
		assert.True(t, d.isStopped())
	case <-time.After(3 * time.Second):
		t.Fatal("app.Run did not terminate upon context cancellation")
	}
}

func TestAppExecuteCLI(t *testing.T) {
	// Test CLI argument parsing logic
	tests := []struct {
		args            []string
		expectedProfile core.Profile
	}{
		{args: []string{"api"}, expectedProfile: core.ProfileAPI},
		{args: []string{"worker"}, expectedProfile: core.ProfileWorker},
		{args: []string{"scheduler"}, expectedProfile: core.ProfileSchedule},
		{args: []string{"schedule"}, expectedProfile: core.ProfileSchedule},
		{args: []string{"all"}, expectedProfile: core.ProfileAll},
		{args: []string{"--profile=worker"}, expectedProfile: core.ProfileWorker},
		{args: []string{"-p=api"}, expectedProfile: core.ProfileAPI},
	}

	for _, tt := range tests {
		t.Run(tt.args[0], func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel() // cancel immediately

			// Use custom root context to control cancellation
			customApp := core.NewApp(core.WithContext(core.NewContext(ctx)))
			_ = customApp.ExecuteCLI(tt.args...)
			assert.Equal(t, tt.expectedProfile, customApp.Profile())
		})
	}
}

func TestAppIdempotencyAndErrorStates(t *testing.T) {
	app := core.NewApp()

	// Double start returns error
	err := app.Start(context.Background())
	require.NoError(t, err)

	err = app.Start(context.Background())
	assert.ErrorIs(t, err, core.ErrAppRunning)

	// Stop clears running state
	err = app.Stop(context.Background())
	require.NoError(t, err)

	// Double stop succeeds
	err = app.Stop(context.Background())
	require.NoError(t, err)

	// Plugin apply failure
	failPlugin := &appMockPlugin{
		name: "failing_plugin",
		applyFn: func(ctx *core.Context) error {
			return errors.New("plugin init boom")
		},
	}
	app2 := core.NewApp(core.WithPlugins(failPlugin))
	err = app2.Start(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "plugin init boom")
	assert.False(t, app2.IsRunning())

	// Migration failure
	migFailRunner := core.MigrationRunner(func(ctx *core.Context, entries []extpoints.MigrationEntry) error {
		return errors.New("sql migrate error")
	})
	sqlFS := fstest.MapFS{
		"migrations/001.sql": &fstest.MapFile{Data: []byte("...")},
	}
	migPlugin := &appMockPlugin{
		name: "db_plugin",
		applyFn: func(ctx *core.Context) error {
			ctx.Migrations().Register("db_plugin", sqlFS)
			return nil
		},
	}
	app3 := core.NewApp(
		core.WithPlugins(migPlugin),
		core.WithMigrationRunner(migFailRunner),
	)
	err = app3.Start(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sql migrate error")
	assert.False(t, app3.IsRunning())
}
