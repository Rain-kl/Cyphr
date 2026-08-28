// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package risk_control

import (
	"Wavelet/plugins/domain/risk_control/logstore"
	"context"
)

// Repository 层：本插件根包内唯一的持久化访问入口。
//
// 真正的 SQL / 驱动实现位于 logstore 子包（受 logstore skill 约束的存储抽象），
// 本文件负责解析当前生效日志库并转发读写、迁移与查询，使 service.go 只做用例编排。

// accessLogMigrationBatchSize 是日志引擎迁移时单批搬运的行数。
const accessLogMigrationBatchSize = 1000

// writeAccessLogBatch 持久化批写缓冲队列中取出的一批访问日志。
func writeAccessLogBatch(ctx context.Context, items []*logstore.UserAccessLog) error {
	rows := make([]logstore.UserAccessLog, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		rows = append(rows, *item)
	}
	store, err := logstore.Active(ctx)
	if err != nil {
		return err
	}
	return store.UserAccessLogs.BatchInsert(ctx, rows)
}

// listAccessLogs 按过滤条件读取一页访问日志。
func listAccessLogs(ctx context.Context, filter logstore.AccessLogFilter, page, pageSize int) ([]logstore.UserAccessLog, uint64, error) {
	store, err := logstore.Active(ctx)
	if err != nil {
		return nil, 0, err
	}
	return store.UserAccessLogs.List(ctx, filter, page, pageSize)
}

// accessLogDailyTrend 读取最近 days 天的按天访问趋势。
func accessLogDailyTrend(ctx context.Context, days int) ([]logstore.DailyTrend, error) {
	store, err := logstore.Active(ctx)
	if err != nil {
		return nil, err
	}
	return store.UserAccessLogs.GetDailyTrend(ctx, days)
}

// activeLogDatabase 返回当前生效日志库的引擎标识。
func activeLogDatabase(ctx context.Context) (string, error) {
	store, err := logstore.Active(ctx)
	if err != nil {
		return "", err
	}
	return store.Status.ActiveDatabase(ctx)
}

// logStoreMigrating 报告日志库是否处于迁移冻结期。
func logStoreMigrating(ctx context.Context) bool {
	return logstore.Migrating(ctx)
}

// loadMigrationStores 解析迁移源（当前生效库）与目标引擎库。
func loadMigrationStores(ctx context.Context, targetEngine string) (src, dst *logstore.Store, err error) {
	src, err = logstore.Active(ctx)
	if err != nil {
		return nil, nil, err
	}
	dst, err = logstore.BuildForMigration(ctx, targetEngine)
	if err != nil {
		return nil, nil, err
	}
	return src, dst, nil
}

// copyAccessLogs 清空目标库、按源库时间范围预建分区后分批搬运全部源数据，
// 并在每批完成后通过 reportProgress 回调累计已搬运行数。
func copyAccessLogs(ctx context.Context, src, dst *logstore.Store, reportProgress func(copied int)) error {
	if _, err := dst.UserAccessLogs.DeleteAll(ctx); err != nil {
		return err
	}
	from, to, err := src.UserAccessLogs.MigrationRange(ctx)
	if err != nil {
		return err
	}
	if !from.IsZero() && !to.IsZero() {
		if err := dst.UserAccessLogs.EnsurePartitions(ctx, from, to); err != nil {
			return err
		}
	}

	var afterID uint64
	var copied int
	for {
		rows, err := src.UserAccessLogs.ListForMigration(ctx, afterID, accessLogMigrationBatchSize)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			break
		}
		if err := dst.UserAccessLogs.BatchInsert(ctx, rows); err != nil {
			return err
		}
		afterID = rows[len(rows)-1].ID
		copied += len(rows)
		if reportProgress != nil {
			reportProgress(copied)
		}
		if len(rows) < accessLogMigrationBatchSize {
			break
		}
	}
	return nil
}

// resetLogStoreCache 丢弃缓存的生效日志库，使下一次访问重新解析。
func resetLogStoreCache() {
	logstore.InvalidateCache()
}
