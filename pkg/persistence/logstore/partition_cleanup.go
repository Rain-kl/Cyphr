// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package logstore

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

// listPartitionNames 列出 table 在当前 schema 下的全部直接分区表名（pg_inherits）。
func listPartitionNames(ctx context.Context, gdb *gorm.DB, table string) ([]string, error) {
	var names []string
	if err := gdb.WithContext(ctx).Raw(`
SELECT c.relname
FROM pg_inherits i
JOIN pg_class c ON c.oid = i.inhrelid
JOIN pg_class p ON p.oid = i.inhparent
JOIN pg_namespace n ON n.oid = p.relnamespace AND n.nspname = current_schema()
WHERE p.relname = ?`, table).Scan(&names).Error; err != nil {
		return nil, fmt.Errorf("list partitions of %s: %w", table, err)
	}
	return names, nil
}

// partitionNameMonth 解析按月分区表名 <table>_YYYYMM 的所属月份；命名不匹配返回 (零值, false)。
func partitionNameMonth(table, name string) (time.Time, bool) {
	suffix, ok := strings.CutPrefix(name, table+"_")
	if !ok || len(suffix) != 6 {
		return time.Time{}, false
	}
	m, err := time.Parse("200601", suffix)
	if err != nil {
		return time.Time{}, false
	}
	return m, true
}

// dropEligiblePartitionNames 返回 before 月份之前、命名合法的分区表名（是否为空由调用方校验）。
func dropEligiblePartitionNames(table string, names []string, before time.Time) []string {
	beforeMonth := time.Date(before.Year(), before.Month(), 1, 0, 0, 0, 0, time.UTC)
	out := make([]string, 0, len(names))
	for _, name := range names {
		month, ok := partitionNameMonth(table, name)
		if !ok || !month.Before(beforeMonth) {
			continue
		}
		out = append(out, name)
	}
	return out
}

// DropEmptyPartitions 幂等清理 PG 空分区表：仅删除 before 月份之前、且无任何数据的分区。
// 非 PG 方言为 no-op。
func (s *gormLogStore) DropEmptyPartitions(ctx context.Context, before time.Time) error {
	if !isPostgresDialect(s.db) {
		return nil
	}
	names, err := listPartitionNames(ctx, s.db, userAccessLogTable)
	if err != nil {
		return err
	}
	for _, name := range dropEligiblePartitionNames(userAccessLogTable, names, before) {
		var one int
		if err := s.db.WithContext(ctx).Raw("SELECT 1 FROM " + name + " LIMIT 1").Scan(&one).Error; err != nil {
			return fmt.Errorf("check partition %s empty: %w", name, err)
		}
		if one == 1 {
			continue
		}
		if err := s.db.WithContext(ctx).Exec("DROP TABLE IF EXISTS " + name).Error; err != nil {
			return fmt.Errorf("drop empty partition %s: %w", name, err)
		}
	}
	return nil
}

// DropExpiredPartitions 直接删除完全过期的 PG 整月分区（避免 retention 清理逐行 DELETE）。
// 候选 = 月份早于 cutoff 月（按 cutoff 的 UTC 时刻取月）；删除前校验分区内不存在 created_at >= cutoff 的行。
// 迁移冻结期间返回 ErrMigrating。CH/SQLite 为 no-op。
func (s *gormLogStore) DropExpiredPartitions(ctx context.Context, cutoff time.Time) error {
	if !isPostgresDialect(s.db) {
		return nil
	}
	if err := s.ensureWritable(ctx); err != nil {
		return err
	}
	names, err := listPartitionNames(ctx, s.db, userAccessLogTable)
	if err != nil {
		return err
	}
	cu := cutoff.UTC()
	cutoffMonth := time.Date(cu.Year(), cu.Month(), 1, 0, 0, 0, 0, time.UTC)
	for _, name := range names {
		month, ok := partitionNameMonth(userAccessLogTable, name)
		if !ok || !month.Before(cutoffMonth) {
			continue
		}
		var hasRetained int
		if err := s.db.WithContext(ctx).Raw("SELECT 1 FROM "+name+" WHERE created_at >= ? LIMIT 1", cu).Scan(&hasRetained).Error; err != nil {
			return fmt.Errorf("check partition %s retained rows: %w", name, err)
		}
		if hasRetained == 1 {
			continue
		}
		if err := s.db.WithContext(ctx).Exec("DROP TABLE IF EXISTS " + name).Error; err != nil {
			return fmt.Errorf("drop expired partition %s: %w", name, err)
		}
	}
	return nil
}
