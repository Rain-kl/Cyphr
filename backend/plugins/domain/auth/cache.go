// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package auth

import (
	"Wavelet/core/contracts"
	"Wavelet/pkg/cache/ram"
	"context"
	"fmt"
	"time"
)

const (
	tokenCacheTTL = 5 * time.Minute
	userCacheTTL  = 5 * time.Minute
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
)

func tokenCacheKey(tokenHash string) string {
	return "oauth:token:" + tokenHash
}

func userCacheKey(userID uint64) string {
	return fmt.Sprintf("oauth:user:%d", userID)
}

// GetCachedToken 获取缓存的 Token
func GetCachedToken(ctx context.Context, tokenHash string) (*CachedToken, error) {
	if val, ok := tokenRAM.GetIfPresent(tokenHash); ok {
		return val, nil
	}

	if cache := getCache(ctx); cache != nil {
		var token CachedToken
		key := tokenCacheKey(tokenHash)
		if err := cache.Get(ctx, key, &token); err == nil {
			tokenRAM.Set(tokenHash, &token)
			return &token, nil
		}
	}
	return nil, fmt.Errorf("cache miss")
}

// SetCachedToken 设置 Token 缓存
func SetCachedToken(ctx context.Context, tokenHash string, token *CachedToken) {
	tokenRAM.Set(tokenHash, token)
	if cache := getCache(ctx); cache != nil {
		key := tokenCacheKey(tokenHash)
		_ = cache.Set(ctx, key, token, tokenCacheTTL)
	}
}

// InvalidateCachedToken 吊销/删除 token 缓存
func InvalidateCachedToken(ctx context.Context, tokenHash string) {
	tokenRAM.Invalidate(tokenHash)
	if cache := getCache(ctx); cache != nil {
		key := tokenCacheKey(tokenHash)
		_ = cache.Delete(ctx, key)
	}
}

// GetCachedUser 获取缓存的 UserDTO
func GetCachedUser(ctx context.Context, userID uint64) (*contracts.UserDTO, error) {
	if val, ok := userRAM.GetIfPresent(userID); ok {
		return val, nil
	}

	if cache := getCache(ctx); cache != nil {
		var u contracts.UserDTO
		key := userCacheKey(userID)
		if err := cache.Get(ctx, key, &u); err == nil {
			userRAM.Set(userID, &u)
			return &u, nil
		}
	}
	return nil, fmt.Errorf("cache miss")
}

// SetCachedUser 设置 UserDTO 缓存
func SetCachedUser(ctx context.Context, userID uint64, u *contracts.UserDTO) {
	userRAM.Set(userID, u)
	if cache := getCache(ctx); cache != nil {
		key := userCacheKey(userID)
		_ = cache.Set(ctx, key, u, userCacheTTL)
	}
}

// InvalidateCachedUser 吊销/失效 UserDTO 缓存
func InvalidateCachedUser(ctx context.Context, userID uint64) {
	userRAM.Invalidate(userID)
	if cache := getCache(ctx); cache != nil {
		key := userCacheKey(userID)
		_ = cache.Delete(ctx, key)
	}
}

// StopAuthCacheListener compatibility stub for tests
func StopAuthCacheListener() {}

// ResetAuthRAMCacheForTest clears only the process-local RAM cache.
func ResetAuthRAMCacheForTest() {
	tokenRAM.InvalidateAll()
	userRAM.InvalidateAll()
}
