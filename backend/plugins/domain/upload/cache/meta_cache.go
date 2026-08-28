// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/Rain-kl/Wavelet/pkg/cache/ram"
	"github.com/Rain-kl/Wavelet/pkg/util"
	"github.com/Rain-kl/Wavelet/plugins/domain/upload/models"
	cachepkg "github.com/Rain-kl/Wavelet/plugins/infra/cache"
	database "github.com/Rain-kl/Wavelet/plugins/infra/database"
)

const (
	uploadMetaRedisCacheTTL    = 30 * 60 // seconds
	uploadMetaRAMMaximumSize   = 4096
	uploadMetaInvalidationChan = "upload:meta_invalidation"
)

type uploadMetaInvalidationMessage struct {
	ID uint64 `json:"id"`
}

var (
	uploadMetaRAM            = ram.MustNew[uint64, models.Upload](ram.Options{MaximumSize: uploadMetaRAMMaximumSize})
	uploadMetaListenerOnce   sync.Once
	uploadMetaListenerCtx    context.Context
	uploadMetaListenerCancel context.CancelFunc
	uploadMetaListenerDone   chan struct{}
)

func uploadMetaRedisKey(id uint64) string {
	return fmt.Sprintf("upload:meta:%d", id)
}

func cloneUpload(u models.Upload) models.Upload {
	return u
}

func ensureUploadMetaCacheListener() {
	if cachepkg.Redis == nil {
		return
	}
	uploadMetaListenerOnce.Do(startUploadMetaCacheInvalidationListener)
}

func startUploadMetaCacheInvalidationListener() {
	uploadMetaListenerCtx, uploadMetaListenerCancel = context.WithCancel(context.Background())
	uploadMetaListenerDone = make(chan struct{})

	redisClient := cachepkg.Redis // 捕获当前客户端：goroutine 不读可变全局，避免与测试置空 cachepkg.Redis 竞争
	util.Go(func() {
		defer close(uploadMetaListenerDone)
		pubsub := redisClient.Subscribe(uploadMetaListenerCtx, uploadMetaInvalidationChan)
		defer func() {
			_ = pubsub.Close()
		}()

		util.Go(func() {
			<-uploadMetaListenerCtx.Done()
			_ = pubsub.Close()
		})

		for msg := range pubsub.Channel() {
			var payload uploadMetaInvalidationMessage
			if err := json.Unmarshal([]byte(msg.Payload), &payload); err != nil || payload.ID == 0 {
				uploadMetaRAM.InvalidateAll()
				continue
			}
			uploadMetaRAM.Invalidate(payload.ID)
		}
	})
}

func publishUploadMetaRAMInvalidation(ctx context.Context, id uint64) {
	if cachepkg.Redis == nil {
		return
	}
	payload, err := json.Marshal(uploadMetaInvalidationMessage{ID: id})
	if err != nil {
		return
	}
	_ = cachepkg.Redis.Publish(ctx, uploadMetaInvalidationChan, payload).Err()
}

// GetUploadByID loads upload metadata from RAM, Redis, or the database.
func GetUploadByID(ctx context.Context, id uint64) (models.Upload, error) {
	ensureUploadMetaCacheListener()

	if u, ok := uploadMetaRAM.GetIfPresent(id); ok {
		return cloneUpload(u), nil
	}

	key := uploadMetaRedisKey(id)
	if cachepkg.Redis != nil {
		var u models.Upload
		if err := cachepkg.GetJSON(ctx, key, &u); err == nil {
			uploadMetaRAM.Set(id, cloneUpload(u))
			return u, nil
		}
	}

	var u models.Upload
	if err := database.DB(ctx).
		Where("id = ? AND status IN (?, ?)", id, models.UploadStatusPending, models.UploadStatusUsed).
		First(&u).Error; err != nil {
		return models.Upload{}, err
	}

	SetUploadMetaCache(ctx, &u)
	return u, nil
}

// SetUploadMetaCache populates RAM and Redis upload metadata caches.
func SetUploadMetaCache(ctx context.Context, u *models.Upload) {
	ensureUploadMetaCacheListener()

	if u == nil {
		return
	}

	cloned := cloneUpload(*u)
	uploadMetaRAM.Set(u.ID, cloned)
	if cachepkg.Redis != nil {
		_ = cachepkg.SetJSON(ctx, uploadMetaRedisKey(u.ID), cloned, uploadMetaRedisCacheTTL)
	}
}

// InvalidateUploadMetaCache clears RAM and Redis upload metadata caches and notifies peer nodes.
func InvalidateUploadMetaCache(ctx context.Context, id uint64) {
	ensureUploadMetaCacheListener()

	uploadMetaRAM.Invalidate(id)
	if cachepkg.Redis != nil {
		_ = cachepkg.Redis.Del(ctx, cachepkg.PrefixedKey(uploadMetaRedisKey(id))).Err()
		publishUploadMetaRAMInvalidation(ctx, id)
	}
}

// ResetUploadMetaCacheForTest clears the in-process upload metadata RAM cache.
func ResetUploadMetaCacheForTest() {
	uploadMetaRAM.InvalidateAll()
}

// StopUploadMetaCacheListener stops the Redis Pub/Sub subscription listener and resets the sync.Once guard.
func StopUploadMetaCacheListener() {
	if uploadMetaListenerCancel != nil {
		uploadMetaListenerCancel()
		if uploadMetaListenerDone != nil {
			<-uploadMetaListenerDone // 等待 goroutine 退出，保证之后置空 cachepkg.Redis 不再竞争
		}
		uploadMetaListenerCancel = nil
		uploadMetaListenerDone = nil
	}
	uploadMetaListenerOnce = sync.Once{}
}
