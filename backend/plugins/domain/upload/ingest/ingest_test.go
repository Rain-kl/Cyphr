// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package ingest

import (
	"Wavelet/core/contracts"
	"Wavelet/plugins/domain/upload/models"
	"Wavelet/plugins/domain/upload/shared"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"sync"
	"testing"
)

type testStorageService struct {
	mu        sync.RWMutex
	mockFiles map[string][]byte
	putCount  *int
}

func (s *testStorageService) Put(_ context.Context, key string, body io.Reader, _ int64, _ string) (contracts.StoragePutResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := io.ReadAll(body)
	if err != nil {
		return contracts.StoragePutResult{}, err
	}
	s.mockFiles[key] = data
	if s.putCount != nil {
		*s.putCount++
	}
	return contracts.StoragePutResult{Key: key, Bucket: "test-bucket"}, nil
}

func (s *testStorageService) Get(_ context.Context, key string) (*contracts.StorageObject, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, ok := s.mockFiles[key]
	if !ok {
		return nil, os.ErrNotExist
	}
	return &contracts.StorageObject{
		Key:           key,
		Body:          io.NopCloser(bytes.NewReader(data)),
		ContentLength: int64(len(data)),
		ContentType:   "application/octet-stream",
	}, nil
}

func (s *testStorageService) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.mockFiles, key)
	return nil
}

func (s *testStorageService) Ingest(_ context.Context, _ io.Reader, _ contracts.IngestOptions) (*contracts.IngestResult, error) {
	return nil, nil
}

func setupMockStorage(t *testing.T, putCount *int) (restore, disable func()) {
	t.Helper()
	mockSvc := &testStorageService{
		mockFiles: make(map[string][]byte),
		putCount:  putCount,
	}
	shared.SetStorageService(mockSvc)
	return func() {
			shared.SetStorageService(nil)
		}, func() {
			shared.SetStorageService(nil)
		}
}

func loadTotalStats(ctx context.Context) (totalStatsSnapshot, error) {
	var rows []models.UploadStat
	if err := shared.GetDB(ctx).Where("dimension = ?", models.UploadStatDimensionTotal).Find(&rows).Error; err != nil {
		return totalStatsSnapshot{}, err
	}
	if len(rows) == 0 {
		return totalStatsSnapshot{}, nil
	}
	return totalStatsSnapshot{
		TotalCount: rows[0].FileCount,
		TotalSize:  rows[0].FileSize,
	}, nil
}

type totalStatsSnapshot struct {
	TotalCount int64
	TotalSize  int64
}

func TestIngestPolicyCreateIncrementsStats(t *testing.T) {
	_, cleanup := shared.SetupTestEnv(t)
	defer cleanup()
	ctx := context.Background()

	content := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01")
	hash := sha256.Sum256(content)

	restoreStorage, disableStorage := setupMockStorage(t, nil)
	defer restoreStorage()
	defer disableStorage()

	result, err := Ingest(ctx, Request{
		UserID:    1001,
		Reader:    bytes.NewReader(content),
		Size:      int64(len(content)),
		FileName:  "mirror.png",
		MimeType:  "image/png",
		Extension: "png",
		Hash:      hex.EncodeToString(hash[:]),
		Type:      "pixez_mirror",
		Policy:    PolicyCreate,
	})
	if err != nil {
		t.Fatalf("Ingest(PolicyCreate) returned error: %v", err)
	}
	if !result.Created || !result.Stored || result.Resolved {
		t.Fatalf("Ingest(PolicyCreate) = %+v, want Created+Stored without Resolved", result)
	}

	stats, err := loadTotalStats(ctx)
	if err != nil {
		t.Fatalf("loadTotalStats returned error: %v", err)
	}
	if stats.TotalCount != 1 || stats.TotalSize != int64(len(content)) {
		t.Fatalf("loadTotalStats() = count %d size %d, want count 1 size %d", stats.TotalCount, stats.TotalSize, len(content))
	}
}

func TestIngestPolicyResolveExistingSkipsStatsOnHit(t *testing.T) {
	_, cleanup := shared.SetupTestEnv(t)
	defer cleanup()
	ctx := context.Background()

	content := []byte("hello duplicate resolution")
	hash := sha256.Sum256(content)
	hashStr := hex.EncodeToString(hash[:])

	putCount := 0
	restoreStorage, disableStorage := setupMockStorage(t, &putCount)
	defer restoreStorage()
	defer disableStorage()

	first, err := Ingest(ctx, Request{
		UserID:    1001,
		Reader:    bytes.NewReader(content),
		Size:      int64(len(content)),
		FileName:  "first.txt",
		MimeType:  "text/plain",
		Extension: "txt",
		Hash:      hashStr,
		Type:      "attachment",
		Policy:    PolicyCreate,
	})
	if err != nil {
		t.Fatalf("first Ingest returned error: %v", err)
	}
	if !first.Created || !first.Stored {
		t.Fatalf("first Ingest = %+v, want Created and Stored true", first)
	}
	if putCount != 1 {
		t.Fatalf("putCount = %d, want 1 after initial store", putCount)
	}

	second, err := Ingest(ctx, Request{
		UserID:    1002,
		Reader:    bytes.NewReader(content),
		Size:      int64(len(content)),
		FileName:  "second.txt",
		MimeType:  "text/plain",
		Extension: "txt",
		Hash:      hashStr,
		Type:      "attachment",
		Policy:    PolicyResolveExisting,
	})
	if err != nil {
		t.Fatalf("second Ingest(PolicyResolveExisting) returned error: %v", err)
	}
	if second.Created || second.Stored || !second.Resolved {
		t.Fatalf("second Ingest = %+v, want Created/Stored false and Resolved true", second)
	}
	if second.Upload.ID != first.Upload.ID {
		t.Fatalf("resolved ID = %d, want %d", second.Upload.ID, first.Upload.ID)
	}
	if putCount != 1 {
		t.Fatalf("putCount = %d, want 1 after hit with PolicyResolveExisting", putCount)
	}

	stats, err := loadTotalStats(ctx)
	if err != nil {
		t.Fatalf("loadTotalStats returned error: %v", err)
	}
	if stats.TotalCount != 1 || stats.TotalSize != int64(len(content)) {
		t.Fatalf("stats = %+v, want count 1 size %d", stats, len(content))
	}
}

