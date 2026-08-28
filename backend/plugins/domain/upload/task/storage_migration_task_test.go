// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package task

import (
	"Wavelet/core/contracts"
	"Wavelet/plugins/domain/upload/models"
	"Wavelet/plugins/domain/upload/shared"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	uploadstorage "Wavelet/plugins/domain/upload/storage"
)

func TestMigrationHandlerExecute(t *testing.T) {
	dbConn, cleanup := shared.SetupTestEnv(t)
	defer cleanup()

	sourceRoot := t.TempDir()
	sourcePath := filepath.Join(sourceRoot, "uploads", "test.txt")
	if err := os.MkdirAll(filepath.Dir(sourcePath), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) returned error: %v", sourcePath, err)
	}
	const content = "storage migration"
	if err := os.WriteFile(sourcePath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) returned error: %v", sourcePath, err)
	}

	ctx := context.Background()
	active := contracts.StorageConfigDTO{
		Driver: contracts.StorageDriverLocal,
		Local:  contracts.LocalStorageConfigDTO{Root: sourceRoot},
	}
	if err := uploadstorage.SaveActiveConfig(ctx, active); err != nil {
		t.Fatalf("SaveActiveConfig() returned error: %v", err)
	}
	target := contracts.StorageConfigDTO{
		Driver: contracts.StorageDriverS3,
		S3: contracts.ObjectStorageConfigDTO{
			Region:          "us-east-1",
			Bucket:          "target",
			AccessKeyID:     "key",
			SecretAccessKey: "secret",
		},
	}
	payload, err := json.Marshal(struct {
		Target contracts.StorageConfigDTO `json:"target"`
	}{Target: target})
	if err != nil {
		t.Fatalf("Marshal(storageMigrationPayload) returned error: %v", err)
	}

	upload := models.Upload{
		ID:        99101,
		UserID:    1,
		FileName:  "test.txt",
		FilePath:  sourcePath,
		FileSize:  int64(len(content)),
		MimeType:  "text/plain",
		Extension: "txt",
		Hash:      "hash",
		Type:      "attachment",
		Status:    models.UploadStatusUsed,
	}
	if err := dbConn.Create(&upload).Error; err != nil {
		t.Fatalf("Create(upload) returned error: %v", err)
	}

	result, err := (&MigrationHandler{}).Execute(ctx, payload)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}
	if result == nil {
		t.Fatal("Execute() result = nil, want non-nil")
	}

	var migrated models.Upload
	if err := dbConn.First(&migrated, upload.ID).Error; err != nil {
		t.Fatalf("First(upload) returned error: %v", err)
	}
	current, err := uploadstorage.LoadStorageConfig(ctx)
	if err != nil {
		t.Fatalf("LoadStorageConfig() returned error: %v", err)
	}
	if current.Driver != contracts.StorageDriverS3 {
		t.Errorf("active driver = %q, want %q", current.Driver, contracts.StorageDriverS3)
	}
}

