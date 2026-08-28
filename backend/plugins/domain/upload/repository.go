// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package upload

import (
	"context"
	"strings"

	"gorm.io/gorm"

	"Wavelet/pkg/idgen"
	"Wavelet/pkg/util"
	database "Wavelet/plugins/infra/database"
)

// UploadListFilter filters paginated upload queries.
//
//nolint:revive
type UploadListFilter struct {
	UserID    uint64
	Keyword   string
	Type      string
	Extension string
	Page      int
	PageSize  int
}

// ListUploads returns paginated upload records matching the filter.
func ListUploads(ctx context.Context, filter UploadListFilter) (int64, []Upload, error) {
	query := database.DB(ctx).Model(&Upload{}).
		Where("status != ?", UploadStatusDeleted)

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

	var items []Upload
	offset := (filter.Page - 1) * filter.PageSize
	if err := query.Order("created_at DESC").Offset(offset).Limit(filter.PageSize).Find(&items).Error; err != nil {
		return 0, nil, err
	}
	return total, items, nil
}

// GetActiveUploadByID loads a non-deleted upload by ID.
func GetActiveUploadByID(ctx context.Context, id uint64) (Upload, error) {
	var upload Upload
	if err := database.DB(ctx).Where("id = ? AND status != ?", id, UploadStatusDeleted).First(&upload).Error; err != nil {
		return Upload{}, err
	}
	return upload, nil
}

// SoftDeleteUpload marks an upload as deleted.
// External modules must use upload.Remove or upload.RemoveOwned; only internal/apps/upload may call this.
func SoftDeleteUpload(ctx context.Context, upload *Upload) error {
	return SoftDeleteUploadTx(database.DB(ctx), upload)
}

// SoftDeleteUploadTx marks an upload as deleted within an existing transaction.
func SoftDeleteUploadTx(tx *gorm.DB, upload *Upload) error {
	return tx.Model(upload).Update("status", UploadStatusDeleted).Error
}

// UpdateUpload applies partial field updates to an upload record.
func UpdateUpload(ctx context.Context, upload *Upload, updates map[string]any) error {
	if len(updates) == 0 {
		return nil
	}
	return database.DB(ctx).Model(upload).Updates(updates).Error
}

// ListDistinctUploadTypes returns all distinct non-empty upload business types.
func ListDistinctUploadTypes(ctx context.Context) ([]string, error) {
	var types []string
	if err := database.DB(ctx).Model(&Upload{}).
		Where("type IS NOT NULL AND type != ''").
		Distinct().
		Pluck("type", &types).Error; err != nil {
		return nil, err
	}
	return types, nil
}

// FindReusableUploadByHash finds an existing upload with the same hash and size.
func FindReusableUploadByHash(ctx context.Context, hash string, size int64) (Upload, error) {
	var existing Upload
	err := database.DB(ctx).
		Where("hash = ? AND file_size = ? AND status IN (?, ?)", hash, size, UploadStatusPending, UploadStatusUsed).
		First(&existing).Error
	return existing, err
}

// CreateUpload persists a new upload record.
func CreateUpload(ctx context.Context, upload *Upload) error {
	return CreateUploadTx(database.DB(ctx), upload)
}

// CreateUploadTx persists a new upload record within an existing transaction.
func CreateUploadTx(tx *gorm.DB, upload *Upload) error {
	if upload.ID == 0 {
		upload.ID = idgen.NextUint64ID()
	}
	return tx.Create(upload).Error
}

// ListUploadsByIDs returns active uploads matching the given IDs.
func ListUploadsByIDs(ctx context.Context, ids []uint64) ([]Upload, error) {
	var uploads []Upload
	if err := database.DB(ctx).
		Where("id IN ? AND status IN (?, ?)", ids, UploadStatusPending, UploadStatusUsed).
		Find(&uploads).Error; err != nil {
		return nil, err
	}
	return uploads, nil
}

// UploadQuery returns a scoped GORM query for uploads.
//
//nolint:revive
func UploadQuery(ctx context.Context) *gorm.DB {
	return database.DB(ctx).Model(&Upload{})
}

// ListUploadStats returns all upload statistics rows.
func ListUploadStats(ctx context.Context) ([]UploadStat, error) {
	var stats []UploadStat
	if err := database.DB(ctx).Find(&stats).Error; err != nil {
		return nil, err
	}
	return stats, nil
}
