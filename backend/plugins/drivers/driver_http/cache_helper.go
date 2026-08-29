// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package driver_http

import (
	"Wavelet/core"
	"Wavelet/core/contracts"
	"context"
	"sync"
)

var (
	cacheMu  sync.RWMutex
	cacheSvc contracts.CacheService
)

func setCacheService(s contracts.CacheService) {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	cacheSvc = s
}

// getCache resolves the cache service from a micro-kernel context, falling back
// to the instance bound during plugin registration.
func getCache(ctx context.Context) contracts.CacheService {
	if c, ok := ctx.(*core.Context); ok && c != nil {
		if s, err := core.Inject[contracts.CacheService](c); err == nil && s != nil {
			return s
		}
	}
	cacheMu.RLock()
	s := cacheSvc
	cacheMu.RUnlock()
	return s
}
