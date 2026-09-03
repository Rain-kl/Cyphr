// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package dao

import (
	"Wavelet/pkg/util"
	"Wavelet/transcribe/plugins/svr/model/entity"
	"context"

	"gorm.io/gorm"
)

// ModelDAO defines data access methods for t_models.
type ModelDAO interface {
	GetByName(ctx context.Context, name string) (*entity.ModelEntity, error)
	ListActive(ctx context.Context, keyword ...string) ([]entity.ModelEntity, error)
	Create(ctx context.Context, model *entity.ModelEntity) error
}

// GormModelDAO implements ModelDAO using GORM.
type GormModelDAO struct {
	db *gorm.DB
}

var _ ModelDAO = (*GormModelDAO)(nil)

// NewModelDAO creates a new ModelDAO.
func NewModelDAO(db *gorm.DB) ModelDAO {
	return &GormModelDAO{db: db}
}

// GetByName finds a model by unique name.
func (d *GormModelDAO) GetByName(ctx context.Context, name string) (*entity.ModelEntity, error) {
	var m entity.ModelEntity
	if err := d.db.WithContext(ctx).Where("name = ?", name).First(&m).Error; err != nil {
		return nil, mapNotFound(err)
	}
	return &m, nil
}

// ListActive lists all active models, with optional keyword filtering.
func (d *GormModelDAO) ListActive(ctx context.Context, keyword ...string) ([]entity.ModelEntity, error) {
	var list []entity.ModelEntity
	query := d.db.WithContext(ctx).Where("is_active = ?", true)
	if len(keyword) > 0 && keyword[0] != "" {
		escaped := util.EscapeLike(keyword[0])
		query = query.Where("name LIKE ? ESCAPE '\\'", "%"+escaped+"%")
	}
	if err := query.Order("id ASC").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// Create inserts a new model.
func (d *GormModelDAO) Create(ctx context.Context, model *entity.ModelEntity) error {
	if model.ID == 0 {
		model.ID = generateID(model.ID)
	}
	return d.db.WithContext(ctx).Create(model).Error
}
