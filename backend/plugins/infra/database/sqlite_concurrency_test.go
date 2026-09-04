// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package database_test

import (
	"Wavelet/plugins/infra/database"
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

type concurrentProbeRow struct {
	ID    uint64 `gorm:"primaryKey"`
	Value int
}

// TestInitSQLiteWritePragmas verifies the embedded SQLite instance is tuned
// for concurrent writers (WAL + busy timeout + single connection), otherwise
// concurrent agent log batches fail with SQLITE_BUSY (surfaced as HTTP 500).
func TestInitSQLiteWritePragmas(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "pragma_test.db")
	gdb, err := database.InitDBWithConfig(database.Config{SQLitePath: dbPath}, false)
	require.NoError(t, err)

	var journalMode string
	require.NoError(t, gdb.Raw("PRAGMA journal_mode;").Scan(&journalMode).Error)
	require.Equal(t, "wal", journalMode)

	var busyTimeout int
	require.NoError(t, gdb.Raw("PRAGMA busy_timeout;").Scan(&busyTimeout).Error)
	require.Equal(t, 5000, busyTimeout)

	sqlDB, err := gdb.DB()
	require.NoError(t, err)
	require.Equal(t, 1, sqlDB.Stats().MaxOpenConnections)
}

// TestInitSQLiteConcurrentWrites mimics concurrent agent log appends (insert
// + progress update per batch) and requires zero write errors.
func TestInitSQLiteConcurrentWrites(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "concurrent_test.db")
	gdb, err := database.InitDBWithConfig(database.Config{SQLitePath: dbPath}, false)
	require.NoError(t, err)
	require.NoError(t, gdb.AutoMigrate(&concurrentProbeRow{}))
	require.NoError(t, gdb.Create(&concurrentProbeRow{ID: 1, Value: 0}).Error)

	const writers = 8
	const batches = 25

	var wg sync.WaitGroup
	errCh := make(chan error, writers*batches*2)
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for b := 0; b < batches; b++ {
				id := uint64(w*batches + b + 2)
				if err := gdb.Create(&concurrentProbeRow{ID: id, Value: b}).Error; err != nil {
					errCh <- fmt.Errorf("writer %d insert %d: %w", w, b, err)
				}
				if err := gdb.Model(&concurrentProbeRow{}).Where("id = ?", 1).Update("value", b).Error; err != nil {
					errCh <- fmt.Errorf("writer %d progress update %d: %w", w, b, err)
				}
			}
		}(w)
	}
	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("concurrent sqlite write failed: %v", err)
	}

	var count int64
	require.NoError(t, gdb.Model(&concurrentProbeRow{}).Count(&count).Error)
	require.Equal(t, int64(writers*batches+1), count)
}
