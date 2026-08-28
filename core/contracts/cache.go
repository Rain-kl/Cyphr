// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package contracts defines unified service interfaces and DTOs for cross-plugin communication.
package contracts

import (
	"context"
	"errors"
	"time"
)

// ErrCacheMiss is returned when an item is not found in the cache.
var ErrCacheMiss = errors.New("contracts/cache: key not found")

// CacheService defines the contract for multi-layer cache operations (RAM L1 + Redis L2 + Pub/Sub invalidation).
type CacheService interface {
	// Get retrieves an item from cache into target. Returns ErrCacheMiss if not found.
	Get(ctx context.Context, key string, target any) error

	// Set stores an item into cache with a specified time-to-live duration.
	Set(ctx context.Context, key string, value any, ttl time.Duration) error

	// Delete evicts a key from local and remote cache tiers and broadcasts invalidation.
	Delete(ctx context.Context, key string) error

	// GetOrSet retrieves an item from cache, or calls loader to populate and return if missing.
	GetOrSet(ctx context.Context, key string, target any, ttl time.Duration, loader func() (any, error)) error

	// Invalidate is a semantic alias for Delete.
	Invalidate(ctx context.Context, key string) error
}
