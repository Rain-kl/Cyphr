// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package upload

import (
	"Wavelet/core/contracts"
	"Wavelet/plugins/domain/upload/ingest"
	"Wavelet/plugins/domain/upload/models"
	"Wavelet/plugins/domain/upload/repository"
	"Wavelet/plugins/domain/upload/shared"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
)

type uploadServiceImpl struct{}

func (s *uploadServiceImpl) GetByID(ctx context.Context, id uint64) (*contracts.UploadDTO, error) {
	u, err := repository.GetActiveUploadByID(ctx, id)
	if err != nil {
		return nil, err
	}
	dto := toUploadDTO(&u)
	return &dto, nil
}

func (s *uploadServiceImpl) OpenStoredUpload(ctx context.Context, id uint64) (*contracts.OpenedUploadDTO, error) {
	u, err := repository.GetActiveUploadByID(ctx, id)
	if err != nil {
		return nil, err
	}
	storageSvc := shared.GetStorage(ctx)
	if storageSvc == nil {
		return nil, errors.New("storage service not available")
	}
	obj, err := storageSvc.Get(ctx, u.FilePath)
	if err != nil {
		return nil, err
	}
	return &contracts.OpenedUploadDTO{
		Upload:        toUploadDTO(&u),
		Body:          obj.Body,
		ContentType:   obj.ContentType,
		ContentLength: obj.ContentLength,
	}, nil
}

func (s *uploadServiceImpl) Remove(ctx context.Context, id uint64) error {
	_, err := Remove(ctx, id)
	return err
}

func (s *uploadServiceImpl) RemoveOwned(ctx context.Context, id uint64, userID uint64) error {
	_, err := RemoveOwned(ctx, userID, id)
	return err
}

func (s *uploadServiceImpl) FindByHash(ctx context.Context, hash string, size int64) (*contracts.UploadDTO, error) {
	u, err := FindByHash(ctx, hash, size)
	if err != nil {
		return nil, err
	}
	dto := toUploadDTO(&u)
	return &dto, nil
}

func (s *uploadServiceImpl) RebuildStats(ctx context.Context) error {
	return RebuildUploadStats(ctx)
}

func (s *uploadServiceImpl) Ingest(ctx context.Context, req contracts.UploadIngestRequest) (*contracts.UploadDTO, error) {
	if req.Reader == nil {
		return nil, errors.New("upload: ingest reader is required")
	}

	// Buffer full body to compute SHA-256 hash required by ingest package.
	var buf bytes.Buffer
	size, err := io.Copy(&buf, req.Reader)
	if err != nil {
		return nil, err
	}
	if req.Size > 0 && size != req.Size {
		return nil, errors.New("upload: ingest size mismatch")
	}

	hashWriter := sha256.New()
	_, _ = hashWriter.Write(buf.Bytes())
	fileHash := hex.EncodeToString(hashWriter.Sum(nil))

	result, err := ingest.Ingest(ctx, ingest.Request{
		UserID:             req.UserID,
		Type:               req.Type,
		Reader:             bytes.NewReader(buf.Bytes()),
		Size:               size,
		FileName:           req.FileName,
		MimeType:           req.MimeType,
		Extension:          req.Extension,
		Hash:               fileHash,
		Policy:             ingest.PolicyDedupNewRecord,
		SkipExtensionCheck: req.SkipExtensionCheck,
	})
	if err != nil {
		return nil, err
	}
	dto := toUploadDTO(&result.Upload)
	return &dto, nil
}

func toUploadDTO(u *models.Upload) contracts.UploadDTO {
	return contracts.UploadDTO{
		ID:       u.ID,
		UserID:   u.UserID,
		FileName: u.FileName,
		FilePath: u.FilePath,
		MimeType: u.MimeType,
		Size:     u.FileSize,
		Hash:     u.Hash,
		Status:   string(u.Status),
		Type:     u.Type,
		Metadata: contracts.UploadMetadataDTO{
			Width:        u.Metadata.Width,
			Height:       u.Metadata.Height,
			Duration:     u.Metadata.Duration,
			OriginalMime: u.Metadata.OriginalMime,
			UserAgent:    u.Metadata.UserAgent,
			ClientIP:     u.Metadata.ClientIP,
			Bucket:       u.Metadata.Bucket,
			Extra:        u.Metadata.Extra,
		},
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
}
