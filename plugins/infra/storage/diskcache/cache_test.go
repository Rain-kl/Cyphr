// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package diskcache

import (
	"context"
	"os"
	"testing"

	db "github.com/Rain-kl/Wavelet/pkg/persistence"
	"github.com/Rain-kl/Wavelet/pkg/testhelper"
)

func TestDiskCacheReloadConfig(t *testing.T) {
	dbConn, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()

	testDir := "uploads/test_diskcache_reload"
	defer func() { _ = os.RemoveAll(testDir) }()
	_ = os.RemoveAll(testDir)

	c := New(testDir)
	defer func() { _ = c.Clear() }()

	// Update DB config values
	dbConn.Table("w_system_configs").Where("key = ?", "disk_cache_max_size_mb").Update("value", "250")
	dbConn.Table("w_system_configs").Where("key = ?", "disk_cache_ttl_minutes").Update("value", "120")
	dbConn.Table("w_system_configs").Where("key = ?", "disk_cache_lru_enabled").Update("value", "false")

	// Invalidate Redis config cache to force DB reload
	if db.Redis != nil {
		db.Redis.Del(context.Background(), db.PrefixedKey("system_configs"))
	}

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
