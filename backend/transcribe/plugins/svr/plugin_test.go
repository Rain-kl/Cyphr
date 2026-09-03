// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package svr_test

import (
	"Wavelet/core"
	"Wavelet/core/contracts"
	"Wavelet/pkg/idgen"
	"Wavelet/transcribe/plugins/svr"
	"context"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type mockDBService struct {
	db *gorm.DB
}

func (m *mockDBService) GORM() *gorm.DB {
	return m.db
}

func (m *mockDBService) DB(ctx context.Context) *gorm.DB {
	return m.db.WithContext(ctx)
}

func (m *mockDBService) Named(name string) *gorm.DB {
	return m.db
}

func TestSvrPlugin_LifecycleAndRegistration(t *testing.T) {
	_ = idgen.Init(1)

	dbPath := filepath.Join(t.TempDir(), "test_svr_plugin.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)

	p := svr.New(svr.WithDB(db))
	assert.Equal(t, "transcribe_svr", p.Name())
	assert.Equal(t, "1.0.0", p.Manifest().Version)

	app := core.NewApp(
		core.WithPlugins(p),
	)

	core.Provide[contracts.DBService](app.Context(), &mockDBService{db: db})

	require.NoError(t, app.Prepare())
	require.NoError(t, app.ApplyPlugins())

	// Verify migrations registered
	entries := app.Context().Migrations().Entries()
	var foundMigration bool
	for _, e := range entries {
		if e.PluginID == "transcribe_svr" {
			foundMigration = true
			break
		}
	}
	assert.True(t, foundMigration, "transcribe_svr migration entry should be registered")

	// Verify whitelisted routes
	whitelisted := app.Context().Router().Whitelist()
	assert.Contains(t, whitelisted, "/api/v1/agent/*")
	assert.Contains(t, whitelisted, "/api/v1/audio/transcriptions")
	assert.Contains(t, whitelisted, "/api/v1/models")

	// Verify routes registered
	routes := app.Context().Router().Routes()
	assert.NotEmpty(t, routes)

	// Verify cleanup
	require.NoError(t, app.Context().Dispose())
}
