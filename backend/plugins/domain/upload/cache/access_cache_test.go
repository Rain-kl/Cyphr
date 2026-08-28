// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"context"
	"testing"
	"time"

	"Wavelet/plugins/domain/upload/shared"
	uploadstorage "Wavelet/plugins/domain/upload/storage"
)

func TestLoadMigrationAccessStateCachesResult(t *testing.T) {
	_, cleanup := shared.SetupTestEnv(t)
	defer cleanup()
	ResetAccessCaches()

	ctx := context.Background()
	first := uploadstorage.LoadMigrationAccessState(ctx)
	second := uploadstorage.LoadMigrationAccessState(ctx)

	if first.ReadOnly != second.ReadOnly {
		t.Fatalf("readOnly mismatch: first=%v second=%v", first.ReadOnly, second.ReadOnly)
	}
	if first.HasTarget != second.HasTarget {
		t.Fatalf("hasTarget mismatch: first=%v second=%v", first.HasTarget, second.HasTarget)
	}
}

func TestIsFilePublicUsesCachedWhitelist(t *testing.T) {
	_, cleanup := shared.SetupTestEnv(t)
	defer cleanup()
	ResetAccessCaches()

	ctx := context.Background()
	if !IsFilePublic(ctx, "avatar") {
		t.Fatal("expected avatar to be public by default")
	}
	if IsFilePublic(ctx, "attachment") {
		t.Fatal("expected attachment to be private by default")
	}
	if !IsFilePublic(ctx, "AVATAR") {
		t.Fatal("expected whitelist lookup to be case-insensitive")
	}
}

func TestResetAccessCachesRefreshesWhitelist(t *testing.T) {
	dbConn, cleanup := shared.SetupTestEnv(t)
	defer cleanup()
	ResetAccessCaches()

	ctx := context.Background()
	if !IsFilePublic(ctx, "avatar") {
		t.Fatal("expected seeded avatar whitelist before reset")
	}

	if err := dbConn.Table("w_system_configs").Where("key = ?", "file_access_whitelist").Update("value", `["attachment"]`).Error; err != nil {
		t.Fatalf("update whitelist config: %v", err)
	}

	ResetAccessCaches()
	if !IsFilePublic(ctx, "attachment") {
		t.Fatal("expected attachment to be public after whitelist refresh")
	}
	if IsFilePublic(ctx, "avatar") {
		t.Fatal("expected avatar to be private after whitelist refresh")
	}
}

func TestAccessCacheTTLExpires(t *testing.T) {
	_, cleanup := shared.SetupTestEnv(t)
	defer cleanup()
	ResetAccessCaches()

	ctx := context.Background()
	_ = loadFileAccessWhitelist(ctx)

	fileAccessWhitelistMu.Lock()
	fileAccessWhitelistCheckedAt = time.Now().Add(-time.Duration(shared.AccessCacheTTL)*time.Second - time.Second)
	fileAccessWhitelistMu.Unlock()

	// Should still work after TTL by reloading from config.
	if !IsFilePublic(ctx, "avatar") {
		t.Fatal("expected whitelist reload after TTL expiration")
	}
}
