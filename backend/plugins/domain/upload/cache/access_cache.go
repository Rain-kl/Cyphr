// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package cache provides in-process upload access-control caches.
package cache

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"Wavelet/plugins/domain/upload/shared"
	uploadstorage "Wavelet/plugins/domain/upload/storage"
)

const fileAccessInvalidationChannel = "upload:file_access_invalidation"

var (
	fileAccessWhitelistMu        sync.RWMutex
	fileAccessWhitelistTypes     map[string]struct{}
	fileAccessWhitelistValid     bool
	fileAccessWhitelistCheckedAt time.Time
)

// ResetAccessCaches clears in-process upload access caches.
func ResetAccessCaches() {
	uploadstorage.ResetMigrationAccessCache()

	fileAccessWhitelistMu.Lock()
	fileAccessWhitelistValid = false
	fileAccessWhitelistTypes = nil
	fileAccessWhitelistMu.Unlock()
}

// PublishAccessCacheInvalidation broadcasts upload access cache eviction to all nodes.
func PublishAccessCacheInvalidation(ctx context.Context) {
	if cache := shared.GetCache(ctx); cache != nil {
		_ = cache.Invalidate(ctx, fileAccessInvalidationChannel)
	}
	ResetAccessCaches()
}

// IsFilePublic reports whether uploadType is in the public access whitelist.
func IsFilePublic(ctx context.Context, uploadType string) bool {
	whitelist := loadFileAccessWhitelist(ctx)
	_, ok := whitelist[strings.ToLower(uploadType)]
	return ok
}

func loadFileAccessWhitelist(ctx context.Context) map[string]struct{} {
	fileAccessWhitelistMu.RLock()
	if fileAccessWhitelistValid && time.Since(fileAccessWhitelistCheckedAt) < time.Duration(shared.AccessCacheTTL)*time.Second {
		types := fileAccessWhitelistTypes
		fileAccessWhitelistMu.RUnlock()
		return types
	}
	fileAccessWhitelistMu.RUnlock()

	fileAccessWhitelistMu.Lock()
	defer fileAccessWhitelistMu.Unlock()

	if fileAccessWhitelistValid && time.Since(fileAccessWhitelistCheckedAt) < time.Duration(shared.AccessCacheTTL)*time.Second {
		return fileAccessWhitelistTypes
	}

	fileAccessWhitelistTypes = fetchFileAccessWhitelist(ctx)
	fileAccessWhitelistValid = true
	fileAccessWhitelistCheckedAt = time.Now()
	return fileAccessWhitelistTypes
}

func fetchFileAccessWhitelist(ctx context.Context) map[string]struct{} {
	whitelist := parseFileAccessWhitelist(ctx)
	types := make(map[string]struct{}, len(whitelist))
	for _, item := range whitelist {
		types[strings.ToLower(item)] = struct{}{}
	}
	return types
}

func parseFileAccessWhitelist(ctx context.Context) []string {
	var sc struct{ Value string }
	db := shared.GetDB(ctx)
	if db != nil {
		_ = db.Table("w_system_configs").Where("key = ?", "file_access_whitelist").First(&sc).Error
	}
	if sc.Value == "" {
		return []string{shared.DefaultPublicUploadType}
	}

	var whitelist []string
	if err := json.Unmarshal([]byte(sc.Value), &whitelist); err == nil && len(whitelist) > 0 {
		return whitelist
	}

	whitelist = parseCommaSeparatedWhitelist(sc.Value)
	if len(whitelist) == 0 {
		return []string{shared.DefaultPublicUploadType}
	}
	return whitelist
}

func parseCommaSeparatedWhitelist(value string) []string {
	parts := strings.Split(value, ",")
	whitelist := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			whitelist = append(whitelist, part)
		}
	}
	return whitelist
}