func TestIngestPolicyDedupNewRecordReusesStorage(t *testing.T) {
	_, cleanup := shared.SetupTestEnv(t)
	defer cleanup()
	ctx := context.Background()

	content := []byte("hello dedup reuse")
	hash := sha256.Sum256(content)
	hashStr := hex.EncodeToString(hash[:])

	putCount := 0
	restoreStorage, disableStorage := setupMockStorage(t, &putCount)
	defer restoreStorage()
	defer disableStorage()

	first, err := Ingest(ctx, Request{
		UserID:    1001,
		Reader:    bytes.NewReader(content),
		Size:      int64(len(content)),
		FileName:  "first.txt",
		MimeType:  "text/plain",
		Extension: "txt",
		Hash:      hashStr,
		Type:      "attachment",
		Policy:    PolicyCreate,
	})
	if err != nil {
		t.Fatalf("first Ingest: %v", err)
	}

	second, err := Ingest(ctx, Request{
		UserID:    1002,
		Reader:    bytes.NewReader(content),
		Size:      int64(len(content)),
		FileName:  "second.txt",
		MimeType:  "text/plain",
		Extension: "txt",
		Hash:      hashStr,
		Type:      "attachment",
		Policy:    PolicyDedupNewRecord,
	})
	if err != nil {
		t.Fatalf("second Ingest(PolicyDedupNewRecord): %v", err)
	}
	if !second.Created || second.Stored || second.Resolved {
		t.Fatalf("second Ingest = %+v, want Created=true Stored=false Resolved=false", second)
	}
	if second.Upload.ID == first.Upload.ID {
		t.Fatalf("expected new upload record, got matching ID %d", second.Upload.ID)
	}
	if second.Upload.FilePath != first.Upload.FilePath {
		t.Fatalf("expected reused FilePath %q, got %q", first.Upload.FilePath, second.Upload.FilePath)
	}
	if putCount != 1 {
		t.Fatalf("putCount = %d, want 1 after dedup new record", putCount)
	}

	stats, err := loadTotalStats(ctx)
	if err != nil {
		t.Fatalf("loadTotalStats: %v", err)
	}
	if stats.TotalCount != 2 || stats.TotalSize != int64(len(content)*2) {
		t.Fatalf("stats = %+v, want count 2 size %d", stats, len(content)*2)
	}
}

func TestRemoveDecrementsStats(t *testing.T) {
	_, cleanup := shared.SetupTestEnv(t)
	defer cleanup()
	ctx := context.Background()

	content := []byte("remove payload")
	hash := sha256.Sum256(content)

	restoreStorage, disableStorage := setupMockStorage(t, nil)
	defer restoreStorage()
	defer disableStorage()

	ingested, err := Ingest(ctx, Request{
		UserID:    1001,
		Reader:    bytes.NewReader(content),
		Size:      int64(len(content)),
		FileName:  "to_remove.txt",
		MimeType:  "text/plain",
		Extension: "txt",
		Hash:      hex.EncodeToString(hash[:]),
		Type:      "generic",
		Policy:    PolicyCreate,
	})
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}

	removed, err := Remove(ctx, ingested.Upload.ID)
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if removed.Status != models.UploadStatusDeleted {
		t.Fatalf("removed status = %q, want deleted", removed.Status)
	}

	stats, err := loadTotalStats(ctx)
	if err != nil {
		t.Fatalf("loadTotalStats: %v", err)
	}
	if stats.TotalCount != 0 || stats.TotalSize != 0 {
		t.Fatalf("stats = %+v, want count 0 size 0 after remove", stats)
	}
}

func TestRemoveOwnedEnforcesOwnership(t *testing.T) {
	_, cleanup := shared.SetupTestEnv(t)
	defer cleanup()
	ctx := context.Background()

	content := []byte("owner payload")
	hash := sha256.Sum256(content)

	restoreStorage, disableStorage := setupMockStorage(t, nil)
	defer restoreStorage()
	defer disableStorage()

	ingested, err := Ingest(ctx, Request{
		UserID:    1001,
		Reader:    bytes.NewReader(content),
		Size:      int64(len(content)),
		FileName:  "owned.txt",
		MimeType:  "text/plain",
		Extension: "txt",
		Hash:      hex.EncodeToString(hash[:]),
		Type:      "generic",
		Policy:    PolicyCreate,
	})
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}

	if _, err := RemoveOwned(ctx, 2002, ingested.Upload.ID); err == nil {
		t.Fatal("expected ErrForbidden for non-owner RemoveOwned")
	}

	removed, err := RemoveOwned(ctx, 1001, ingested.Upload.ID)
	if err != nil {
		t.Fatalf("RemoveOwned owner failed: %v", err)
	}
	if removed.Status != models.UploadStatusDeleted {
		t.Fatalf("removed status = %q, want deleted", removed.Status)
	}
}
