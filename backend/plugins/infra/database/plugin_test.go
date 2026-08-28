// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package database_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Rain-kl/Wavelet/backend/core"
	"github.com/Rain-kl/Wavelet/backend/core/contracts"
	"github.com/Rain-kl/Wavelet/backend/plugins/infra/database"
)

type Item struct {
	ID   uint64 `gorm:"primaryKey"`
	Name string
}

func TestDatabasePlugin(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "db_test.db")
	gdb, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, gdb.AutoMigrate(&Item{}))

	namedPath := filepath.Join(t.TempDir(), "named_test.db")
	namedDB, err := gorm.Open(sqlite.Open(namedPath), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, namedDB.AutoMigrate(&Item{}))

	p := database.New(
		database.WithDB(gdb),
		database.WithNamedDB("analytics", namedDB),
	)
	assert.Equal(t, "database", p.Name())

	ctx := core.NewContext(context.Background())
	require.NoError(t, p.Apply(ctx))

	svc, err := core.Inject[contracts.DBService](ctx)
	require.NoError(t, err)
	require.NotNil(t, svc)

	assert.Equal(t, gdb, svc.GORM())
	assert.NotNil(t, svc.DB(context.Background()))
	assert.Equal(t, namedDB, svc.Named("analytics"))
	assert.Equal(t, gdb, svc.Named("non_existent"))

	// Verify DB write
	item := Item{ID: 1, Name: "TestItem"}
	require.NoError(t, svc.DB(context.Background()).Create(&item).Error)

	var retrieved Item
	require.NoError(t, svc.GORM().First(&retrieved, 1).Error)
	assert.Equal(t, "TestItem", retrieved.Name)
}
