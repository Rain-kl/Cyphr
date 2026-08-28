// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package auth_test

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/redis/go-redis/v9/maintnotifications"

	"github.com/Rain-kl/Wavelet/backend/core/contracts"
	"github.com/Rain-kl/Wavelet/backend/plugins/domain/auth"
	db "github.com/Rain-kl/Wavelet/backend/plugins/infra/cache"
)

func setupOauthCacheTest(t *testing.T) (*miniredis.Miniredis, func()) {
	t.Helper()

	miniRedis, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}

	db.Redis = redis.NewClient(&redis.Options{
		Addr: miniRedis.Addr(),
		MaintNotificationsConfig: &maintnotifications.Config{
			Mode: maintnotifications.ModeDisabled,
		},
	})

	auth.ResetAuthRAMCacheForTest()

	cleanup := func() {
		auth.StopAuthCacheListener()
		auth.ResetAuthRAMCacheForTest()
		_ = db.Redis.Close()
		miniRedis.Close()
		db.Redis = nil
	}
	return miniRedis, cleanup
}

func TestTokenCache_GetSetInvalidate(t *testing.T) {
	_, cleanup := setupOauthCacheTest(t)
	defer cleanup()
	ctx := context.Background()

	tokenHash := "test-token-hash"
	token := &auth.CachedToken{
		ID:      123,
		UserID:  456,
		IsAdmin: true,
	}

	// 1. Get from empty cache -> miss
	_, err := auth.GetCachedToken(ctx, tokenHash)
	if err == nil {
		t.Fatal("expected cache miss for un-cached token")
	}

	// 2. Set to cache
	auth.SetCachedToken(ctx, tokenHash, token)

	// 3. Get from cache -> hit
	cached, err := auth.GetCachedToken(ctx, tokenHash)
	if err != nil {
		t.Fatalf("GetCachedToken() failed: %v", err)
	}
	if cached.ID != token.ID || cached.UserID != token.UserID || cached.IsAdmin != token.IsAdmin {
		t.Fatalf("expected cached token %+v, got %+v", token, cached)
	}

	// 4. Invalidate cache
	auth.InvalidateCachedToken(ctx, tokenHash)

	// 5. Get from cache -> miss
	_, err = auth.GetCachedToken(ctx, tokenHash)
	if err == nil {
		t.Fatal("expected cache miss after invalidation")
	}
}

func TestUserCache_GetSetInvalidate(t *testing.T) {
	_, cleanup := setupOauthCacheTest(t)
	defer cleanup()
	ctx := context.Background()

	userID := uint64(789)
	user := &contracts.UserDTO{
		ID:       userID,
		Username: "testuser",
		Email:    "test@example.com",
	}

	// 1. Get from empty cache -> miss
	_, err := auth.GetCachedUser(ctx, userID)
	if err == nil {
		t.Fatal("expected cache miss for un-cached user")
	}

	// 2. Set to cache
	auth.SetCachedUser(ctx, userID, user)

	// 3. Get from cache -> hit
	cached, err := auth.GetCachedUser(ctx, userID)
	if err != nil {
		t.Fatalf("GetCachedUser() failed: %v", err)
	}
	if cached.ID != user.ID || cached.Username != user.Username {
		t.Fatalf("expected cached user %+v, got %+v", user, cached)
	}

	// 4. Invalidate cache
	auth.InvalidateCachedUser(ctx, userID)

	// 5. Get from cache -> miss
	_, err = auth.GetCachedUser(ctx, userID)
	if err == nil {
		t.Fatal("expected cache miss after invalidation")
	}
}
