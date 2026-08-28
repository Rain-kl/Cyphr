// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package logstore

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Rain-kl/Wavelet/backend/pkg/config"
	"github.com/Rain-kl/Wavelet/backend/pkg/logger"
	db "github.com/Rain-kl/Wavelet/backend/plugins/infra/database"
)

const (
	logDatabaseKey  = "log_database"
	logMigrationKey = "log_db_migration"
)

const (
	dbNamePostgres   = "postgres"
	dbNameSQLite     = "sqlite"
	dbNameClickHouse = "clickhouse"
)

var errConfigReaderNotWired = errors.New("logstore: config reader not wired")

// ConfigReader 读取系统配置字符串值，由 bootstrap 注入（避免 logstore ↔ repository 循环依赖）。
type ConfigReader func(ctx context.Context, key string) (string, error)

const resolveCacheTTL = 1 * time.Second

var (
	configReader ConfigReader

	storeMu         sync.RWMutex
	active          *Store
	activeDB        string
	lastResolveDB   string
	lastResolveTime time.Time
)

// SetConfigReader 注入系统配置读取函数（bootstrap 调用，测试可注入内存实现）。
func SetConfigReader(fn ConfigReader) { configReader = fn }

func getConfig(ctx context.Context, key string) (string, error) {
	if configReader == nil {
		return "", errConfigReaderNotWired
	}
	return configReader(ctx, key)
}

// Active 返回当前生效的日志库 Store。
func Active(ctx context.Context) (*Store, error) {
	current, err := resolveDatabase(ctx)
	if err != nil {
		return nil, err
	}
	storeMu.RLock()
	if active != nil && activeDB == current {
		s := active
		storeMu.RUnlock()
		return s, nil
	}
	storeMu.RUnlock()

	storeMu.Lock()
	defer storeMu.Unlock()
	if active != nil && activeDB == current {
		return active, nil
	}
	s, err := buildStore(ctx, current, false)
	if err != nil {
		return nil, err
	}
	active = s
	activeDB = current
	return s, nil
}

// Build 直接按目标构造 store（不经 Active 缓存）。
func Build(ctx context.Context, database string) (*Store, error) {
	return buildStore(ctx, database, false)
}

// BuildForMigration 构造迁移目标 store，跳过冻结检查。
func BuildForMigration(ctx context.Context, database string) (*Store, error) {
	return buildStore(ctx, database, true)
}

func buildStore(ctx context.Context, database string, skipFreeze bool) (*Store, error) {
	switch database {
	case dbNameClickHouse:
		ual := newClickHouseUserAccessLogStore()
		ual.skipFreeze = skipFreeze
		return &Store{UserAccessLogs: ual, Status: ual}, nil
	case dbNamePostgres, dbNameSQLite:
		gdb := db.DB(ctx)
		ual := newUserAccessLogGormStore(gdb)
		ual.skipFreeze = skipFreeze
		return &Store{UserAccessLogs: ual, Status: ual}, nil
	default:
		return nil, fmt.Errorf("unsupported log database: %s", database)
	}
}

// Migrating 返回日志库是否处于迁移冻结状态。
func Migrating(ctx context.Context) bool {
	v, err := getConfig(ctx, logMigrationKey)
	if err != nil {
		if !errors.Is(err, errConfigReaderNotWired) {
			logger.ErrorF(ctx, "read log migration config failed: %v", err)
		}
		return false
	}
	return v == "migrating"
}

// Init 预热激活 store，并兜底预建当前月及未来分区。
func Init(ctx context.Context) {
	s, err := Active(ctx)
	if err != nil {
		return
	}
	now := time.Now().UTC()
	if err := s.UserAccessLogs.EnsurePartitions(ctx, now, now.AddDate(0, partitionLeadMonths, 0)); err != nil {
		logger.WarnF(ctx, "logstore: ensure startup partitions failed: %v", err)
	}
}

// InvalidateCache 清空日志库解析缓存。
func InvalidateCache() {
	storeMu.Lock()
	defer storeMu.Unlock()
	lastResolveTime = time.Time{}
	lastResolveDB = ""
}

// ResetForTest 清空缓存的激活 store 与 config reader。
func ResetForTest() {
	storeMu.Lock()
	active = nil
	activeDB = ""
	lastResolveDB = ""
	lastResolveTime = time.Time{}
	storeMu.Unlock()
	configReader = nil
}

// ActiveDatabase 返回当前日志主库名。
func ActiveDatabase(ctx context.Context) (string, error) {
	return resolveDatabase(ctx)
}

func resolveDatabase(ctx context.Context) (string, error) {
	storeMu.RLock()
	if active != nil && time.Since(lastResolveTime) < resolveCacheTTL {
		name := lastResolveDB
		storeMu.RUnlock()
		return name, nil
	}
	storeMu.RUnlock()

	v, err := getConfig(ctx, logDatabaseKey)
	if err != nil && !errors.Is(err, errConfigReaderNotWired) {
		return "", err
	}

	resolved := v
	if resolved == "" {
		resolved = dbNameSQLite
		if config.Config.Database.Enabled {
			resolved = dbNamePostgres
		}
		if config.Config.ClickHouse.Enabled {
			resolved = dbNameClickHouse
		}
	}

	storeMu.Lock()
	lastResolveDB = resolved
	lastResolveTime = time.Now()
	storeMu.Unlock()
	return resolved, nil
}
