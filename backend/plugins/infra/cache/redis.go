// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"Wavelet/pkg/config"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
	"github.com/redis/go-redis/v9/maintnotifications"
	"go.opentelemetry.io/otel/attribute"
)

// Redis 全局 Redis 客户端实例
var Redis redis.UniversalClient

// InitRedis 初始化全局/默认 Redis 客户端实例
func InitRedis() (redis.UniversalClient, error) {
	cfg := config.Config.Redis

	if !cfg.Enabled {
		log.Println("[Redis] is disabled, skipping Redis initialization")
		return nil, nil
	}

	var client redis.UniversalClient

	if cfg.ClusterMode {
		// Cluster 模式
		client = redis.NewClusterClient(&redis.ClusterOptions{
			Addrs:                    cfg.Addrs,
			Username:                 cfg.Username,
			Password:                 cfg.Password,
			PoolSize:                 cfg.PoolSize,
			MinIdleConns:             cfg.MinIdleConn,
			DialTimeout:              time.Duration(cfg.DialTimeout) * time.Second,
			ReadTimeout:              time.Duration(cfg.ReadTimeout) * time.Second,
			WriteTimeout:             time.Duration(cfg.WriteTimeout) * time.Second,
			MaxRetries:               cfg.MaxRetries,
			PoolTimeout:              time.Duration(cfg.PoolTimeout) * time.Second,
			ConnMaxIdleTime:          time.Duration(cfg.ConnMaxIdleTime) * time.Second,
			MaintNotificationsConfig: redisMaintNotificationsConfig(cfg.MaintNotifications),
		})
		log.Println("[Redis] initialized in Cluster mode")
	} else {
		// Standalone 或 Sentinel 模式
		options := &redis.UniversalOptions{
			Addrs:                    cfg.Addrs,
			MasterName:               cfg.MasterName, // 非空时启用 Sentinel
			Username:                 cfg.Username,
			Password:                 cfg.Password,
			DB:                       cfg.DB,
			PoolSize:                 cfg.PoolSize,
			MinIdleConns:             cfg.MinIdleConn,
			DialTimeout:              time.Duration(cfg.DialTimeout) * time.Second,
			ReadTimeout:              time.Duration(cfg.ReadTimeout) * time.Second,
			WriteTimeout:             time.Duration(cfg.WriteTimeout) * time.Second,
			MaxRetries:               cfg.MaxRetries,
			PoolTimeout:              time.Duration(cfg.PoolTimeout) * time.Second,
			ConnMaxIdleTime:          time.Duration(cfg.ConnMaxIdleTime) * time.Second,
			MaintNotificationsConfig: redisMaintNotificationsConfig(cfg.MaintNotifications),
		}
		if cfg.MasterName != "" {
			failoverClient := redis.NewFailoverClient(options.Failover())
			failoverClient.Options().MaintNotificationsConfig = redisMaintNotificationsConfig(cfg.MaintNotifications)
			client = failoverClient
			log.Println("[Redis] initialized in Sentinel mode")
		} else {
			client = redis.NewUniversalClient(options)
			log.Println("[Redis] initialized in Standalone mode")
		}
	}

	// OpenTelemetry 追踪（UniversalClient 兼容）
	if err := redisotel.InstrumentTracing(
		client,
		redisotel.WithAttributes(
			attribute.String("db.instance", fmt.Sprintf("%v", cfg.DB)),
			attribute.String("db.ip", strings.Join(cfg.Addrs, ",")),
			attribute.String("db.system", "Redis"),
		),
	); err != nil {
		return nil, fmt.Errorf("redis: init trace: %w", err)
	}

	// 测试连接
	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("redis: ping: %w", err)
	}

	Redis = client
	return client, nil
}

// SetRedisClient 设置包级 Redis 客户端（主要用于测试）
func SetRedisClient(client redis.UniversalClient) {
	Redis = client
}

func redisMaintNotificationsConfig(enabled bool) *maintnotifications.Config {
	mode := maintnotifications.ModeDisabled
	if enabled {
		mode = maintnotifications.ModeAuto
	}
	return &maintnotifications.Config{Mode: mode}
}

// PrefixedKey 返回带前缀的 Key
func PrefixedKey(key string) string {
	prefix := config.Config.Redis.KeyPrefix
	if prefix == "" {
		return key
	}
	return prefix + key
}

// HSetJSON 将泛型数据序列化为 JSON 并设置到 Redis Hash
// ctx: 上下文
// hashKey: Redis Hash key
// fieldKey: Hash field key
// data: 要存储的数据（泛型）
func HSetJSON[T any](ctx context.Context, hashKey, fieldKey string, data T) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	if err := Redis.HSet(ctx, PrefixedKey(hashKey), fieldKey, jsonData).Err(); err != nil {
		return fmt.Errorf(errRedisHashSetFailed, err)
	}

	return nil
}

// HDel removes one or more fields from a Redis Hash.
func HDel(ctx context.Context, hashKey string, fieldKeys ...string) error {
	if Redis == nil || len(fieldKeys) == 0 {
		return nil
	}
	if err := Redis.HDel(ctx, PrefixedKey(hashKey), fieldKeys...).Err(); err != nil {
		return fmt.Errorf(errRedisHashDeleteFailed, err)
	}
	return nil
}

// HGetJSON 从 Redis Hash 获取数据并反序列化为泛型类型
// ctx: 上下文
// hashKey: Redis Hash key
// fieldKey: Hash field key
// data: 用于接收数据的指针（泛型）
func HGetJSON[T any](ctx context.Context, hashKey, fieldKey string, data *T) error {
	val, err := Redis.HGet(ctx, PrefixedKey(hashKey), fieldKey).Result()
	if err != nil {
		return err
	}

	if err := json.Unmarshal([]byte(val), data); err != nil {
		return fmt.Errorf(errUnmarshalDataFailed, err)
	}

	return nil
}

// GetJSON 从Redis获取数据并反序列化为泛型类型
// ctx: 上下文
// key: Redis key
// data: 用于接收数据的指针（泛型）
func GetJSON[T any](ctx context.Context, key string, data *T) error {
	val, err := Redis.Get(ctx, PrefixedKey(key)).Bytes()
	if err != nil {
		return err
	}

	if err := json.Unmarshal(val, data); err != nil {
		return fmt.Errorf(errUnmarshalDataFailed, err)
	}

	return nil
}

// SetJSON 将泛型数据序列化为JSON并设置到Redis
// ctx: 上下文
// key: Redis key
// data: 要存储的数据（泛型）
// expiration: 过期时间
func SetJSON[T any](ctx context.Context, key string, data T, expiration time.Duration) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf(errMarshalDataFailed, err)
	}

	if err := Redis.Set(ctx, PrefixedKey(key), jsonData, expiration).Err(); err != nil {
		return fmt.Errorf(errRedisKeySetFailed, err)
	}

	return nil
}