func TestMigrationHandlerExecuteWithHashValidation(t *testing.T) {
	dbConn, cleanup := shared.SetupTestEnv(t)
	defer cleanup()

	sourceRoot := t.TempDir()
	sourcePath := filepath.Join(sourceRoot, "uploads", "test-hash.txt")
	if err := os.MkdirAll(filepath.Dir(sourcePath), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) returned error: %v", sourcePath, err)
	}
	const content = "storage migration integrity check content"
	if err := os.WriteFile(sourcePath, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) returned error: %v", sourcePath, err)
	}

	// Calculate correct SHA-256 hash
	h := sha256.New()
	h.Write([]byte(content))
	correctHash := hex.EncodeToString(h.Sum(nil))

	ctx := context.Background()
	active := contracts.StorageConfigDTO{
		Driver: contracts.StorageDriverLocal,
		Local:  contracts.LocalStorageConfigDTO{Root: sourceRoot},
	}
	if err := uploadstorage.SaveActiveConfig(ctx, active); err != nil {
		t.Fatalf("SaveActiveConfig() returned error: %v", err)
	}

	target := contracts.StorageConfigDTO{
		Driver: contracts.StorageDriverS3,
		S3: contracts.ObjectStorageConfigDTO{
			Region:          "us-east-1",
			Bucket:          "target",
			AccessKeyID:     "key",
			SecretAccessKey: "secret",
		},
	}
	payload, err := json.Marshal(struct {
		Target contracts.StorageConfigDTO `json:"target"`
	}{Target: target})
	if err != nil {
		t.Fatalf("Marshal(storageMigrationPayload) returned error: %v", err)
	}

	// Case 1: Incorrect Hash (should fail validation)
	uploadIncorrect := models.Upload{
		ID:        99102,
		UserID:    1,
		FileName:  "test-hash.txt",
		FilePath:  sourcePath,
		FileSize:  int64(len(content)),
		MimeType:  "text/plain",
		Extension: "txt",
		Hash:      "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", // Invalid hash
		Type:      "attachment",
		Status:    models.UploadStatusUsed,
	}
	if err := dbConn.Create(&uploadIncorrect).Error; err != nil {
		t.Fatalf("Create(uploadIncorrect) returned error: %v", err)
	}

	// Running execution with incorrect hash should fail with integrity error
	_, err = (&MigrationHandler{}).Execute(ctx, payload)
	if err == nil {
		t.Fatal("Execute() succeeded with incorrect hash, want error")
	}
	if !strings.Contains(err.Error(), "integrity check failed") {
		t.Errorf("expected integrity check failed error, got: %v", err)
	}

	// Case 2: Correct Hash (should succeed)
	if err := dbConn.Model(&models.Upload{}).Where("id = ?", uploadIncorrect.ID).Update("hash", correctHash).Error; err != nil {
		t.Fatalf("Update hash to correct value returned error: %v", err)
	}

	// Run execution with correct hash should succeed
	result, err := (&MigrationHandler{}).Execute(ctx, payload)
	if err != nil {
		t.Fatalf("Execute() with correct hash failed: %v", err)
	}
	if result == nil {
		t.Fatal("Execute() result = nil, want non-nil")
	}

	var migrated models.Upload
	if err := dbConn.First(&migrated, uploadIncorrect.ID).Error; err != nil {
		t.Fatalf("First(upload) returned error: %v", err)
	}
	if migrated.FilePath != sourcePath {
		t.Errorf("FilePath = %q, want %q", migrated.FilePath, sourcePath)
	}
}

func TestMigrationHandlerExecuteWithLock(t *testing.T) {
	_, cleanup := shared.SetupTestEnv(t)
	defer cleanup()

	ctx := context.Background()
	cacheSvc := shared.GetCache(ctx)
	if cacheSvc != nil {
		_ = cacheSvc.Set(ctx, "lock:storage:migrate", "locked", 3600)
	}

	active := contracts.StorageConfigDTO{
		Driver: contracts.StorageDriverLocal,
	}
	if err := uploadstorage.SaveActiveConfig(ctx, active); err != nil {
		t.Fatalf("SaveActiveConfig() returned error: %v", err)
	}

	payload, err := json.Marshal(struct {
		Target contracts.StorageConfigDTO `json:"target"`
	}{Target: active})
	if err != nil {
		t.Fatalf("Marshal payload failed: %v", err)
	}

	// Execution should fail because lock is already acquired
	_, err = (&MigrationHandler{}).Execute(ctx, payload)
	if err == nil {
		t.Fatal("Execute() succeeded when lock was held, want error")
	}
	if !strings.Contains(err.Error(), "另一个存储迁移任务正在运行中") {
		t.Errorf("expected lock warning, got: %v", err)
	}

	// Release lock and run again, should succeed
	if cacheSvc != nil {
		_ = cacheSvc.Delete(ctx, "lock:storage:migrate")
	}

	_, err = (&MigrationHandler{}).Execute(ctx, payload)
	if err != nil {
		t.Fatalf("Execute() failed after lock released: %v", err)
	}
}
