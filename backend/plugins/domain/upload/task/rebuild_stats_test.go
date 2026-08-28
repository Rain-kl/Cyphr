// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package task

import (
	"Wavelet/plugins/domain/upload/models"
	"Wavelet/plugins/domain/upload/shared"
	"context"
	"testing"
	"time"
)

func TestRebuildUploadStatsHandler_Execute(t *testing.T) {
	_, cleanup := shared.SetupTestEnv(t)
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
	db := shared.GetDB(ctx)
	for i := range uploads {
		if err := db.Create(&uploads[i]).Error; err != nil {
			t.Fatalf("seed upload failed: %v", err)
		}
	}

	// Corrupt stats to ensure rebuild recalculates from uploads.
	if err := db.Create(&models.UploadStat{
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
	if err := db.Where("dimension = ? AND stat_key = ?", models.UploadStatDimensionTotal, "").First(&totalStat).Error; err != nil {
		t.Fatalf("query total stat failed: %v", err)
	}
	if totalStat.FileCount != 2 || totalStat.FileSize != 300 {
		t.Fatalf("total stat mismatch: count=%d size=%d, want count=2 size=300", totalStat.FileCount, totalStat.FileSize)
	}
}
