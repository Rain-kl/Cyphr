// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package auth

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"Wavelet/core/contracts"
	"Wavelet/pkg/cache/ram"
	"Wavelet/pkg/util"
	db "Wavelet/plugins/infra/cache"
)

const (
	tokenCacheTTL = 5 * time.Minute
	userCacheTTL  = 5 * time.Minute

	//nolint:gosec // This is a Redis Pub/Sub channel name, not a credential
	oauthTokenInvalidationChannel = "oauth:token_invalidation"
	oauthUserInvalidationChannel  = "oauth:user_invalidation"
)

// CachedToken represents the minimal cached representation of an access token.
type CachedToken struct {
	ID      uint64 `json:"id"`
	UserID  uint64 `json:"user_id"`
	IsAdmin bool   `json:"is_admin"`
}

var (
	tokenRAM = ram.MustNew[string, *CachedToken](ram.Options{MaximumSize: 2048})
	userRAM  = ram.MustNew[uint64, *contracts.UserDTO](ram.Options{MaximumSize: 2048})

	tokenListenerOnce   sync.Once
	tokenListenerCtx    context.Context
	tokenListenerCancel context.CancelFunc
	tokenListenerDone   chan struct{}

	userListenerOnce   sync.Once
	userListenerCtx    context.Context
	userListenerCancel context.CancelFunc
	userListenerDone   chan struct{}
)

func tokenCacheKey(tokenHash string) string {
	return "oauth:token:" + tokenHash
}

func userCacheKey(userID uint64) string {
	return fmt.Sprintf("oauth:user:%d", userID)
}

func ensureTokenCacheListener() {
	if db.Redis == nil {
		return
	}
	tokenListenerOnce.Do(startTokenCacheInvalidationListener)
}

func startTokenCacheInvalidationListener() {
	tokenListenerCtx, tokenListenerCancel = context.WithCancel(context.Background())
	tokenListenerDone = make(chan struct{})

	redisClient := db.Redis // 捕获当前客户端：goroutine 不读可变全局，避免与测试置空 db.Redis 竞争
	util.Go(func() {
		listenerCtx := tokenListenerCtx
		defer close(tokenListenerDone)

		pubsub := redisClient.Subscribe(listenerCtx, oauthTokenInvalidationChannel)
		defer func() {
			_ = pubsub.Close()
		}()

		util.Go(func() {
			<-listenerCtx.Done()
			_ = pubsub.Close()
		})

		for msg := range pubsub.Channel() {
			tokenHash := msg.Payload
			if tokenHash == "" || tokenHash == "*" || tokenHash == "reset" {
				tokenRAM.InvalidateAll()
			} else {
				tokenRAM.Invalidate(tokenHash)
			}
		}
	})
}

func publishTokenRAMInvalidation(ctx context.Context, tokenHash string) {
	if db.Redis == nil {
		return
	}
	_ = db.Redis.Publish(ctx, oauthTokenInvalidationChannel, tokenHash).Err()
}

func ensureUserCacheListener() {
	if db.Redis == nil {
		return
	}
	userListenerOnce.Do(startUserCacheInvalidationListener)
}

func startUserCacheInvalidationListener() {
	userListenerCtx, userListenerCancel = context.WithCancel(context.Background())
	userListenerDone = make(chan struct{})

	redisClient := db.Redis // 捕获当前客户端：goroutine 不读可变全局，避免与测试置空 db.Redis 竞争
	util.Go(func() {
		listenerCtx := userListenerCtx
		defer close(userListenerDone)

		pubsub := redisClient.Subscribe(listenerCtx, oauthUserInvalidationChannel)
		defer func() {
			_ = pubsub.Close()
		}()

		util.Go(func() {
			<-listenerCtx.Done()
			_ = pubsub.Close()
		})

		for msg := range pubsub.Channel() {
			userIDStr := msg.Payload
			if userIDStr == "" || userIDStr == "*" || userIDStr == "reset" {
				userRAM.InvalidateAll()
			} else if userID, err := strconv.ParseUint(userIDStr, 10, 64); err == nil {
				userRAM.Invalidate(userID)
			}
		}
	})
}

