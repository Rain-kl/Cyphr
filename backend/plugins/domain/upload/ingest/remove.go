// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package ingest

import (
	"context"

	uploadcache "Wavelet/plugins/domain/upload/cache"
	"Wavelet/plugins/domain/upload/models"
	"Wavelet/plugins/domain/upload/repository"
	uploadstats "Wavelet/plugins/domain/upload/stats"
	database "Wavelet/plugins/infra/database"
	"gorm.io/gorm"
)

// Remove soft-deletes an upload and decrements incremental stats.
func Remove(ctx context.Context, uploadID uint64) (models.Upload, error) {
	upload, err := repository.GetActiveUploadByID(ctx, uploadID)
	if err != nil {
		return models.Upload{}, err
	}
	if err := softDeleteUploadWithStats(ctx, &upload); err != nil {
		return models.Upload{}, err
	}
	upload.Status = models.UploadStatusDeleted
	return upload, nil
}

// RemoveOwned soft-deletes an upload owned by userID and decrements incremental stats.
func RemoveOwned(ctx context.Context, userID, uploadID uint64) (models.Upload, error) {
	upload, err := repository.GetActiveUploadByID(ctx, uploadID)
	if err != nil {
		return models.Upload{}, err
	}
	if upload.UserID != userID {
		return models.Upload{}, ErrForbidden
	}
	if err := softDeleteUploadWithStats(ctx, &upload); err != nil {
		return models.Upload{}, err
	}
	upload.Status = models.UploadStatusDeleted
	return upload, nil
}

func softDeleteUploadWithStats(ctx context.Context, upload *models.Upload) error {
	statsSnapshot := *upload
	if err := database.DB(ctx).Transaction(func(tx *gorm.DB) error {
		if err := repository.SoftDeleteUploadTx(tx, upload); err != nil {
			return err
		}
		return uploadstats.ApplyUploadStatsDeltaTx(tx, &statsSnapshot, -1)
	}); err != nil {
		return err
	}
	uploadcache.InvalidateUploadMetaCache(ctx, upload.ID)
	return nil
}
