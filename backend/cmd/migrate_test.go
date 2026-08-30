// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"Wavelet/core"
	"Wavelet/core/contracts"
	"context"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type migrateTestDB struct {
	db *gorm.DB
}

func (s migrateTestDB) GORM() *gorm.DB { return s.db }

func (s migrateTestDB) DB(ctx context.Context) *gorm.DB { return s.db.WithContext(ctx) }

func (s migrateTestDB) Named(string) *gorm.DB { return s.db }

type migrateTestPlugin struct {
	db *gorm.DB
	fs fstest.MapFS
}

func (p *migrateTestPlugin) Name() string { return "t" }

func (p *migrateTestPlugin) Apply(ctx *core.Context) error {
	core.Provide[contracts.DBService](ctx, migrateTestDB{db: p.db})
	ctx.Migrations().Register("t", p.fs)
	return nil
}

func sqliteTableExists(t *testing.T, db *gorm.DB, name string) bool {
	t.Helper()
	var n int
	err := db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?", name).Scan(&n).Error
	require.NoError(t, err)
	return n > 0
}

func testMigrationFS() fstest.MapFS {
	return fstest.MapFS{
		"migrations/sqlite/00001_init.sql": &fstest.MapFile{Data: []byte(`-- +goose Up
CREATE TABLE t_up (id INTEGER PRIMARY KEY);

-- +goose Down
DROP TABLE t_up;
`)},
	}
}

func openMigrateTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "migrate.db")
	gdb, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	require.NoError(t, err)
	return gdb
}

func TestGooseEngineMigrateOrderCreateTableBaselineUp(t *testing.T) {
	gdb := openMigrateTestDB(t)
	var order []string

	app := core.NewApp(
		core.WithMigrationEngine(&gooseEngine{}),
		core.WithMigrationBaseline(func(*core.Context) error {
			require.True(t, sqliteTableExists(t, gdb, "w_schema_versions"), "version table must exist before baseline")
			require.False(t, sqliteTableExists(t, gdb, "t_up"), "plugin Up must not run before baseline")
			order = append(order, "create-table", "baseline")
			return nil
		}),
		core.WithPlugins(&migrateTestPlugin{db: gdb, fs: testMigrationFS()}),
	)

	require.NoError(t, app.Prepare())
	require.NoError(t, app.ApplyPlugins())
	require.NoError(t, app.RunMigrations())

	require.True(t, sqliteTableExists(t, gdb, "t_up"), "plugin Up must run after baseline")
	order = append(order, "up")
	assert.Equal(t, []string{"create-table", "baseline", "up"}, order)
}

func TestGooseEngineBaselineErrorSkipsUp(t *testing.T) {
	gdb := openMigrateTestDB(t)

	app := core.NewApp(
		core.WithMigrationEngine(&gooseEngine{}),
		core.WithMigrationBaseline(func(*core.Context) error {
			require.True(t, sqliteTableExists(t, gdb, "w_schema_versions"), "version table must exist before baseline")
			return assert.AnError
		}),
		core.WithPlugins(&migrateTestPlugin{db: gdb, fs: testMigrationFS()}),
	)

	require.NoError(t, app.Prepare())
	require.NoError(t, app.ApplyPlugins())
	err := app.RunMigrations()
	require.Error(t, err)
	assert.ErrorContains(t, err, "migration baseline")
	assert.False(t, sqliteTableExists(t, gdb, "t_up"), "plugin Up must not run when baseline fails")
}

func TestGooseEngineNilBaselineStillMigrates(t *testing.T) {
	gdb := openMigrateTestDB(t)

	app := core.NewApp(
		core.WithMigrationEngine(&gooseEngine{}),
		core.WithPlugins(&migrateTestPlugin{db: gdb, fs: testMigrationFS()}),
	)

	require.NoError(t, app.Prepare())
	require.NoError(t, app.ApplyPlugins())
	require.NoError(t, app.RunMigrations())
	assert.True(t, sqliteTableExists(t, gdb, "w_schema_versions"))
	assert.True(t, sqliteTableExists(t, gdb, "t_up"))
}
