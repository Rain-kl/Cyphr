// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package diskcache

import (
	"context"
	"testing"

	"gorm.io/gorm"

	"Wavelet/pkg/testhelper"
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

func (m *mockDBService) Named(_ string) *gorm.DB {
	return m.db
}

func TestDiskCacheReloadConfig(t *testing.T) {
	dbConn, _, cleanup := testhelper.SetupTestEnvironment(t)
	SetDBService(&mockDBService{db: dbConn})
	defer func() {
		SetDBService(nil)
		cleanup()
	}()

	testDir := t.TempDir()

	c := New(testDir)
	defer func() { _ = c.Clear() }()

	// Update DB config values
	dbConn.Table("w_system_configs").Where("key = ?", "disk_cache_max_size_mb").Update("value", "250")
	dbConn.Table("w_system_configs").Where("key = ?", "disk_cache_ttl_minutes").Update("value", "120")
	dbConn.Table("w_system_configs").Where("key = ?", "disk_cache_lru_enabled").Update("value", "false")

	// Reload config
	c.ReloadConfig(context.Background())

	status := c.Status()
	if status.MaxSizeMB != 250 {
		t.Errorf("expected MaxSizeMB to be 250, got %d", status.MaxSizeMB)
	}
	if status.TTLMinutes != 120 {
		t.Errorf("expected TTLMinutes to be 120, got %d", status.TTLMinutes)
	}
	if status.LRUEnabled != false {
		t.Errorf("expected LRUEnabled to be false, got %v", status.LRUEnabled)
	}
}