func publishUserRAMInvalidation(ctx context.Context, userID uint64) {
	if db.Redis == nil {
		return
	}
	_ = db.Redis.Publish(ctx, oauthUserInvalidationChannel, strconv.FormatUint(userID, 10)).Err()
}

// GetCachedToken 获取缓存的 Token
func GetCachedToken(ctx context.Context, tokenHash string) (*CachedToken, error) {
	ensureTokenCacheListener()

	if val, ok := tokenRAM.GetIfPresent(tokenHash); ok {
		return val, nil
	}

	if db.Redis != nil {
		var token CachedToken
		key := tokenCacheKey(tokenHash)
		if err := db.GetJSON(ctx, key, &token); err == nil {
			// Write back to local cache
			tokenRAM.Set(tokenHash, &token)
			return &token, nil
		}
	}
	return nil, fmt.Errorf("cache miss")
}

// SetCachedToken 设置 Token 缓存
func SetCachedToken(ctx context.Context, tokenHash string, token *CachedToken) {
	ensureTokenCacheListener()

	tokenRAM.Set(tokenHash, token)
	if db.Redis != nil {
		key := tokenCacheKey(tokenHash)
		_ = db.SetJSON(ctx, key, token, tokenCacheTTL)
	}
}

// InvalidateCachedToken 吊销/删除 token 缓存
func InvalidateCachedToken(ctx context.Context, tokenHash string) {
	ensureTokenCacheListener()

	tokenRAM.Invalidate(tokenHash)
	if db.Redis != nil {
		key := tokenCacheKey(tokenHash)
		_ = db.Redis.Del(ctx, db.PrefixedKey(key)).Err()
		publishTokenRAMInvalidation(ctx, tokenHash)
	}
}

// GetCachedUser 获取缓存的 UserDTO
func GetCachedUser(ctx context.Context, userID uint64) (*contracts.UserDTO, error) {
	ensureUserCacheListener()

	if val, ok := userRAM.GetIfPresent(userID); ok {
		return val, nil
	}

	if db.Redis != nil {
		var u contracts.UserDTO
		key := userCacheKey(userID)
		if err := db.GetJSON(ctx, key, &u); err == nil {
			// Write back to local cache
			userRAM.Set(userID, &u)
			return &u, nil
		}
	}
	return nil, fmt.Errorf("cache miss")
}

// SetCachedUser 设置 UserDTO 缓存
func SetCachedUser(ctx context.Context, userID uint64, u *contracts.UserDTO) {
	ensureUserCacheListener()

	userRAM.Set(userID, u)
	if db.Redis != nil {
		key := userCacheKey(userID)
		_ = db.SetJSON(ctx, key, u, userCacheTTL)
	}
}

// InvalidateCachedUser 吊销/失效 UserDTO 缓存
func InvalidateCachedUser(ctx context.Context, userID uint64) {
	ensureUserCacheListener()

	userRAM.Invalidate(userID)
	if db.Redis != nil {
		key := userCacheKey(userID)
		_ = db.Redis.Del(ctx, db.PrefixedKey(key)).Err()
		publishUserRAMInvalidation(ctx, userID)
	}
}

// StopAuthCacheListener stops both token and user Redis Pub/Sub subscription listeners and resets the sync.Once guards.
func StopAuthCacheListener() {
	if tokenListenerCancel != nil {
		tokenListenerCancel()
		if tokenListenerDone != nil {
			<-tokenListenerDone
		}
		tokenListenerCancel = nil
		tokenListenerDone = nil
	}
	tokenListenerOnce = sync.Once{}

	if userListenerCancel != nil {
		userListenerCancel()
		if userListenerDone != nil {
			<-userListenerDone
		}
		userListenerCancel = nil
		userListenerDone = nil
	}
	userListenerOnce = sync.Once{}
}

// ResetAuthRAMCacheForTest clears only the process-local RAM cache.
func ResetAuthRAMCacheForTest() {
	tokenRAM.InvalidateAll()
	userRAM.InvalidateAll()
}
