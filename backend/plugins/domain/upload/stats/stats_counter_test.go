// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package stats

import (
	"Wavelet/pkg/testhelper"
	"Wavelet/plugins/domain/upload/models"
	"Wavelet/plugins/domain/upload/shared"
	"context"
	"testing"
	"time"

	"gorm.io/gorm"
)

type mockDBService struct {
	db *gorm.DB
}

func (m *mockDBService) GORM() *gorm.DB {
	return m.db
}

func (m *mockDBService) DB(ctx context.Context) *gorm.DB {
	return m.db.WithContext(ctx)
}

func (m *mockDBService) Named(_ string) *gorm.DB {
	return m.db
}

func TestApplyUploadStatsDeltaTxWithinTransaction(t *testing.T) {
	dbConn, _, cleanup := testhelper.SetupTestEnvironment(t)
	shared.SetDBService(&mockDBService{db: dbConn})
	defer func() {
		shared.SetDBService(nil)
		cleanup()
	}()
	ctx := context.Background()

	upload := &models.Upload{
		ID:        42002,
		FileSize:  256,
		MimeType:  "image/jpeg",
		Extension: "jpg",
		Type:      "avatar",
		Status:    models.UploadStatusUsed,
		CreatedAt: time.Now(),
	}

	if err := shared.GetDB(ctx).Transaction(func(tx *gorm.DB) error {
		return ApplyUploadStatsDeltaTx(tx, upload, 1)
	}); err != nil {
		t.Fatalf("ApplyUploadStatsDeltaTx returned error: %v", err)
	}

	stats, err := loadUploadStats(ctx)
	if err != nil {
		t.Fatalf("loadUploadStats returned error: %v", err)
	}
	if stats.TotalCount != 1 || stats.TotalSize != 256 {
		t.Fatalf("unexpected total stats: count=%d size=%d", stats.TotalCount, stats.TotalSize)
	}
}

func TestApplyUploadStatsAddAndRemove(t *testing.T) {
	dbConn, _, cleanup := testhelper.SetupTestEnvironment(t)
	shared.SetDBService(&mockDBService{db: dbConn})
	defer func() {
		shared.SetDBService(nil)
		cleanup()
	}()
	ctx := context.Background()

	upload := &models.Upload{
		ID:        42001,
		FileSize:  128,
		MimeType:  "image/png",
		Extension: "png",
		Type:      "avatar",
		Status:    models.UploadStatusUsed,
		CreatedAt: time.Now(),
	}
	if err := ApplyUploadStatsAdd(ctx, upload); err != nil {
		t.Fatalf("ApplyUploadStatsAdd returned error: %v", err)
	}

	stats, err := loadUploadStats(ctx)
	if err != nil {
		t.Fatalf("loadUploadStats returned error: %v", err)
	}
	if stats.TotalCount != 1 || stats.TotalSize != 128 {
		t.Fatalf("unexpected total stats: count=%d size=%d", stats.TotalCount, stats.TotalSize)
	}

	if err := ApplyUploadStatsRemove(ctx, upload); err != nil {
		t.Fatalf("ApplyUploadStatsRemove returned error: %v", err)
	}

	stats, err = loadUploadStats(ctx)
	if err != nil {
		t.Fatalf("loadUploadStats after remove returned error: %v", err)
	}
	if stats.TotalCount != 0 || stats.TotalSize != 0 {
		t.Fatalf("expected zeroed total stats, got count=%d size=%d", stats.TotalCount, stats.TotalSize)
	}
}

type uploadStatsSnapshot struct {
	TotalCount int64
	TotalSize  int64
}

func loadUploadStats(ctx context.Context) (uploadStatsSnapshot, error) {
	var rows []models.UploadStat
	if err := shared.GetDB(ctx).Where("dimension = ?", models.UploadStatDimensionTotal).Find(&rows).Error; err != nil {
		return uploadStatsSnapshot{}, err
	}
	if len(rows) == 0 {
		return uploadStatsSnapshot{}, nil
	}
	return uploadStatsSnapshot{
		TotalCount: rows[0].FileCount,
		TotalSize:  rows[0].FileSize,
	}, nil
}
