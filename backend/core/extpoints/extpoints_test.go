// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package extpoints_test

import (
	"context"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"Wavelet/core"
	"Wavelet/core/extpoints"
)

func TestRouterExtension(t *testing.T) {
	r := extpoints.NewRouterRegistry()
	require.NotNil(t, r)

	mGlobal := "global_middleware"
	r.Use(mGlobal)
	assert.Equal(t, []any{mGlobal}, r.Middlewares())

	// Test root methods
	hRoot := "root_handler"
	r.GET("/", hRoot)
	r.POST("/root_post", hRoot)
	r.PUT("/root_put", hRoot)
	r.DELETE("/root_del", hRoot)
	r.PATCH("/root_patch", hRoot)
	r.HEAD("/root_head", hRoot)
	r.OPTIONS("/root_opt", hRoot)
	anyRootDefs := r.Any("/root_any", hRoot)
	assert.Len(t, anyRootDefs, 7)

	// Group and Group.Use
	mAPI := "api_middleware"
	api := r.Group("/api/v1", mAPI)
	api.Use("api_extra_middleware")
	assert.Len(t, api.Middlewares(), 2)

	hList := "list_orders_handler"
	hCreate := "create_order_handler"
	api.GET("/orders", hList)
	api.POST("/orders", hCreate)

	mAdmin := "admin_middleware"
	admin := api.Group("admin", mAdmin)

	hUserGet := "get_user_handler"
	hUserPut := "put_user_handler"
	hUserDel := "del_user_handler"
	hUserPatch := "patch_user_handler"
	hUserHead := "head_user_handler"
	hUserOptions := "options_user_handler"
	admin.GET("/users/:id", hUserGet)
	admin.PUT("/users/:id", hUserPut)
	admin.DELETE("/users/:id", hUserDel)
	admin.PATCH("/users/:id", hUserPatch)
	admin.HEAD("/users/:id", hUserHead)
	admin.OPTIONS("/users/:id", hUserOptions)

	hCustom := "custom_handler"
	admin.Handle("CUSTOM", "/custom", hCustom)

	hAny := "any_handler"
	anyRoutes := admin.Any("/all", hAny)
	assert.NotEmpty(t, anyRoutes)

	// Group.Routes() returns root routes
	assert.Equal(t, r.Routes(), admin.Routes())

	routes := r.Routes()

	// Verify route paths and middlewares
	var foundOrderGet bool
	var foundUserPut bool
	for _, route := range routes {
		if route.Method == "GET" && route.Path == "/api/v1/orders" {
			foundOrderGet = true
			assert.Equal(t, []any{mGlobal, mAPI, "api_extra_middleware"}, route.Middlewares)
			assert.Equal(t, []any{hList}, route.Handlers)
		}
		if route.Method == "PUT" && route.Path == "/api/v1/admin/users/:id" {
			foundUserPut = true
			assert.Equal(t, []any{mGlobal, mAPI, "api_extra_middleware", mAdmin}, route.Middlewares)
			assert.Equal(t, []any{hUserPut}, route.Handlers)
		}
	}
	assert.True(t, foundOrderGet)
	assert.True(t, foundUserPut)
}

func TestMigrationExtension(t *testing.T) {
	m := extpoints.NewMigrationRegistry()
	require.NotNil(t, m)

	fs1 := fstest.MapFS{
		"migrations/001_init.sql": &fstest.MapFile{Data: []byte("CREATE TABLE t1(id int);")},
	}
	fs2 := fstest.MapFS{
		"custom/001_order.sql": &fstest.MapFile{Data: []byte("CREATE TABLE t2(id int);")},
	}

	m.Register("auth", fs1)
	m.Register("order", fs2, "custom")

	// Update existing entry
	fs1Updated := fstest.MapFS{
		"migrations/002_update.sql": &fstest.MapFile{Data: []byte("ALTER TABLE t1 ADD col int;")},
	}
	m.Register("auth", fs1Updated, "")

	entries := m.Entries()
	require.Len(t, entries, 2)
	assert.Equal(t, "auth", entries[0].PluginID)
	assert.Equal(t, "migrations", entries[0].Dir)
	assert.Equal(t, "order", entries[1].PluginID)
	assert.Equal(t, "custom", entries[1].Dir)

	authEntry, ok := m.Get("auth")
	assert.True(t, ok)
	assert.Equal(t, "auth", authEntry.PluginID)

	_, ok = m.Get("non_existent")
	assert.False(t, ok)
}

