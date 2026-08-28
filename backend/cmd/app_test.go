// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Rain-kl/Wavelet/backend/core"
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
			app := newWaveletApp(prof)
			require.NotNil(t, app)
			assert.Equal(t, prof, app.Profile())

			// Verify 4 infra plugins + 8 domain plugins + 3 driver plugins registered
			plugins := app.Plugins()
			assert.Len(t, plugins, 15)

			// Verify each standard infra plugin is registered
			_, ok := app.Plugin("database")
			assert.True(t, ok, "database plugin missing")

			_, ok = app.Plugin("cache")
			assert.True(t, ok, "cache plugin missing")

			_, ok = app.Plugin("logger")
			assert.True(t, ok, "logger plugin missing")

			_, ok = app.Plugin("storage")
			assert.True(t, ok, "storage plugin missing")

			// Verify domain plugins
			_, ok = app.Plugin("auth")
			assert.True(t, ok, "auth plugin missing")

			_, ok = app.Plugin("user")
			assert.True(t, ok, "user plugin missing")

			_, ok = app.Plugin("message_gateway")
			assert.True(t, ok, "message_gateway plugin missing")

			_, ok = app.Plugin("risk_control")
			assert.True(t, ok, "risk_control plugin missing")

			_, ok = app.Plugin("admin")
			assert.True(t, ok, "admin plugin missing")

			_, ok = app.Plugin("upload")
			assert.True(t, ok, "upload plugin missing")

			_, ok = app.Plugin("cap")
			assert.True(t, ok, "cap plugin missing")

			_, ok = app.Plugin("system")
			assert.True(t, ok, "system plugin missing")

			// Verify driver plugins
			_, ok = app.Plugin("driver_http")
			assert.True(t, ok, "http driver missing")

			_, ok = app.Plugin("driver_asynq_worker")
			assert.True(t, ok, "worker driver missing")

			_, ok = app.Plugin("driver_asynq_cron")
			assert.True(t, ok, "scheduler driver missing")
		})
	}
}
