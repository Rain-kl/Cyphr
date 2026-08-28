// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package admin_test

import (
	"Wavelet/core"
	"Wavelet/plugins/domain/admin"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminPluginUnit(t *testing.T) {
	ctx := core.NewContext(context.Background())
	p := admin.New()
	assert.Equal(t, "admin", p.Name())
	assert.Equal(t, "1.0.0", p.Manifest().Version)
	require.NoError(t, p.Apply(ctx))

	// Verify routes
	routes := ctx.Router().Routes()
	assert.NotEmpty(t, routes)

	// Verify tasks
	_, ok := ctx.Tasks().Get("admin:system_cleanup")
	require.True(t, ok)

	// Verify schedules
	sched, ok := ctx.Schedules().Get("admin:system_cleanup")
	require.True(t, ok)
	assert.Equal(t, "0 4 * * *", sched.Spec)

	// Verify settings
	setting, ok := ctx.Settings().Get("admin.system_cleanup_cron")
	require.True(t, ok)
	assert.Equal(t, "0 4 * * *", setting.Default)
}
