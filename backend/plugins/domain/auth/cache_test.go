// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package auth_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"Wavelet/core"
	"Wavelet/core/contracts"
	"Wavelet/plugins/domain/auth"
)

type mockCacheService struct {
	items map[string][]byte
}

func newMockCacheService() *mockCacheService {
	return &mockCacheService{items: make(map[string][]byte)}
}

func (m *mockCacheService) Get(ctx context.Context, key string, target any) error {
	b, ok := m.items[key]
	if !ok {
		return contracts.ErrCacheMiss
	}
	return json.Unmarshal(b, target)
}

func (m *mockCacheService) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	m.items[key] = b
	return nil
}

func (m *mockCacheService) Delete(ctx context.Context, key string) error {
	delete(m.items, key)
	return nil
}

func (m *mockCacheService) Invalidate(ctx context.Context, key string) error {
	return m.Delete(ctx, key)
}

func (m *mockCacheService) GetOrSet(ctx context.Context, key string, target any, ttl time.Duration, loader func() (any, error)) error {
	err := m.Get(ctx, key, target)
	if err == nil {
		return nil
	}
	val, err := loader()
	if err != nil {
		return err
	}
	if err := m.Set(ctx, key, val, ttl); err != nil {
		return err
	}
	b, _ := json.Marshal(val)
	return json.Unmarshal(b, target)
}

func TestTokenCache_GetSetInvalidate(t *testing.T) {
	ctx := core.NewContext(context.Background())
	mockCache := newMockCacheService()
	core.Provide[contracts.CacheService](ctx, mockCache)

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
	ctx := core.NewContext(context.Background())
	mockCache := newMockCacheService()
	core.Provide[contracts.CacheService](ctx, mockCache)

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
