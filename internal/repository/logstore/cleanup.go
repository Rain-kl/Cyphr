// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package logstore

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/pkg/logger"
)

const (
	defaultLogRetentionDays = 30
	partitionLeadMonths     = 2
	userAccessLogTable      = "w_user_access_logs"
)

// CleanupSummary 汇总本次清理结果。
type CleanupSummary struct {
	ActiveDatabase string `json:"active_database"`
	RetentionDays  int    `json:"retention_days"`
	Deleted        int64  `json:"deleted"`
}

// CleanupExpired 按当前日志库保留天数删除过期用户访问日志，并预建 PG 分区。
func CleanupExpired(ctx context.Context) (CleanupSummary, error) {
	active, err := ActiveDatabase(ctx)
	if err != nil {
		return CleanupSummary{}, err
	}
	days := retentionDaysForDatabase(ctx, active)
	summary := CleanupSummary{ActiveDatabase: active, RetentionDays: days}

	store, err := Active(ctx)
	if err != nil {
		return summary, err
	}
	now := time.Now().UTC()
	if err := store.UserAccessLogs.EnsurePartitions(ctx, now, now.AddDate(0, partitionLeadMonths, 0)); err != nil {
		logger.WarnF(ctx, "logstore: ensure partitions during cleanup failed: %v", err)
	}
	cutoff := now.AddDate(0, 0, -days)
	// 先 DROP 完全过期的整月分区，再对边界月逐行 DeleteBefore。
	if err := store.UserAccessLogs.DropExpiredPartitions(ctx, cutoff); err != nil {
		return summary, fmt.Errorf("drop expired partitions: %w", err)
	}
	deleted, err := store.UserAccessLogs.DeleteBefore(ctx, cutoff)
	if err != nil {
		return summary, fmt.Errorf("delete expired user access logs: %w", err)
	}
	summary.Deleted = deleted
	if err := store.UserAccessLogs.DropEmptyPartitions(ctx, now); err != nil {
		logger.WarnF(ctx, "drop empty log partitions failed: %v", err)
	}
	return summary, nil
}

func retentionDaysForDatabase(ctx context.Context, dbName string) int {
	key := model.ConfigKeyLogRetentionDaysPostgres
	switch dbName {
	case dbNameSQLite:
		key = model.ConfigKeyLogRetentionDaysSQLite
	case dbNameClickHouse:
		key = model.ConfigKeyLogRetentionDaysClickHouse
	}
	v, err := getConfig(ctx, key)
	if err != nil {
		if !errors.Is(err, errConfigReaderNotWired) {
			logger.ErrorF(ctx, "读取日志保留天数配置失败(key=%s)，回退默认 %d 天: %v", key, defaultLogRetentionDays, err)
		}
		return defaultLogRetentionDays
	}
	days, perr := strconv.Atoi(v)
	if perr != nil || days <= 0 {
		logger.ErrorF(ctx, "日志保留天数配置非法(key=%s, value=%q)，回退默认 %d 天", key, v, defaultLogRetentionDays)
		return defaultLogRetentionDays
	}
	return days
}

func partitionStatementsRange(from, to time.Time) []string {
	var out []string
	start := time.Date(from.Year(), from.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(to.Year(), to.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, 1, 0)
	for ; start.Before(end); start = start.AddDate(0, 1, 0) {
		monthEnd := start.AddDate(0, 1, 0)
		suffix := start.Format("200601")
		fromDay := start.Format("2006-01-02")
		toDay := monthEnd.Format("2006-01-02")
		out = append(out, fmt.Sprintf(
			"CREATE TABLE IF NOT EXISTS %s_%s PARTITION OF %s FOR VALUES FROM ('%s') TO ('%s')",
			userAccessLogTable, suffix, userAccessLogTable, fromDay, toDay))
	}
	return out
}
