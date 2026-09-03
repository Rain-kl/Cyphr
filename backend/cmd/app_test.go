// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"Wavelet/core"
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewWaveletAppProfiles(t *testing.T) {
	profiles := []core.Profile{
		core.ProfileAPI,
		core.ProfileWorker,
		core.ProfileSchedule,
		core.ProfileAll,
	}

	for _, prof := range profiles {
		t.Run(string(prof), func(t *testing.T) {
			app := newWaveletApp(prof, core.WithConfigValues(map[string]any{
				"app": map[string]any{
					"addr": "127.0.0.1:0",
				},
				"redis": map[string]any{
					"enabled": false,
				},
			}))
			require.NotNil(t, app)
			assert.Equal(t, prof, app.Profile())

			// 3 infra + 2 cache + 4 worker/cron + 7 domain + 1 transcribe + 1 http driver = 18 plugins
			plugins := app.Plugins()
			assert.Len(t, plugins, 18)

			require.NoError(t, app.Reconcile())

			// Verify standard infra plugins
			_, ok := app.Plugin("database")
			assert.True(t, ok, "database plugin missing")

			_, ok = app.Plugin("logger")
			assert.True(t, ok, "logger plugin missing")

			_, ok = app.Plugin("storage")
			assert.True(t, ok, "storage plugin missing")

			// In zero-Redis mode (default in test)
			f, ok := app.Fiber("cache_memory")
			assert.True(t, ok, "cache_memory fiber missing")
			assert.Equal(t, core.FiberActive, f.State())

			f, ok = app.Fiber("cache")
			assert.True(t, ok, "cache fiber missing")
			assert.Equal(t, core.FiberSkipped, f.State())

			f, ok = app.Fiber("driver_inproc_worker")
			assert.True(t, ok, "inproc worker driver fiber missing")
			assert.Equal(t, core.FiberActive, f.State())

			f, ok = app.Fiber("driver_asynq_worker")
			assert.True(t, ok, "asynq worker driver fiber missing")
			assert.Equal(t, core.FiberSkipped, f.State())

			f, ok = app.Fiber("driver_inproc_cron")
			assert.True(t, ok, "inproc scheduler driver fiber missing")
			assert.Equal(t, core.FiberActive, f.State())

			f, ok = app.Fiber("driver_asynq_cron")
			assert.True(t, ok, "asynq scheduler driver fiber missing")
			assert.Equal(t, core.FiberSkipped, f.State())

			// Verify domain plugins
			_, ok = app.Plugin("auth")
			assert.True(t, ok, "auth plugin missing")

			_, ok = app.Plugin("user")
			assert.True(t, ok, "user plugin missing")

			_, ok = app.Plugin("msg_gateway")
			assert.True(t, ok, "msg_gateway plugin missing")

			_, ok = app.Plugin("risk_control")
			assert.True(t, ok, "risk_control plugin missing")

			_, ok = app.Plugin("admin")
			assert.True(t, ok, "admin plugin missing")

			_, ok = app.Plugin("upload")
			assert.True(t, ok, "upload plugin missing")

			_, ok = app.Plugin("system")
			assert.True(t, ok, "system plugin missing")

			_, ok = app.Plugin("transcribe_svr")
			assert.True(t, ok, "transcribe_svr plugin missing")

			// Verify driver plugins
			_, ok = app.Plugin("driver_http")
			assert.True(t, ok, "http driver missing")
		})
	}
}

func TestNewWaveletAppWithRedisEnabled(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	app := newWaveletApp(core.ProfileAll, core.WithConfigValues(map[string]any{
		"redis": map[string]any{
			"enabled": true,
			"addrs":   []string{mr.Addr()},
		},
	}))
	require.NotNil(t, app)
	defer func() {
		_ = app.Stop(context.Background())
		_ = app.Context().Dispose()
	}()
	require.NoError(t, app.Reconcile())

	f, ok := app.Fiber("cache")
	assert.True(t, ok, "cache plugin missing in Redis mode")
	assert.Equal(t, core.FiberActive, f.State())

	f, ok = app.Fiber("driver_asynq_worker")
	assert.True(t, ok, "asynq worker driver missing in Redis mode")
	assert.Equal(t, core.FiberActive, f.State())

	f, ok = app.Fiber("driver_asynq_cron")
	assert.True(t, ok, "asynq scheduler driver missing in Redis mode")
	assert.Equal(t, core.FiberActive, f.State())

	f, ok = app.Fiber("cache_memory")
	assert.True(t, ok)
	assert.Equal(t, core.FiberSkipped, f.State())

	f, ok = app.Fiber("driver_inproc_worker")
	assert.True(t, ok)
	assert.Equal(t, core.FiberSkipped, f.State())

	f, ok = app.Fiber("driver_inproc_cron")
	assert.True(t, ok)
	assert.Equal(t, core.FiberSkipped, f.State())
}
