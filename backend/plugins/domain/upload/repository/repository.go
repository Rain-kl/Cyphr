// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package repository 提供上传域数据库仓储层操作。
package repository

import (
	"context"
	"strings"

	"gorm.io/gorm"

	"Wavelet/pkg/util"
	"Wavelet/plugins/domain/upload/models"
	"Wavelet/plugins/domain/upload/shared"
)

// UploadListFilter filters paginated upload queries.
type UploadListFilter struct {
	UserID    uint64
	Keyword   string
	Type      string
	Extension string
	Page      int
	PageSize  int
}

// ListUploads returns paginated upload records matching the filter.
func ListUploads(ctx context.Context, filter UploadListFilter) (int64, []models.Upload, error) {
	query := shared.GetDB(ctx).Model(&models.Upload{}).
		Where("status != ?", models.UploadStatusDeleted)

	if filter.UserID != 0 {
		query = query.Where("user_id = ?", filter.UserID)
	}
	if filter.Keyword != "" {
		query = query.Where("LOWER(file_name) LIKE ? ESCAPE '\\'", "%"+util.EscapeLike(strings.ToLower(filter.Keyword))+"%")
	}
	if filter.Type != "" {
		query = query.Where("type = ?", filter.Type)
	}
	if filter.Extension != "" {
		query = query.Where("extension = ?", strings.ToLower(filter.Extension))
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return 0, nil, err
	}

	var items []models.Upload
	offset := (filter.Page - 1) * filter.PageSize
	if err := query.Order("created_at DESC").Offset(offset).Limit(filter.PageSize).Find(&items).Error; err != nil {
		return 0, nil, err
	}
	return total, items, nil
}

// GetActiveUploadByID loads a non-deleted upload by ID.
func GetActiveUploadByID(ctx context.Context, id uint64) (models.Upload, error) {
	var upload models.Upload
	if err := shared.GetDB(ctx).Where("id = ? AND status != ?", id, models.UploadStatusDeleted).First(&upload).Error; err != nil {
		return models.Upload{}, err
	}
	return upload, nil
}

// SoftDeleteUpload marks an upload as deleted.
func SoftDeleteUpload(ctx context.Context, upload *models.Upload) error {
	return SoftDeleteUploadTx(shared.GetDB(ctx), upload)
}

// SoftDeleteUploadTx marks an upload as deleted within an existing transaction.
func SoftDeleteUploadTx(tx *gorm.DB, upload *models.Upload) error {
	return tx.Model(upload).Update("status", models.UploadStatusDeleted).Error
}

// UpdateUpload applies partial field updates to an upload record.
func UpdateUpload(ctx context.Context, upload *models.Upload, updates map[string]any) error {
	if len(updates) == 0 {
		return nil
	}
	return shared.GetDB(ctx).Model(upload).Updates(updates).Error
}

// ListDistinctUploadTypes returns all distinct non-empty upload business types.
func ListDistinctUploadTypes(ctx context.Context) ([]string, error) {
	var types []string
	if err := shared.GetDB(ctx).Model(&models.Upload{}).
		Where("type IS NOT NULL AND type != ''").
		Distinct().
		Pluck("type", &types).Error; err != nil {
		return nil, err
	}
	return types, nil
}

// FindReusableUploadByHash finds an existing upload with the same hash and size.
func FindReusableUploadByHash(ctx context.Context, hash string, size int64) (models.Upload, error) {
	var existing models.Upload
	err := shared.GetDB(ctx).
		Where("hash = ? AND file_size = ? AND status IN (?, ?)", hash, size, models.UploadStatusPending, models.UploadStatusUsed).
		First(&existing).Error
	return existing, err
}

// CreateUpload persists a new upload record.
func CreateUpload(ctx context.Context, upload *models.Upload) error {
	return CreateUploadTx(shared.GetDB(ctx), upload)
}

// CreateUploadTx persists a new upload record within an existing transaction.
func CreateUploadTx(tx *gorm.DB, upload *models.Upload) error {
	return tx.Create(upload).Error
}

// ListUploadsByIDs returns active uploads matching the given IDs.
func ListUploadsByIDs(ctx context.Context, ids []uint64) ([]models.Upload, error) {
	var uploads []models.Upload
	if err := shared.GetDB(ctx).
		Where("id IN ? AND status IN (?, ?)", ids, models.UploadStatusPending, models.UploadStatusUsed).
		Find(&uploads).Error; err != nil {
		return nil, err
	}
	return uploads, nil
}

// UploadQuery returns a scoped GORM query for uploads.
func UploadQuery(ctx context.Context) *gorm.DB {
	return shared.GetDB(ctx).Model(&models.Upload{})
}

// ListUploadStats returns all upload statistics rows.
func ListUploadStats(ctx context.Context) ([]models.UploadStat, error) {
	var stats []models.UploadStat
	if err := shared.GetDB(ctx).Find(&stats).Error; err != nil {
		return nil, err
	}
	return stats, nil
}
