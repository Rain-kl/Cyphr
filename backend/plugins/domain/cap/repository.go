// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cap

import (
	"Wavelet/core"
	"Wavelet/core/contracts"
	"context"
	"sync"

	"gorm.io/gorm"
)

var (
	dbMu  sync.RWMutex
	dbSvc contracts.DBService
)

// setDBService caches the DBService contract used by the persistence layer.
func setDBService(s contracts.DBService) {
	dbMu.Lock()
	defer dbMu.Unlock()
	dbSvc = s
}

// getDB resolves a GORM handle, preferring the *core.Context when supplied by callers.
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

// loadRuntimeSettings reads the CAPTCHA owned rows from the system config table.
func loadRuntimeSettings(ctx context.Context) (RuntimeSettings, error) {
	var records []configRecord
	db := getDB(ctx)
	if db == nil {
		return parseRuntimeSettings(nil), nil
	}
	if err := db.Table("w_system_configs").Where("key IN ?", runtimeConfigKeys).Find(&records).Error; err != nil {
		return RuntimeSettings{}, err
	}
	configs := make(map[string]string, len(records))
	for _, r := range records {
		configs[r.Key] = r.Value
	}
	return parseRuntimeSettings(configs), nil
}
