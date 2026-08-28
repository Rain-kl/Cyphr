// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package admin

import (
	"context"
	"sync"

	"gorm.io/gorm"

	"Wavelet/core/contracts"
)

var (
	servicesMu         sync.RWMutex
	dbService          contracts.DBService
	cacheService       contracts.CacheService
	userService        contracts.UserService
	authService        contracts.AuthService
	taskService        contracts.TaskService
	storageSvc         contracts.StorageService
	riskControlService contracts.RiskControlService
	eventEmitter       func(ctx context.Context, topic string, payload any) error
)

// SetDBService injects the DBService contract.
func SetDBService(s contracts.DBService) {
	servicesMu.Lock()
	defer servicesMu.Unlock()
	dbService = s
}

// SetCacheService injects the CacheService contract.
func SetCacheService(s contracts.CacheService) {
	servicesMu.Lock()
	defer servicesMu.Unlock()
	cacheService = s
}

// SetUserService injects the UserService contract.
func SetUserService(s contracts.UserService) {
	servicesMu.Lock()
	defer servicesMu.Unlock()
	userService = s
}

// SetAuthService injects the AuthService contract.
func SetAuthService(s contracts.AuthService) {
	servicesMu.Lock()
	defer servicesMu.Unlock()
	authService = s
}

// SetTaskService injects the TaskService contract.
func SetTaskService(s contracts.TaskService) {
	servicesMu.Lock()
	defer servicesMu.Unlock()
	taskService = s
}

// SetStorageService injects the StorageService contract.
func SetStorageService(s contracts.StorageService) {
	servicesMu.Lock()
	defer servicesMu.Unlock()
	storageSvc = s
}

// SetRiskControlService injects the RiskControlService contract.
func SetRiskControlService(s contracts.RiskControlService) {
	servicesMu.Lock()
	defer servicesMu.Unlock()
	riskControlService = s
}

// SetEventEmitter sets the event emission callback.
func SetEventEmitter(fn func(ctx context.Context, topic string, payload any) error) {
	servicesMu.Lock()
	defer servicesMu.Unlock()
	eventEmitter = fn
}

// EmitEvent publishes a domain event if an emitter is registered.
func EmitEvent(ctx context.Context, topic string, payload any) error {
	servicesMu.RLock()
	defer servicesMu.RUnlock()
	if eventEmitter == nil {
		return nil
	}
	return eventEmitter(ctx, topic, payload)
}

// ResetServices clears all injected services (used on disposal and testing).
func ResetServices() {
	servicesMu.Lock()
	defer servicesMu.Unlock()
	dbService = nil
	cacheService = nil
	userService = nil
	authService = nil
	taskService = nil
	storageSvc = nil
	riskControlService = nil
	eventEmitter = nil
}

// GetDB returns the GORM DB instance bound to the context if available.
func GetDB(ctx context.Context) *gorm.DB {
	servicesMu.RLock()
	defer servicesMu.RUnlock()
	if dbService == nil {
		return nil
	}
	return dbService.DB(ctx)
}

// GetCache returns the unified CacheService instance.
func GetCache(ctx context.Context) contracts.CacheService {
	servicesMu.RLock()
	defer servicesMu.RUnlock()
	return cacheService
}

// GetUserService returns the UserService instance.
func GetUserService(ctx context.Context) contracts.UserService {
	servicesMu.RLock()
	defer servicesMu.RUnlock()
	return userService
}

// GetAuthService returns the AuthService instance.
func GetAuthService(ctx context.Context) contracts.AuthService {
	servicesMu.RLock()
	defer servicesMu.RUnlock()
	return authService
}

// GetTaskService returns the TaskService instance.
func GetTaskService() contracts.TaskService {
	servicesMu.RLock()
	defer servicesMu.RUnlock()
	return taskService
}

// GetStorageService returns the StorageService instance.
func GetStorageService() contracts.StorageService {
	servicesMu.RLock()
	defer servicesMu.RUnlock()
	return storageSvc
}

// GetRiskControlService returns the RiskControlService instance.
func GetRiskControlService() contracts.RiskControlService {
	servicesMu.RLock()
	defer servicesMu.RUnlock()
	return riskControlService
}
