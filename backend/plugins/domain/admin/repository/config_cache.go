// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"Wavelet/pkg/cache/ram"
	"Wavelet/plugins/domain/admin/model"
	"context"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"
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

// LoadOne loads a single system config from database as CacheItem.
func (ConfigLoader) LoadOne(ctx context.Context, configType, key string) (ram.CacheItem, error) {
	cfg, err := GetSystemConfigByKey(ctx, key)
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

// GetCachedSystemConfig retrieves a single system config with RAM L1 fallback to DB.
func GetCachedSystemConfig(ctx context.Context, key string) (*model.SystemConfig, error) {
	if item, ok := ram.Get(ConfigCacheType, key); ok {
		var cfg model.SystemConfig
		if err := json.Unmarshal([]byte(item.Value), &cfg); err == nil {
			return &cfg, nil
		}
	}

	cfg, err := GetSystemConfigByKey(ctx, key)
	if err != nil {
		return nil, err
	}

	valBytes, err := json.Marshal(cfg)
	if err == nil {
		ram.Set(ram.CacheItem{
			Key:   cfg.Key,
			Value: string(valBytes),
			Type:  ConfigCacheType,
			TTL:   determineTTL(key),
		})
	}
	return &cfg, nil
}

// StopSystemConfigCacheListener stops the cache invalidation listener (kept for backward compatibility).
func StopSystemConfigCacheListener() {
}

// StartSystemConfigCacheListener starts the cache listener (kept for backward compatibility).
func StartSystemConfigCacheListener() {
}

func ensureSystemConfigCacheListener() {
}

func determineTTL(_ string) time.Duration {
	return -1
}

// InvalidateSystemConfigCache triggers a broadcast to refresh the cache for key.
func InvalidateSystemConfigCache(ctx context.Context, key string) error {
	ram.Delete(ConfigCacheType, key)
	if cacheSvc := GetCache(ctx); cacheSvc != nil {
		_ = cacheSvc.Delete(ctx, "system:config:"+key)
		_ = cacheSvc.Delete(ctx, SystemConfigVisibleListRedisKey)
	}
	return nil
}

// InvalidateAllSystemConfigCaches triggers a broadcast to refresh the entire config cache.
func InvalidateAllSystemConfigCaches(ctx context.Context) error {
	ram.UpdateTypeItems(ConfigCacheType, nil)
	if cacheSvc := GetCache(ctx); cacheSvc != nil {
		_ = cacheSvc.Delete(ctx, SystemConfigRedisHashKey)
		_ = cacheSvc.Delete(ctx, SystemConfigVisibleListRedisKey)
	}
	return nil
}

// ResetSystemConfigRAMCacheForTest clears only the process-local RAM cache.
func ResetSystemConfigRAMCacheForTest() {
	ram.ResetForTest()
}
