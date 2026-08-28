// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package logstore

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/backend/pkg/idgen"
	"gorm.io/gorm"
)

const (
	insertBatchSize   = 500
	migrationPageSize = 100
	defaultPageSize   = 20
	defaultTopN       = 10
	topUserAgents     = 100
	dayDuration       = 24 * time.Hour
)

type gormLogStore struct {
	db         *gorm.DB
	skipFreeze bool
}

func newGormStore(db *gorm.DB) *gormLogStore { return &gormLogStore{db: db} }

type userAccessLogGormStore struct {
	*gormLogStore
}

func newUserAccessLogGormStore(db *gorm.DB) *userAccessLogGormStore {
	return &userAccessLogGormStore{gormLogStore: newGormStore(db)}
}

var (
	_ UserAccessLogStore = (*userAccessLogGormStore)(nil)
	_ StatusStore        = (*userAccessLogGormStore)(nil)
)

func (s *gormLogStore) ActiveDatabase(_ context.Context) (string, error) {
	if isPostgresDialect(s.db) {
		return dbNamePostgres, nil
	}
	return dbNameSQLite, nil
}

func (s *gormLogStore) ensureWritable(ctx context.Context) error {
	if !s.skipFreeze && Migrating(ctx) {
		return ErrMigrating
	}
	return nil
}

func (s *userAccessLogGormStore) BatchInsert(ctx context.Context, logs []UserAccessLog) error {
	if len(logs) == 0 {
		return nil
	}
	if err := s.ensureWritable(ctx); err != nil {
		return err
	}
	for i := range logs {
		if logs[i].ID == 0 {
			logs[i].ID = idgen.NextUint64ID()
		}
	}
	return s.db.WithContext(ctx).CreateInBatches(logs, insertBatchSize).Error
}

func (s *userAccessLogGormStore) DeleteAll(ctx context.Context) (int64, error) {
	if err := s.ensureWritable(ctx); err != nil {
		return 0, err
	}
	res := s.db.WithContext(ctx).Where("1 = 1").Delete(&UserAccessLog{})
	return res.RowsAffected, res.Error
}

func (s *userAccessLogGormStore) DeleteBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	if err := s.ensureWritable(ctx); err != nil {
		return 0, err
	}
	res := s.db.WithContext(ctx).Where("created_at < ?", cutoff).Delete(&UserAccessLog{})
	if res.Error != nil && isMissingRelation(res.Error) {
		return 0, nil
	}
	return res.RowsAffected, res.Error
}

func (s *userAccessLogGormStore) ListForMigration(ctx context.Context, afterID uint64, limit int) ([]UserAccessLog, error) {
	var rows []UserAccessLog
	q := s.db.WithContext(ctx).Model(&UserAccessLog{}).
		Where("id > ?", afterID).
		Order("id ASC").
		Limit(limitOr(limit, migrationPageSize))
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *userAccessLogGormStore) MigrationRange(ctx context.Context) (time.Time, time.Time, error) {
	return gormMigrationRange(ctx, s.db, "created_at", UserAccessLog{}, func(v *UserAccessLog) time.Time {
		return v.CreatedAt
	})
}

func (s *userAccessLogGormStore) Count(ctx context.Context, filter AccessLogFilter) (uint64, error) {
	where, args, ok := buildUserAccessLogWhere(filter)
	if !ok {
		return 0, nil
	}
	var total int64
	if err := s.db.WithContext(ctx).Model(&UserAccessLog{}).Where(where, args...).Count(&total).Error; err != nil {
		return 0, err
	}
	return countToUint64(total), nil
}

func (s *userAccessLogGormStore) List(ctx context.Context, filter AccessLogFilter, page, pageSize int) ([]UserAccessLog, uint64, error) {
	where, args, ok := buildUserAccessLogWhere(filter)
	if !ok {
		return []UserAccessLog{}, 0, nil
	}
	var total int64
	if err := s.db.WithContext(ctx).Model(&UserAccessLog{}).Where(where, args...).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []UserAccessLog{}, 0, nil
	}
	var rows []UserAccessLog
	q := s.db.WithContext(ctx).Where(where, args...).Order("created_at DESC, id DESC")
	if err := q.Limit(limitOr(pageSize, defaultPageSize)).Offset(offsetOf(page, pageSize)).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, countToUint64(total), nil
}

func buildUserAccessLogWhere(filter AccessLogFilter) (string, []any, bool) {
	if filter.UserIDs != nil && len(filter.UserIDs) == 0 {
		return "", nil, false
	}
	var parts []string
	var args []any
	if filter.UserIDs != nil {
		parts = append(parts, "user_id IN ?")
		args = append(args, filter.UserIDs)
	}
	if trimmed := strings.TrimSpace(filter.Path); trimmed != "" {
		parts = append(parts, "path LIKE ?")
		args = append(args, "%"+trimmed+"%")
	}
	if filter.StartTime != nil {
		parts = append(parts, "created_at >= ?")
		args = append(args, *filter.StartTime)
	}
	if filter.EndTime != nil {
		parts = append(parts, "created_at <= ?")
		args = append(args, *filter.EndTime)
	}
	if len(parts) == 0 {
		return "1 = 1", args, true
	}
	return strings.Join(parts, " AND "), args, true
}

