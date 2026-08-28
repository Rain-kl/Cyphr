// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package logstore provides data access for analytics tables.
package logstore

import (
	"Wavelet/pkg/util"
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// CountAccessLogs returns the number of access logs matching filter.
func CountAccessLogs(ctx context.Context, filter AccessLogFilter) (uint64, error) {
	ch := getChDB(ctx)
	if ch == nil {
		return 0, fmt.Errorf("clickhouse gorm connection is not initialized")
	}

	var count int64
	query := applyFilter(ch.Model(&UserAccessLog{}), filter)
	if err := query.Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count access logs: %w", err)
	}
	return safeUint64Count(count), nil
}

// ListAccessLogs returns paginated access logs and the total match count.
func ListAccessLogs(ctx context.Context, filter AccessLogFilter, page, pageSize int) ([]UserAccessLog, uint64, error) {
	ch := getChDB(ctx)
	if ch == nil {
		return nil, 0, fmt.Errorf("clickhouse gorm connection is not initialized")
	}

	if filter.UserIDs != nil && len(filter.UserIDs) == 0 {
		return []UserAccessLog{}, 0, nil
	}

	var count int64
	query := applyFilter(ch.Model(&UserAccessLog{}), filter)
	if err := query.Count(&count).Error; err != nil {
		return nil, 0, fmt.Errorf("count access logs: %w", err)
	}

	var logs []UserAccessLog
	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").Limit(pageSize).Offset(offset).Find(&logs).Error; err != nil {
		return nil, 0, fmt.Errorf("list access logs: %w", err)
	}

	return logs, safeUint64Count(count), nil
}

// DeleteAllUserAccessLogs hard-deletes all user access logs via TRUNCATE.
func DeleteAllUserAccessLogs(ctx context.Context) (int64, error) {
	conn := getChConn()
	if conn == nil {
		return 0, fmt.Errorf("clickhouse connection is not initialized")
	}
	if err := conn.Exec(ctx, "TRUNCATE TABLE "+UserAccessLog{}.TableName()); err != nil {
		return 0, fmt.Errorf("truncate user access logs: %w", err)
	}
	return 0, nil
}

// DeleteUserAccessLogsBefore deletes user access logs older than cutoff.
func DeleteUserAccessLogsBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	conn := getChConn()
	if conn == nil {
		return 0, fmt.Errorf("clickhouse connection is not initialized")
	}
	if err := conn.Exec(ctx, "ALTER TABLE "+UserAccessLog{}.TableName()+" DELETE WHERE created_at < ?", cutoff); err != nil {
		return 0, fmt.Errorf("delete expired user access logs: %w", err)
	}
	return 0, nil
}

func safeUint64Count(count int64) uint64 {
	if count < 0 {
		return 0
	}
	return uint64(count)
}

func applyFilter(query *gorm.DB, filter AccessLogFilter) *gorm.DB {
	if filter.UserIDs != nil {
		if len(filter.UserIDs) == 0 {
			return query.Where("1 = 0")
		}
		query = query.Where("user_id IN ?", filter.UserIDs)
	}
	if filter.Path != "" {
		query = query.Where("path LIKE ? ESCAPE '\\'", "%"+util.EscapeLike(filter.Path)+"%")
	}
	if filter.StartTime != nil {
		query = query.Where("created_at >= ?", *filter.StartTime)
	}
	if filter.EndTime != nil {
		query = query.Where("created_at <= ?", *filter.EndTime)
	}
	return query
}
