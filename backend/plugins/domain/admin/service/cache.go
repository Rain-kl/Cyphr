// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"strconv"

	pkgcache "Wavelet/pkg/cache/disk"
	"Wavelet/plugins/domain/admin/model"
	"Wavelet/plugins/domain/admin/repository"
)

// DiskCacheStatus reports the disk cache usage counters.
func DiskCacheStatus() pkgcache.Status {
	return pkgcache.Default().Status()
}

// ClearDiskCache purges every cached object and resets the tracking counters.
func ClearDiskCache() error {
	return pkgcache.Default().Clear()
}

// UpdateDiskCachePolicy persists the disk cache settings and applies them hot.
func UpdateDiskCachePolicy(ctx context.Context, req model.UpdateCacheConfigRequest) error {
	if err := saveOrUpdateCacheConfig(ctx, model.ConfigKeyDiskCacheMaxSizeMB, strconv.FormatInt(req.MaxSizeMB, 10)); err != nil {
		return err
	}

	if err := saveOrUpdateCacheConfig(ctx, model.ConfigKeyDiskCacheTTLMinutes, strconv.FormatInt(req.TTLMinutes, 10)); err != nil {
		return err
	}

	if err := saveOrUpdateCacheConfig(ctx, model.ConfigKeyDiskCacheLRUEnabled, strconv.FormatBool(req.LRUEnabled)); err != nil {
		return err
	}

	pkgcache.Default().UpdatePolicy(req.MaxSizeMB, req.TTLMinutes, req.LRUEnabled)
	return nil
}

func saveOrUpdateCacheConfig(ctx context.Context, key, value string) error {
	return repository.SaveOrUpdateSystemConfig(ctx, key, value)
}
