// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package cache provides in-process upload access-control caches.
package cache

import (
	"Wavelet/pkg/logger"
	"Wavelet/plugins/domain/upload/shared"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"

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

	types, err := fetchFileAccessWhitelist(ctx)
	if err != nil {
		logger.ErrorF(ctx, "[Upload] 读取公开访问白名单失败: %v", err)
		if fileAccessWhitelistTypes != nil {
			// A previously loaded list is still the best answer; keep it until its
			// TTL rather than narrowing access because one read failed.
			fileAccessWhitelistValid = true
			fileAccessWhitelistCheckedAt = time.Now()
			return fileAccessWhitelistTypes
		}
		// Nothing cached yet: serve the restricted default but stay invalid so the
		// next request retries instead of pinning it for the whole TTL.
		return fallbackFileAccessWhitelist()
	}

	fileAccessWhitelistTypes = types
	fileAccessWhitelistValid = true
	fileAccessWhitelistCheckedAt = time.Now()
	return types
}

// fallbackFileAccessWhitelist is the restricted default used when nothing better is known.
func fallbackFileAccessWhitelist() map[string]struct{} {
	return map[string]struct{}{strings.ToLower(shared.DefaultPublicUploadType): {}}
}

func fetchFileAccessWhitelist(ctx context.Context) (map[string]struct{}, error) {
	whitelist, err := parseFileAccessWhitelist(ctx)
	if err != nil {
		return nil, err
	}
	types := make(map[string]struct{}, len(whitelist))
	for _, item := range whitelist {
		types[strings.ToLower(item)] = struct{}{}
	}
	return types, nil
}

// parseFileAccessWhitelist reads the configured public access types.
//
// A missing row or an empty value is a real answer and yields the default; only a
// read that actually fails is reported as an error, so a database outage is no
// longer mistaken for "the admin never configured a whitelist".
func parseFileAccessWhitelist(ctx context.Context) ([]string, error) {
	var sc struct{ Value string }
	db := shared.GetDB(ctx)
	if db == nil {
		return nil, errors.New("database not available")
	}
	if err := db.Table("w_system_configs").Where("key = ?", "file_access_whitelist").First(&sc).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return []string{shared.DefaultPublicUploadType}, nil
		}
		return nil, fmt.Errorf("read file_access_whitelist: %w", err)
	}
	if sc.Value == "" {
		return []string{shared.DefaultPublicUploadType}, nil
	}

	var whitelist []string
	if err := json.Unmarshal([]byte(sc.Value), &whitelist); err == nil && len(whitelist) > 0 {
		return whitelist, nil
	}

	whitelist = parseCommaSeparatedWhitelist(sc.Value)
	if len(whitelist) == 0 {
		return []string{shared.DefaultPublicUploadType}, nil
	}
	return whitelist, nil
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
