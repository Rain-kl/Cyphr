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

func TestAdminMigrationsIncludeTaskExecutionsAndSchedules(t *testing.T) {
	ctx := core.NewContext(context.Background())
	p := admin.New()
	require.NoError(t, p.Apply(ctx))

	entry, ok := ctx.Migrations().Get("admin")
	require.True(t, ok, "admin plugin must register migrations")
	assert.Equal(t, "admin", entry.PluginID)

	// Verify sqlite migration files include w_schedules and w_task_executions
	sqliteDir, err := entry.FS.Open("migrations/sqlite/00001_initial.sql")
	require.NoError(t, err)
	defer sqliteDir.Close()

	stat, err := sqliteDir.Stat()
	require.NoError(t, err)
	buf := make([]byte, stat.Size())
	_, err = sqliteDir.Read(buf)
	require.NoError(t, err)
	content := string(buf)

	assert.Contains(t, content, "CREATE TABLE IF NOT EXISTS w_task_executions")
	assert.Contains(t, content, "CREATE TABLE IF NOT EXISTS w_schedules")
	assert.Contains(t, content, "CREATE TABLE IF NOT EXISTS w_system_configs")
	assert.Contains(t, content, "CREATE TABLE IF NOT EXISTS w_templates")

	// Verify postgres migration files include w_schedules and w_task_executions
	pgDir, err := entry.FS.Open("migrations/postgres/00001_initial.sql")
	require.NoError(t, err)
	defer pgDir.Close()

	stat, err = pgDir.Stat()
	require.NoError(t, err)
	buf = make([]byte, stat.Size())
	_, err = pgDir.Read(buf)
	require.NoError(t, err)
	pgContent := string(buf)

	assert.Contains(t, pgContent, "CREATE TABLE IF NOT EXISTS w_task_executions")
	assert.Contains(t, pgContent, "CREATE TABLE IF NOT EXISTS w_schedules")
	assert.Contains(t, pgContent, "CREATE TABLE IF NOT EXISTS w_system_configs")
	assert.Contains(t, pgContent, "CREATE TABLE IF NOT EXISTS w_templates")
}
