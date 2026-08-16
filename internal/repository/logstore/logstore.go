// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package logstore abstracts user access-log storage across ClickHouse, PostgreSQL and SQLite.
package logstore

import (
	"context"
	"errors"
	"time"

	analyticsmodel "github.com/Rain-kl/Wavelet/internal/model/analytics"
	analyticsrepo "github.com/Rain-kl/Wavelet/internal/repository/analytics"
)

// ErrMigrating 表示日志数据库正在迁移，当前禁止写入。
var ErrMigrating = errors.New("log database is migrating, writes are disabled")

// UserAccessLogStore 用户访问日志（w_user_access_logs）。
type UserAccessLogStore interface {
	BatchInsert(ctx context.Context, logs []analyticsmodel.UserAccessLog) error
	DeleteAll(ctx context.Context) (int64, error)
	DeleteBefore(ctx context.Context, cutoff time.Time) (int64, error)
	Count(ctx context.Context, filter analyticsrepo.AccessLogFilter) (uint64, error)
	List(ctx context.Context, filter analyticsrepo.AccessLogFilter, page, pageSize int) ([]analyticsmodel.UserAccessLog, uint64, error)
	GetDailyTrend(ctx context.Context, days int) ([]analyticsrepo.DailyTrend, error)
	GetBrowserDistribution(ctx context.Context, startTime time.Time) ([]analyticsrepo.BrowserShare, error)
	GetTopActiveUsers(ctx context.Context, startTime time.Time, limit int) ([]analyticsrepo.TopUser, error)
	ListForMigration(ctx context.Context, afterID uint64, limit int) ([]analyticsmodel.UserAccessLog, error)
	MigrationRange(ctx context.Context) (from, to time.Time, err error)
	EnsurePartitions(ctx context.Context, from, to time.Time) error
	// DropEmptyPartitions 幂等清理 PG 空分区表：删除 before 月份之前、且无任何数据的按月分区；
	// CH/SQLite 为 no-op。
	DropEmptyPartitions(ctx context.Context, before time.Time) error
	// DropExpiredPartitions 直接删除完全过期的 PG 整月分区（候选为月份早于 cutoff 月的分区，
	// 删除前校验分区内无保留期内数据，避免时区偏移下误删；迁移冻结期间拒绝执行）；CH/SQLite 为 no-op。
	DropExpiredPartitions(ctx context.Context, cutoff time.Time) error
}

// AccessLogFilter 是查询过滤器的别名，供 apps 使用，避免 import repository/analytics。
type AccessLogFilter = analyticsrepo.AccessLogFilter

// StatusStore 日志库状态。
type StatusStore interface {
	ActiveDatabase(ctx context.Context) (string, error)
}

// Store 当前生效日志库。
type Store struct {
	UserAccessLogs UserAccessLogStore
	Status         StatusStore
}
