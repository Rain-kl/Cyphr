// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package admin

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"gorm.io/gorm"

	"github.com/Rain-kl/Wavelet/backend/pkg/cache/ram"
	"github.com/Rain-kl/Wavelet/backend/pkg/util"
	cachepkg "github.com/Rain-kl/Wavelet/backend/plugins/infra/cache"
)

const (
	// SystemConfigBroadcastChannel broadcasts system config cache updates across nodes.
	SystemConfigBroadcastChannel = "system:config_broadcast"

	// SystemConfigInvalidationChannel is kept as an alias for backward compatibility.
	SystemConfigInvalidationChannel = SystemConfigBroadcastChannel

	// SystemConfigRedisHashKey is kept for backward compatibility in tests.
	SystemConfigRedisHashKey = "system:system_configs"
	// SystemConfigVisibleListRedisKey is kept for backward compatibility in tests.
	SystemConfigVisibleListRedisKey = "system:visible_configs"

	// ConfigCacheType is the cache type for all system configs.
	ConfigCacheType = "config"
)

type systemConfigBroadcastMessage struct {
	Type string `json:"type"`
	Key  string `json:"key"`
}

// ConfigLoader loads configuration data from the database.
type ConfigLoader struct{}

// LoadAll loads all system configs from database as CacheItems.
func (ConfigLoader) LoadAll(ctx context.Context, configType string) ([]ram.CacheItem, error) {
	configs, err := PreheatSystemConfigs(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]ram.CacheItem, len(configs))
	for i, cfg := range configs {
		valBytes, err := json.Marshal(cfg)
		if err != nil {
			return nil, err
		}
		items[i] = ram.CacheItem{
			Key:   cfg.Key,
			Value: string(valBytes),
			Type:  configType,
			TTL:   determineTTL(cfg.Key),
		}
	}
	return items, nil
}

// LoadOne loads a single system config from database as a CacheItem.
func (ConfigLoader) LoadOne(ctx context.Context, configType string, key string) (ram.CacheItem, error) {
	cfg, err := PreheatSystemConfigByKey(ctx, key)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ram.CacheItem{}, ram.ErrNotFound
		}
		return ram.CacheItem{}, err
	}

	valBytes, err := json.Marshal(cfg)
	if err != nil {
		return ram.CacheItem{}, err
	}

	return ram.CacheItem{
		Key:   cfg.Key,
		Value: string(valBytes),
		Type:  configType,
		TTL:   determineTTL(cfg.Key),
	}, nil
}

// PreloadSystemConfigs warms the in-memory RAM cache from database on startup.
func PreloadSystemConfigs(ctx context.Context) error {
	return ram.Refresh(ctx, ConfigCacheType, "", ConfigLoader{})
}

var (
	systemConfigListenerOnce   sync.Once
	systemConfigListenerCtx    context.Context
	systemConfigListenerCancel context.CancelFunc
	systemConfigListenerDone   chan struct{}
)

func ensureSystemConfigCacheListener() {
	systemConfigListenerOnce.Do(startSystemConfigCacheInvalidationListener)
}

func startSystemConfigCacheInvalidationListener() {
	if cachepkg.Redis == nil {
		return
	}

	systemConfigListenerCtx, systemConfigListenerCancel = context.WithCancel(context.Background())
	systemConfigListenerDone = make(chan struct{})

	redisClient := cachepkg.Redis // 捕获当前客户端：goroutine 不读可变全局，避免与测试置空 cachepkg.Redis 竞争
	util.Go(func() {
		listenerCtx := systemConfigListenerCtx
		defer close(systemConfigListenerDone)

		pubsub := redisClient.Subscribe(listenerCtx, SystemConfigBroadcastChannel)
		defer func() {
			_ = pubsub.Close()
		}()

		util.Go(func() {
			<-listenerCtx.Done()
			_ = pubsub.Close()
		})

		for msg := range pubsub.Channel() {
			var payload systemConfigBroadcastMessage
			if err := json.Unmarshal([]byte(msg.Payload), &payload); err != nil {
				ram.UpdateTypeItems(ConfigCacheType, nil)
				continue
			}

			key := payload.Key
			if key == "*" || key == "" {
				ram.UpdateTypeItems(payload.Type, nil)
			} else {
				ram.Delete(payload.Type, key)
			}
		}
	})
}

// StopSystemConfigCacheListener stops the Redis Pub/Sub subscription listener and resets the sync.Once guard.
func StopSystemConfigCacheListener() {
	if systemConfigListenerCancel != nil {
		systemConfigListenerCancel()
		if systemConfigListenerDone != nil {
			<-systemConfigListenerDone
		}
		systemConfigListenerCancel = nil
		systemConfigListenerDone = nil
	}
	systemConfigListenerOnce = sync.Once{}
}

func determineTTL(_ string) time.Duration {
	// Program-determined TTL: -1 means never expire for all configs by default
	return -1
}

// InvalidateSystemConfigCache triggers a broadcast to refresh the cache for key.
func InvalidateSystemConfigCache(ctx context.Context, key string) error {
	ensureSystemConfigCacheListener()

	// Invalidate local cache synchronously first
	ram.Delete(ConfigCacheType, key)

	// Broadcast to other nodes and clean legacy Redis cache key
	if cachepkg.Redis != nil {
		_ = cachepkg.HDel(ctx, SystemConfigRedisHashKey, key)
		publishSystemConfigBroadcast(ctx, ConfigCacheType, key)
	}
	return nil
}

// InvalidateAllSystemConfigCaches triggers a broadcast to refresh the entire config cache.
func InvalidateAllSystemConfigCaches(ctx context.Context) error {
	ensureSystemConfigCacheListener()

	// Invalidate all items of type ConfigCacheType synchronously first
	ram.UpdateTypeItems(ConfigCacheType, nil)

	// Broadcast to other nodes and clean legacy Redis cache keys
	if cachepkg.Redis != nil {
		_ = cachepkg.Redis.Del(ctx, cachepkg.PrefixedKey(SystemConfigRedisHashKey), cachepkg.PrefixedKey(SystemConfigVisibleListRedisKey)).Err()
		publishSystemConfigBroadcast(ctx, ConfigCacheType, "*")
	}
	return nil
}

func publishSystemConfigBroadcast(ctx context.Context, configType string, key string) {
	if cachepkg.Redis == nil {
		return
	}
	payload, err := json.Marshal(systemConfigBroadcastMessage{Type: configType, Key: key})
	if err != nil {
		return
	}
	_ = cachepkg.Redis.Publish(ctx, SystemConfigBroadcastChannel, payload).Err()
}

// ResetSystemConfigRAMCacheForTest clears only the process-local RAM cache.
func ResetSystemConfigRAMCacheForTest() {
	ram.ResetForTest()
}
