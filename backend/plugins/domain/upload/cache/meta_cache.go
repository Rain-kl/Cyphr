// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"Wavelet/pkg/cache/ram"
	"Wavelet/plugins/domain/upload/models"
	"Wavelet/plugins/domain/upload/shared"
)

const (
	uploadMetaRedisCacheTTL    = 30 * 60 // seconds
	uploadMetaRAMMaximumSize   = 4096
	uploadMetaInvalidationChan = "upload:meta_invalidation"
)

var (
	uploadMetaRAM = ram.MustNew[uint64, models.Upload](ram.Options{MaximumSize: uploadMetaRAMMaximumSize})
)

func uploadMetaRedisKey(id uint64) string {
	return fmt.Sprintf("upload:meta:%d", id)
}

func cloneUpload(u models.Upload) models.Upload {
	return u
}

// PublishUploadMetaInvalidation broadcasts upload metadata cache eviction.
func PublishUploadMetaInvalidation(ctx context.Context, id uint64) {
	if cache := shared.GetCache(ctx); cache != nil {
		_ = cache.Invalidate(ctx, uploadMetaInvalidationChan)
	}
	EvictUploadMetaLocal(id)
}

// GetUploadByID loads upload metadata from RAM, Redis, or the database.
func GetUploadByID(ctx context.Context, id uint64) (models.Upload, error) {
	if id == 0 {
		return models.Upload{}, gorm.ErrRecordNotFound
	}

	// 1. RAM L1 Cache
	if u, ok := uploadMetaRAM.GetIfPresent(id); ok {
		return cloneUpload(u), nil
	}

	key := uploadMetaRedisKey(id)

	// 2. Redis L2 Cache
	if cache := shared.GetCache(ctx); cache != nil {
		var u models.Upload
		if err := cache.Get(ctx, key, &u); err == nil {
			uploadMetaRAM.Set(id, u)
			return cloneUpload(u), nil
		}
	}

	// 3. Database L3 Source of Truth
	var upload models.Upload
	db := shared.GetDB(ctx)
	if db == nil {
		return models.Upload{}, gorm.ErrRecordNotFound
	}
	if err := db.
		Where("id = ? AND status != ?", id, models.UploadStatusDeleted).
		First(&upload).Error; err != nil {
		return models.Upload{}, err
	}

	SetUploadMeta(ctx, upload)
	return cloneUpload(upload), nil
}

// SetUploadMeta populates RAM and Redis caches with the provided upload metadata.
func SetUploadMeta(ctx context.Context, u models.Upload) {
	if u.ID == 0 {
		return
	}
	cloned := cloneUpload(u)
	uploadMetaRAM.Set(u.ID, cloned)

	if cache := shared.GetCache(ctx); cache != nil {
		_ = cache.Set(ctx, uploadMetaRedisKey(u.ID), cloned, uploadMetaRedisCacheTTL*time.Second)
	}
}

// EvictUploadMeta evicts an upload metadata record from RAM, Redis, and broadcasts eviction.
func EvictUploadMeta(ctx context.Context, id uint64) {
	EvictUploadMetaLocal(id)

	if cache := shared.GetCache(ctx); cache != nil {
		_ = cache.Delete(ctx, uploadMetaRedisKey(id))
	}

	PublishUploadMetaInvalidation(ctx, id)
}

// EvictUploadMetaLocal removes upload metadata from the local process RAM cache only.
func EvictUploadMetaLocal(id uint64) {
	uploadMetaRAM.Invalidate(id)
}

// ResetUploadMetaCache cleans up local memory cache.
func ResetUploadMetaCache() {
	uploadMetaRAM.InvalidateAll()
}

// ResetUploadMetaCacheForTest clears the in-memory cache for tests.
func ResetUploadMetaCacheForTest() {
	ResetUploadMetaCache()
}

// SetUploadMetaCache is a backward-compatible alias for SetUploadMeta.
func SetUploadMetaCache(ctx context.Context, u *models.Upload) {
	if u != nil {
		SetUploadMeta(ctx, *u)
	}
}

// InvalidateUploadMetaCache is an alias for EvictUploadMeta.
func InvalidateUploadMetaCache(ctx context.Context, id uint64) {
	EvictUploadMeta(ctx, id)
}

// StopUploadMetaCacheListener stops listener for tests.
func StopUploadMetaCacheListener() {}
