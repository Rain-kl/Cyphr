// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package logstore

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	db "github.com/Rain-kl/Wavelet/pkg/persistence"
	"github.com/Rain-kl/Wavelet/pkg/persistence/analytics"
)

type clickhouseUserAccessLogStore struct {
	skipFreeze bool
}

func newClickHouseUserAccessLogStore() *clickhouseUserAccessLogStore {
	return &clickhouseUserAccessLogStore{}
}

var (
	_ UserAccessLogStore = (*clickhouseUserAccessLogStore)(nil)
	_ StatusStore        = (*clickhouseUserAccessLogStore)(nil)
)

func (s *clickhouseUserAccessLogStore) ActiveDatabase(_ context.Context) (string, error) {
	return dbNameClickHouse, nil
}

func (s *clickhouseUserAccessLogStore) ensureWritable(ctx context.Context) error {
	if !s.skipFreeze && Migrating(ctx) {
		return ErrMigrating
	}
	return nil
}

func (s *clickhouseUserAccessLogStore) BatchInsert(ctx context.Context, logs []analytics.UserAccessLog) error {
	if len(logs) == 0 {
		return nil
	}
	if err := s.ensureWritable(ctx); err != nil {
		return err
	}
	return analytics.BatchInsert(ctx, logs)
}

func (s *clickhouseUserAccessLogStore) DeleteAll(ctx context.Context) (int64, error) {
	if err := s.ensureWritable(ctx); err != nil {
		return 0, err
	}
	return analytics.DeleteAllUserAccessLogs(ctx)
}

func (s *clickhouseUserAccessLogStore) DeleteBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	if err := s.ensureWritable(ctx); err != nil {
		return 0, err
	}
	return analytics.DeleteUserAccessLogsBefore(ctx, cutoff)
}

func (s *clickhouseUserAccessLogStore) Count(ctx context.Context, filter analytics.AccessLogFilter) (uint64, error) {
	return analytics.CountAccessLogs(ctx, filter)
}

func (s *clickhouseUserAccessLogStore) List(ctx context.Context, filter analytics.AccessLogFilter, page, pageSize int) ([]analytics.UserAccessLog, uint64, error) {
	return analytics.ListAccessLogs(ctx, filter, page, pageSize)
}

func (s *clickhouseUserAccessLogStore) GetDailyTrend(ctx context.Context, days int) ([]analytics.DailyTrend, error) {
	return analytics.GetDailyTrend(ctx, days)
}

func (s *clickhouseUserAccessLogStore) GetBrowserDistribution(ctx context.Context, startTime time.Time) ([]analytics.BrowserShare, error) {
	return analytics.GetBrowserDistribution(ctx, startTime)
}

func (s *clickhouseUserAccessLogStore) GetTopActiveUsers(ctx context.Context, startTime time.Time, limit int) ([]analytics.TopUser, error) {
	return analytics.GetTopActiveUsers(ctx, startTime, limit)
}

func (s *clickhouseUserAccessLogStore) EnsurePartitions(_ context.Context, _, _ time.Time) error {
	return nil
}

func (s *clickhouseUserAccessLogStore) DropEmptyPartitions(_ context.Context, _ time.Time) error {
	return nil
}

func (s *clickhouseUserAccessLogStore) DropExpiredPartitions(_ context.Context, _ time.Time) error {
	return nil
}

func (s *clickhouseUserAccessLogStore) MigrationRange(ctx context.Context) (time.Time, time.Time, error) {
	if db.ChConn == nil {
		return time.Time{}, time.Time{}, fmt.Errorf("clickhouse connection is not initialized")
	}
	table := analytics.UserAccessLog{}.TableName()
	var minTime, maxTime *time.Time
	if err := db.ChConn.QueryRow(ctx, "SELECT min(created_at), max(created_at) FROM "+table).Scan(&minTime, &maxTime); err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("query migration range %s: %w", table, err)
	}
	if minTime == nil || maxTime == nil {
		return time.Time{}, time.Time{}, nil
	}
	return minTime.UTC(), maxTime.UTC(), nil
}

func (s *clickhouseUserAccessLogStore) ListForMigration(ctx context.Context, afterID uint64, limit int) ([]analytics.UserAccessLog, error) {
	if db.ChConn == nil {
		return nil, fmt.Errorf("clickhouse connection is not initialized")
	}
	if limit <= 0 {
		limit = migrationPageSize
	}
	table := analytics.UserAccessLog{}.TableName()
	columns := analytics.UserAccessLog{}.InsertColumns()
	rows, err := db.ChConn.Query(ctx, fmt.Sprintf(
		"SELECT %s FROM %s WHERE id > ? ORDER BY id ASC LIMIT ?",
		columns, table,
	), afterID, limit)
	if err != nil {
		return nil, fmt.Errorf("list user access logs for migration: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanUserAccessLogs(rows)
}

func scanUserAccessLogs(rows driver.Rows) ([]analytics.UserAccessLog, error) {
	var result []analytics.UserAccessLog
	for rows.Next() {
		var item analytics.UserAccessLog
		if err := rows.Scan(
			&item.ID,
			&item.UserID,
			&item.Path,
			&item.Method,
			&item.IP,
			&item.UserAgent,
			&item.Headers,
			&item.Status,
			&item.Latency,
			&item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan user access log row: %w", err)
		}
		item.CreatedAt = item.CreatedAt.UTC()
		result = append(result, item)
	}
	return result, nil
}