func (s *userAccessLogGormStore) GetDailyTrend(ctx context.Context, days int) ([]DailyTrend, error) {
	if days <= 0 {
		days = 7
	}
	start := time.Now().AddDate(0, 0, -(days - 1)).Truncate(dayDuration)
	type row struct {
		Date string
		Cnt  uint64
	}
	var rows []row
	err := s.db.WithContext(ctx).Model(&UserAccessLog{}).
		Select(dailyTrendDateSQL(s.db)+" AS date, COUNT(*) AS cnt").
		Where("created_at >= ?", start).
		Group("date").Order("date ASC").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	counts := make(map[string]uint64, len(rows))
	for _, r := range rows {
		counts[r.Date] = r.Cnt
	}
	out := make([]DailyTrend, 0, days)
	for i := 0; i < days; i++ {
		d := start.AddDate(0, 0, i).Format("2006-01-02")
		out = append(out, DailyTrend{Date: d, Count: counts[d]})
	}
	return out, nil
}

func (s *userAccessLogGormStore) GetBrowserDistribution(ctx context.Context, startTime time.Time) ([]BrowserShare, error) {
	type row struct {
		UserAgent string
		Cnt       uint64
	}
	var rows []row
	err := s.db.WithContext(ctx).Model(&UserAccessLog{}).
		Select("user_agent, COUNT(*) AS cnt").
		Where("created_at >= ?", startTime).
		Group("user_agent").Order("cnt DESC").Limit(topUserAgents).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	counts := make(map[string]uint64)
	for _, r := range rows {
		counts[ParseBrowserName(r.UserAgent)] += r.Cnt
	}
	out := make([]BrowserShare, 0, len(counts))
	for label, count := range counts {
		out = append(out, BrowserShare{Browser: label, Count: count})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Count > out[j].Count })
	return out, nil
}

func (s *userAccessLogGormStore) GetTopActiveUsers(ctx context.Context, startTime time.Time, limit int) ([]TopUser, error) {
	type row struct {
		UserID uint64
		Cnt    uint64
	}
	var rows []row
	err := s.db.WithContext(ctx).Model(&UserAccessLog{}).
		Select("user_id, COUNT(*) AS cnt").
		Where("user_id <> 0 AND created_at >= ?", startTime).
		Group("user_id").Order("cnt DESC").Limit(limitOr(limit, defaultTopN)).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]TopUser, len(rows))
	for i, r := range rows {
		out[i] = TopUser{UserID: r.UserID, Count: r.Cnt}
	}
	return out, nil
}

func (s *userAccessLogGormStore) EnsurePartitions(ctx context.Context, from, to time.Time) error {
	if !isPostgresDialect(s.db) {
		return nil
	}
	for _, sql := range partitionStatementsRange(from, to) {
		if err := s.db.WithContext(ctx).Exec(sql).Error; err != nil {
			return fmt.Errorf("ensure partition: %w", err)
		}
	}
	return nil
}

func gormMigrationRange[T any](
	ctx context.Context,
	gdb *gorm.DB,
	column string,
	model T,
	timeOf func(*T) time.Time,
) (time.Time, time.Time, error) {
	var first, last T
	found := false
	for _, order := range []string{"ASC", "DESC"} {
		out := &first
		if order == "DESC" {
			out = &last
		}
		res := gdb.WithContext(ctx).Model(model).Order(column + " " + order).Limit(1).Take(out)
		if res.Error != nil && !errors.Is(res.Error, gorm.ErrRecordNotFound) {
			return time.Time{}, time.Time{}, fmt.Errorf("query migration range %s: %w", column, res.Error)
		}
		if res.Error == nil {
			found = true
		}
	}
	if !found {
		return time.Time{}, time.Time{}, nil
	}
	return timeOf(&first).UTC(), timeOf(&last).UTC(), nil
}

func limitOr(v, def int) int {
	if v <= 0 {
		return def
	}
	return v
}

func offsetOf(page, pageSize int) int {
	if page < 1 {
		page = 1
	}
	return (page - 1) * limitOr(pageSize, defaultPageSize)
}

func countToUint64(v int64) uint64 {
	if v < 0 {
		return 0
	}
	return uint64(v)
}

func isPostgresDialect(db *gorm.DB) bool {
	return db != nil && db.Dialector != nil && db.Name() == "postgres"
}

func dailyTrendDateSQL(db *gorm.DB) string {
	if isPostgresDialect(db) {
		return "to_char(created_at, 'YYYY-MM-DD')"
	}
	return "strftime('%Y-%m-%d', created_at)"
}

func isMissingRelation(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no such table") || strings.Contains(msg, "does not exist")
}
