// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package task

import (
	"context"
	"testing"
	"time"

	"Wavelet/pkg/testhelper"
	"Wavelet/plugins/domain/upload/models"
	database "Wavelet/plugins/infra/database"
)

func TestRebuildUploadStatsHandler_Execute(t *testing.T) {
	_, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now()

	uploads := []models.Upload{
		{
			UserID: 1001, FileName: "a.jpg", FilePath: "uploads/a.jpg",
			FileSize: 100, MimeType: "image/jpeg", Extension: "jpg", Hash: "hash-a",
			Type: "pixez_mirror", Status: models.UploadStatusUsed, CreatedAt: now,
		},
		{
			UserID: 1001, FileName: "b.png", FilePath: "uploads/b.png",
			FileSize: 200, MimeType: "image/png", Extension: "png", Hash: "hash-b",
			Type: "attachment", Status: models.UploadStatusUsed, CreatedAt: now,
		},
	}
	for i := range uploads {
		if err := database.DB(ctx).Create(&uploads[i]).Error; err != nil {
			t.Fatalf("seed upload failed: %v", err)
		}
	}

	// Corrupt stats to ensure rebuild recalculates from uploads.
	if err := database.DB(ctx).Create(&models.UploadStat{
		Dimension: models.UploadStatDimensionTotal,
		StatKey:   "",
		FileCount: 0,
		FileSize:  0,
	}).Error; err != nil {
		t.Fatalf("seed broken total stat failed: %v", err)
	}

	handler := &RebuildUploadStatsHandler{}
	result, err := handler.Execute(ctx, nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result == nil || result.Message == "" {
		t.Fatalf("Execute() returned empty result: %+v", result)
	}

	var totalStat models.UploadStat
	if err := database.DB(ctx).
		Where("dimension = ? AND stat_key = ?", models.UploadStatDimensionTotal, "").
		First(&totalStat).Error; err != nil {
		t.Fatalf("load total stat failed: %v", err)
	}
	if totalStat.FileCount != 2 || totalStat.FileSize != 300 {
		t.Fatalf("total stat = count %d size %d, want 2 / 300", totalStat.FileCount, totalStat.FileSize)
	}
}