func TestTaskExtension(t *testing.T) {
	tr := extpoints.NewTaskRegistry()
	require.NotNil(t, tr)

	handler := func(ctx context.Context, payload []byte) error { return nil }

	tr.Register("order:cancel_timeout", handler,
		extpoints.WithTaskConcurrency(5),
		extpoints.WithTaskRetry(3),
		extpoints.WithTaskTimeout(10*time.Second),
		extpoints.WithTaskMetadata("queue", "critical"),
		nil, // test nil option
	)

	// Re-register to test update
	tr.Register("order:cancel_timeout", handler,
		extpoints.WithTaskConcurrency(10),
		extpoints.WithTaskMetadata("queue", "high"),
	)

	tasks := tr.Tasks()
	require.Len(t, tasks, 1)
	assert.Equal(t, "order:cancel_timeout", tasks[0].Pattern)
	assert.Equal(t, 10, tasks[0].Concurrency)
	assert.Equal(t, "high", tasks[0].Metadata["queue"])

	task, ok := tr.Get("order:cancel_timeout")
	assert.True(t, ok)
	assert.Equal(t, "order:cancel_timeout", task.Pattern)

	_, ok = tr.Get("unknown")
	assert.False(t, ok)
}

func TestScheduleExtension(t *testing.T) {
	sr := extpoints.NewScheduleRegistry()
	require.NotNil(t, sr)

	type ReportPayload struct {
		Type string `json:"type"`
	}

	sr.RegisterCron("0 2 * * *", "report:daily_summary", ReportPayload{Type: "daily"})
	sr.Register("@every 1h", "cleanup:expired_sessions", nil,
		extpoints.WithScheduleOption("retry", 2),
		nil, // test nil option
	)

	// Re-register to test update
	sr.RegisterCron("0 3 * * *", "report:daily_summary", ReportPayload{Type: "all"})

	schedules := sr.Schedules()
	require.Len(t, schedules, 2)

	assert.Equal(t, "0 3 * * *", schedules[0].Spec)
	assert.Equal(t, "report:daily_summary", schedules[0].TaskType)
	assert.Equal(t, ReportPayload{Type: "all"}, schedules[0].Payload)

	assert.Equal(t, "@every 1h", schedules[1].Spec)
	assert.Equal(t, "cleanup:expired_sessions", schedules[1].TaskType)
	assert.Equal(t, 2, schedules[1].Options["retry"])

	sched, ok := sr.Get("report:daily_summary")
	assert.True(t, ok)
	assert.Equal(t, "0 3 * * *", sched.Spec)

	_, ok = sr.Get("unknown")
	assert.False(t, ok)
}

func TestSettingExtension(t *testing.T) {
	sr := extpoints.NewSettingRegistry()
	require.NotNil(t, sr)

	assert.Panics(t, func() {
		sr.Register(extpoints.SettingSchema{}) // empty key panics
	})

	sr.Register(extpoints.SettingSchema{
		Key:         "order.auto_cancel_mins",
		Default:     15,
		Description: "Order auto cancellation timeout in minutes",
		Category:    "order",
		Public:      true,
	})

	// Re-register to test update
	sr.Register(extpoints.SettingSchema{
		Key:         "order.auto_cancel_mins",
		Default:     30,
		Description: "Updated timeout",
	})

	sr.Register(extpoints.SettingSchema{
		Key:         "auth.jwt_secret",
		Default:     "default-secret",
		Description: "JWT secret key",
		Category:    "auth",
		ReadOnly:    true,
	})

	schemas := sr.Schemas()
	require.Len(t, schemas, 2)

	schema, ok := sr.Get("order.auto_cancel_mins")
	assert.True(t, ok)
	assert.Equal(t, 30, schema.Default)

	_, ok = sr.Get("unknown")
	assert.False(t, ok)
}

