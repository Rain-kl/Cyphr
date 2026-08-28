// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cache_memory

import (
	"context"
	"encoding/json"
	"time"

	"Wavelet/core"
	"Wavelet/core/contracts"
	"Wavelet/pkg/cache/ram"
)

type ramEntry struct {
	data     []byte
	expireAt time.Time
}

type memoryCacheService struct {
	ramCache *ram.Cache[string, ramEntry]
	events   *core.EventBus
}

func newMemoryCacheService(capacity int, events *core.EventBus) (*memoryCacheService, error) {
	ramCache, err := ram.New[string, ramEntry](ram.Options{
		MaximumSize: capacity,
	})
	if err != nil {
		return nil, err
	}

	return &memoryCacheService{
		ramCache: ramCache,
		events:   events,
	}, nil
}

func (s *memoryCacheService) Get(ctx context.Context, key string, target any) error {
	if entry, ok := s.ramCache.GetIfPresent(key); ok {
		if entry.expireAt.IsZero() || time.Now().Before(entry.expireAt) {
			return json.Unmarshal(entry.data, target)
		}
		// Expired
		s.ramCache.Invalidate(key)
	}
	return contracts.ErrCacheMiss
}

func (s *memoryCacheService) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}

	var expireAt time.Time
	if ttl > 0 {
		expireAt = time.Now().Add(ttl)
	}

	s.ramCache.Set(key, ramEntry{
		data:     data,
		expireAt: expireAt,
	})
	return nil
}

func (s *memoryCacheService) Delete(ctx context.Context, key string) error {
	s.ramCache.Invalidate(key)
	if s.events != nil {
		_ = s.events.Emit(ctx, "cache:invalidate", key)
	}
	return nil
}

func (s *memoryCacheService) Invalidate(ctx context.Context, key string) error {
	return s.Delete(ctx, key)
}

func (s *memoryCacheService) GetOrSet(ctx context.Context, key string, target any, ttl time.Duration, loader func() (any, error)) error {
	err := s.Get(ctx, key, target)
	if err == nil {
		return nil
	}

	val, err := loader()
	if err != nil {
		return err
	}

	if setErr := s.Set(ctx, key, val, ttl); setErr != nil {
		return setErr
	}

	data, err := json.Marshal(val)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}
