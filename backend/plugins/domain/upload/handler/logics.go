// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"context"
	"errors"
	"sort"

	"gorm.io/gorm"

	"Wavelet/plugins/domain/upload/ingest"
	"Wavelet/plugins/domain/upload/models"
	"Wavelet/plugins/domain/upload/repository"
)

func listUploadFiles(ctx context.Context, filter repository.UploadListFilter) (int64, []models.Upload, error) {
	return repository.ListUploads(ctx, filter)
}

func listMyUploadFiles(ctx context.Context, userID uint64, filter repository.UploadListFilter) (int64, []models.Upload, error) {
	filter.UserID = userID
	return repository.ListUploads(ctx, filter)
}

func softDeleteUpload(ctx context.Context, uploadID uint64) (models.Upload, error) {
	return ingest.Remove(ctx, uploadID)
}

func softDeleteOwnedUpload(ctx context.Context, userID, uploadID uint64) (models.Upload, error) {
	return ingest.RemoveOwned(ctx, userID, uploadID)
}

func listDistinctUploadTypes(ctx context.Context) ([]string, error) {
	types, err := repository.ListDistinctUploadTypes(ctx)
	if err != nil {
		return nil, err
	}
	sort.Strings(types)
	return types, nil
}

type updateMyUploadInput struct {
	FileName   string
	AccessMode *int
}

func updateOwnedUpload(ctx context.Context, userID, uploadID uint64, input updateMyUploadInput) (models.Upload, error) {
	u, err := repository.GetActiveUploadByID(ctx, uploadID)
	if err != nil {
		return models.Upload{}, err
	}
	if u.UserID != userID {
		return models.Upload{}, ingest.ErrForbidden
	}

	updates := make(map[string]any)
	if input.FileName != "" {
		updates["file_name"] = input.FileName
	}
	if input.AccessMode != nil {
		updates["access_mode"] = *input.AccessMode
	}
	if err := repository.UpdateUpload(ctx, &u, updates); err != nil {
		return models.Upload{}, err
	}
	if name, ok := updates["file_name"].(string); ok {
		u.FileName = name
	}
	if mode, ok := updates["access_mode"].(int); ok {
		u.AccessMode = mode
	}
	return u, nil
}

func listUploadsForBatchDownload(ctx context.Context, ids []uint64) ([]models.Upload, error) {
	return repository.ListUploadsByIDs(ctx, ids)
}

func loadUploadStats(ctx context.Context) ([]models.UploadStat, error) {
	return repository.ListUploadStats(ctx)
}

func isRecordNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}
