// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package stats

import (
	"context"
	"time"

	"Wavelet/pkg/logger"
	"Wavelet/plugins/domain/upload/models"
	database "Wavelet/plugins/infra/database"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ApplyUploadStatsAdd increments incremental stats for a newly active upload record.
func ApplyUploadStatsAdd(ctx context.Context, upload *models.Upload) error {
	return applyUploadStatsDelta(ctx, upload, 1)
}

// ApplyUploadStatsRemove decrements incremental stats for a removed active upload record.
func ApplyUploadStatsRemove(ctx context.Context, upload *models.Upload) error {
	return applyUploadStatsDelta(ctx, upload, -1)
}

// RebuildUploadStats rebuilds all incremental stats from current upload records.
func RebuildUploadStats(ctx context.Context) error {
	return database.DB(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("1 = 1").Delete(&models.UploadStat{}).Error; err != nil {
			return err
		}

		var uploads []models.Upload
		if err := tx.Where("status != ?", models.UploadStatusDeleted).Find(&uploads).Error; err != nil {
			return err
		}

		for i := range uploads {
			if err := ApplyUploadStatsDeltaTx(tx, &uploads[i], 1); err != nil {
				return err
			}
		}
		return nil
	})
}

func applyUploadStatsDelta(ctx context.Context, upload *models.Upload, sign int64) error {
	if upload == nil || !isActiveUploadStatus(upload.Status) {
		return nil
	}
	return database.DB(ctx).Transaction(func(tx *gorm.DB) error {
		return ApplyUploadStatsDeltaTx(tx, upload, sign)
	})
}

// ApplyUploadStatsDeltaTx applies incremental upload stats within an existing transaction.
func ApplyUploadStatsDeltaTx(tx *gorm.DB, upload *models.Upload, sign int64) error {
	if upload == nil || !isActiveUploadStatus(upload.Status) || sign == 0 {
		return nil
	}

	countDelta := sign
	sizeDelta := sign * upload.FileSize
	typeKey := upload.Type
	if typeKey == "" {
		typeKey = "generic"
	}

	entries := []struct {
		dimension string
		key       string
	}{
		{models.UploadStatDimensionTotal, ""},
		{models.UploadStatDimensionType, typeKey},
		{models.UploadStatDimensionCategory, GetFileCategory(upload.MimeType, upload.Extension)},
		{models.UploadStatDimensionTrend, upload.CreatedAt.Format("2006-01-02")},
	}

	for _, entry := range entries {
		if err := upsertUploadStatDelta(tx, entry.dimension, entry.key, countDelta, sizeDelta); err != nil {
			return err
		}
	}
	return nil
}

func upsertUploadStatDelta(tx *gorm.DB, dimension, key string, countDelta, sizeDelta int64) error {
	return tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "dimension"},
			{Name: "stat_key"},
		},
		DoUpdates: clause.Assignments(map[string]any{
			"file_count": gorm.Expr(
				"CASE WHEN w_upload_stats.file_count + ? < 0 THEN 0 ELSE w_upload_stats.file_count + ? END",
				countDelta,
				countDelta,
			),
			"file_size": gorm.Expr(
				"CASE WHEN w_upload_stats.file_size + ? < 0 THEN 0 ELSE w_upload_stats.file_size + ? END",
				sizeDelta,
				sizeDelta,
			),
			"updated_at": time.Now(),
		}),
	}).Create(&models.UploadStat{
		Dimension: dimension,
		StatKey:   key,
		FileCount: countDelta,
		FileSize:  sizeDelta,
	}).Error
}

// RecordUploadStatsAdd logs and applies upload stats increment.
func RecordUploadStatsAdd(ctx context.Context, upload *models.Upload) {
	if err := ApplyUploadStatsAdd(ctx, upload); err != nil {
		logger.WarnF(ctx, "increment upload stats failed: %v", err)
	}
}

// RecordUploadStatsRemove logs and applies upload stats decrement.
func RecordUploadStatsRemove(ctx context.Context, upload *models.Upload) {
	if err := ApplyUploadStatsRemove(ctx, upload); err != nil {
		logger.WarnF(ctx, "decrement upload stats failed: %v", err)
	}
}

func isActiveUploadStatus(status models.UploadStatus) bool {
	return status == models.UploadStatusPending || status == models.UploadStatusUsed
}
