// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"Wavelet/plugins/domain/upload/shared"
	"context"
	"testing"
)

// configRow mirrors just enough of w_system_configs to rebuild it mid-test.
type configRow struct {
	Key   string `gorm:"primaryKey;size:64"`
	Value string `gorm:"type:text;not null"`
	Type  string `gorm:"size:32;not null"`
}

// TableName returns the system config table.
func (configRow) TableName() string { return "w_system_configs" }

// TestWhitelistReadFailureIsNotCachedAsConfigured pins the defect where a failed
// read of the public access whitelist was discarded and the resulting restricted
// default was then cached as though the admin had configured it, narrowing access
// for the whole TTL with nothing to retry it sooner.
func TestWhitelistReadFailureIsNotCachedAsConfigured(t *testing.T) {
	db, cleanup := shared.SetupTestEnv(t)
	defer cleanup()
	ResetAccessCaches()
	t.Cleanup(ResetAccessCaches)

	ctx := context.Background()

	// Make the config unreadable, then take one sample through the failure path.
	if err := db.Exec("DROP TABLE w_system_configs").Error; err != nil {
		t.Fatalf("drop config table: %v", err)
	}
	if IsFilePublic(ctx, "document") {
		t.Fatal(`document reported public while the whitelist cannot be read`)
	}

	// Restore a configured whitelist that includes document.
	if err := db.AutoMigrate(&configRow{}); err != nil {
		t.Fatalf("recreate config table: %v", err)
	}
	row := configRow{Key: "file_access_whitelist", Value: `["avatar","document"]`, Type: "system"}
	if err := db.Create(&row).Error; err != nil {
		t.Fatalf("seed whitelist: %v", err)
	}

	// A failed first read must not have been cached as a real answer, so this call
	// has to re-read and see the configured list.
	if !IsFilePublic(ctx, "document") {
		t.Error("whitelist stayed pinned to the default after a read failure; the failed lookup must be retried")
	}
}
