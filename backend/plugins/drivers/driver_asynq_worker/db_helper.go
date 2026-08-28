// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package driver_asynq_worker

import (
	"context"
	"sync"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"Wavelet/core"
	"Wavelet/core/contracts"
)

var (
	dbMu      sync.RWMutex
	dbSvc     contracts.DBService
	redisMu   sync.RWMutex
	rdbClient redis.UniversalClient
)

func setDBService(s contracts.DBService) {
	dbMu.Lock()
	defer dbMu.Unlock()
	dbSvc = s
}

// SetRedisClient sets the redis client used for task logs.
func SetRedisClient(c redis.UniversalClient) {
	redisMu.Lock()
	defer redisMu.Unlock()
	rdbClient = c
}

func getDB(ctx context.Context) *gorm.DB {
	if c, ok := ctx.(*core.Context); ok && c != nil {
		if s, err := core.Inject[contracts.DBService](c); err == nil && s != nil {
			return s.DB(ctx)
		}
	}
	dbMu.RLock()
	s := dbSvc
	dbMu.RUnlock()
	if s != nil {
		return s.DB(ctx)
	}
	return nil
}

func getRedisClient() redis.UniversalClient {
	redisMu.RLock()
	defer redisMu.RUnlock()
	return rdbClient
}
