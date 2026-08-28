// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package message_gateway

import (
	"Wavelet/core"
	"Wavelet/core/contracts"
	"context"
	"sync"

	"gorm.io/gorm"
)

var (
	dbMu     sync.RWMutex
	dbSvc    contracts.DBService
	cacheMu  sync.RWMutex
	cacheSvc contracts.CacheService
	taskMu   sync.RWMutex
	taskSvc  contracts.TaskService
	userMu   sync.RWMutex
	userSvc  contracts.UserService
)

// SetDBServiceForTest injects a DBService for tests. Production wiring must use Apply.
func SetDBServiceForTest(s contracts.DBService) {
	setDBService(s)
}

func setDBService(s contracts.DBService) {
	dbMu.Lock()
	defer dbMu.Unlock()
	dbSvc = s
}

func setCacheService(s contracts.CacheService) {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	cacheSvc = s
}

func setTaskService(s contracts.TaskService) {
	taskMu.Lock()
	defer taskMu.Unlock()
	taskSvc = s
}

func setUserService(s contracts.UserService) {
	userMu.Lock()
	defer userMu.Unlock()
	userSvc = s
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

func getTaskService() contracts.TaskService {
	taskMu.RLock()
	defer taskMu.RUnlock()
	return taskSvc
}

func getUserService(ctx context.Context) contracts.UserService {
	if c, ok := ctx.(*core.Context); ok && c != nil {
		if s, err := core.Inject[contracts.UserService](c); err == nil && s != nil {
			return s
		}
	}
	userMu.RLock()
	defer userMu.RUnlock()
	return userSvc
}
