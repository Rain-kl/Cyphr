// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"Wavelet/pkg/testhelper"
	"Wavelet/plugins/domain/upload/models"
	"Wavelet/plugins/domain/upload/shared"
	"context"
	"testing"

	"gorm.io/gorm"
)

func init() {
	testhelper.RegisterCleanup(func() {
		ResetUploadMetaCacheForTest()
	})
}

func seedUpload(t *testing.T, dbConn *gorm.DB, upload models.Upload) {
	t.Helper()
	if err := dbConn.Create(&upload).Error; err != nil {
		t.Fatalf("create upload: %v", err)
	}
}

func TestGetUploadByIDLoadsFromDBAndPopulatesCache(t *testing.T) {
	dbConn, cleanup := shared.SetupTestEnv(t)
	defer cleanup()
	ResetUploadMetaCacheForTest()

	ctx := context.Background()
	upload := models.Upload{
		ID:         91001,
		UserID:     1,
		FileName:   "cached.png",
		FilePath:   "cached.png",
		FileSize:   12,
		MimeType:   "image/png",
		Extension:  "png",
		Type:       "avatar",
		Status:     models.UploadStatusUsed,
		AccessMode: 1,
	}
	seedUpload(t, dbConn, upload)

	got, err := GetUploadByID(ctx, upload.ID)
	if err != nil {
		t.Fatalf("GetUploadByID: %v", err)
	}
	if got.ID != upload.ID || got.FileName != upload.FileName {
		t.Fatalf("unexpected upload: %+v", got)
	}

	if err := dbConn.Delete(&models.Upload{}, upload.ID).Error; err != nil {
		t.Fatalf("delete upload from db: %v", err)
	}

	gotCached, err := GetUploadByID(ctx, upload.ID)
	if err != nil {
		t.Fatalf("GetUploadByID from RAM cache: %v", err)
	}
	if gotCached.ID != upload.ID {
		t.Fatalf("expected RAM cache hit for upload %d", upload.ID)
	}
}

func TestGetUploadByIDReadsFromRedisWhenRAMEmpty(t *testing.T) {
	dbConn, cleanup := shared.SetupTestEnv(t)
	defer cleanup()
	ResetUploadMetaCacheForTest()

	ctx := context.Background()
	upload := models.Upload{
		ID:         91002,
		UserID:     1,
		FileName:   "redis.png",
		FilePath:   "redis.png",
		FileSize:   8,
		MimeType:   "image/png",
		Extension:  "png",
		Type:       "avatar",
		Status:     models.UploadStatusPending,
		AccessMode: 0,
	}
	seedUpload(t, dbConn, upload)
	SetUploadMetaCache(ctx, &upload)
	ResetUploadMetaCacheForTest()

	if err := dbConn.Delete(&models.Upload{}, upload.ID).Error; err != nil {
		t.Fatalf("delete upload from db: %v", err)
	}

	got, err := GetUploadByID(ctx, upload.ID)
	if err != nil {
		t.Fatalf("GetUploadByID from redis: %v", err)
	}
	if got.ID != upload.ID || got.FileName != upload.FileName {
		t.Fatalf("unexpected upload from redis: %+v", got)
	}
}

func TestInvalidateUploadMetaCacheClearsRAMAndRedis(t *testing.T) {
	dbConn, cleanup := shared.SetupTestEnv(t)
	defer cleanup()
	ResetUploadMetaCacheForTest()

	ctx := context.Background()
	upload := models.Upload{
		ID:         91003,
		UserID:     1,
		FileName:   "invalidate.png",
		FilePath:   "invalidate.png",
		FileSize:   4,
		MimeType:   "image/png",
		Extension:  "png",
		Type:       "avatar",
		Status:     models.UploadStatusUsed,
		AccessMode: 1,
	}
	seedUpload(t, dbConn, upload)
	SetUploadMetaCache(ctx, &upload)

	InvalidateUploadMetaCache(ctx, upload.ID)

	got, err := GetUploadByID(ctx, upload.ID)
	if err != nil {
		t.Fatalf("GetUploadByID after invalidate should reload from DB: %v", err)
	}
	if got.ID != upload.ID {
		t.Fatalf("unexpected upload reloaded from DB: %+v", got)
	}
}

func TestGetUploadByIDSkipsDeletedUploads(t *testing.T) {
	dbConn, cleanup := shared.SetupTestEnv(t)
	defer cleanup()
	ResetUploadMetaCacheForTest()

	ctx := context.Background()
	upload := models.Upload{
		ID:         91004,
		UserID:     1,
		FileName:   "deleted.png",
		FilePath:   "deleted.png",
		FileSize:   4,
		MimeType:   "image/png",
		Extension:  "png",
		Type:       "avatar",
		Status:     models.UploadStatusDeleted,
		AccessMode: 1,
	}
	seedUpload(t, dbConn, upload)

	if _, err := GetUploadByID(ctx, upload.ID); err == nil {
		t.Fatal("expected error for deleted upload")
	}
}