func TestContextExtensionPointsIntegration(t *testing.T) {
	ctx := core.NewContext(context.Background())

	require.NotNil(t, ctx.Events())
	require.NotNil(t, ctx.Router())
	require.NotNil(t, ctx.Migrations())
	require.NotNil(t, ctx.Tasks())
	require.NotNil(t, ctx.Task())
	require.NotNil(t, ctx.Schedules())
	require.NotNil(t, ctx.Schedule())
	require.NotNil(t, ctx.Settings())
	require.NotNil(t, ctx.Setting())

	// Register from child context and verify shared application registry
	child := ctx.Fork()
	child.Router().GET("/ping", "pong_handler")
	child.Task().Register("sample:task", "handler")
	child.Schedule().RegisterCron("@hourly", "sample:cron", nil)
	child.Settings().Register(extpoints.SettingSchema{
		Key:     "app.name",
		Default: "Wavelet",
	})

	assert.Len(t, ctx.Router().Routes(), 1)
	assert.Len(t, ctx.Tasks().Tasks(), 1)
	assert.Len(t, ctx.Schedules().Schedules(), 1)
	assert.Len(t, ctx.Settings().Schemas(), 1)

	// Child and root events
	var eventReceived bool
	child.Events().On("app:ready", func() {
		eventReceived = true
	})
	err := ctx.Events().Emit(context.Background(), "app:ready", nil)
	assert.NoError(t, err)
	assert.True(t, eventReceived)
}

func TestExtensionPointsUnregister(t *testing.T) {
	ctx := core.NewContext(context.Background())

	// 1. Router unregister
	rd := ctx.Router().GET("/temp", "temp_handler")
	assert.Greater(t, rd.ID, uint64(0))
	assert.Len(t, ctx.Router().Routes(), 1)
	assert.True(t, ctx.Router().Unregister("GET", "/temp"))
	assert.Len(t, ctx.Router().Routes(), 0)

	rd2 := ctx.Router().POST("/temp2", "temp2_handler")
	assert.Len(t, ctx.Router().Routes(), 1)
	assert.True(t, ctx.Router().UnregisterByID(rd2.ID))
	assert.Len(t, ctx.Router().Routes(), 0)

	// 2. Task unregister
	ctx.Task().Register("temp:task", "handler")
	assert.Len(t, ctx.Task().Tasks(), 1)
	assert.True(t, ctx.Task().Unregister("temp:task"))
	assert.Len(t, ctx.Task().Tasks(), 0)

	// 3. Schedule unregister
	ctx.Schedule().RegisterCron("@hourly", "temp:cron", nil)
	assert.Len(t, ctx.Schedule().Schedules(), 1)
	assert.True(t, ctx.Schedule().Unregister("temp:cron"))
	assert.Len(t, ctx.Schedule().Schedules(), 0)

	// 4. Setting unregister
	ctx.Settings().Register(extpoints.SettingSchema{Key: "temp.key", Default: 1})
	assert.Len(t, ctx.Settings().Schemas(), 1)
	assert.True(t, ctx.Settings().Unregister("temp.key"))
	assert.Len(t, ctx.Settings().Schemas(), 0)

	// 5. Migration unregister
	fsys := fstest.MapFS{"001.sql": &fstest.MapFile{Data: []byte("-- migration")}}
	ctx.Migrations().Register("temp_plugin", fsys)
	assert.Len(t, ctx.Migrations().Entries(), 1)
	assert.True(t, ctx.Migrations().Unregister("temp_plugin"))
	assert.Len(t, ctx.Migrations().Entries(), 0)
}
