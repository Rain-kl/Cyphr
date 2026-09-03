// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package dao

import (
	"Wavelet/pkg/util"
	"Wavelet/transcribe/plugins/svr/model/entity"
	"context"
	"time"

	"gorm.io/gorm"
)

// NodeDAO defines data access methods for t_nodes.
type NodeDAO interface {
	GetByTokenHash(ctx context.Context, hash string) (*entity.NodeEntity, error)
	GetByID(ctx context.Context, id uint64) (*entity.NodeEntity, error)
	ListAll(ctx context.Context, keyword ...string) ([]entity.NodeEntity, error)
	Create(ctx context.Context, node *entity.NodeEntity) error
	UpdateLastSeen(ctx context.Context, id uint64, ip string) error
	Delete(ctx context.Context, id uint64) error
}

// GormNodeDAO implements NodeDAO using GORM.
type GormNodeDAO struct {
	db *gorm.DB
}

var _ NodeDAO = (*GormNodeDAO)(nil)

// NewNodeDAO creates a new NodeDAO.
func NewNodeDAO(db *gorm.DB) NodeDAO {
	return &GormNodeDAO{db: db}
}

// GetByTokenHash retrieves an active or inactive node by token hash.
func (d *GormNodeDAO) GetByTokenHash(ctx context.Context, hash string) (*entity.NodeEntity, error) {
	var node entity.NodeEntity
	if err := d.db.WithContext(ctx).Where("token_hash = ?", hash).First(&node).Error; err != nil {
		return nil, mapNotFound(err)
	}
	return &node, nil
}

// GetByID retrieves a node by ID.
func (d *GormNodeDAO) GetByID(ctx context.Context, id uint64) (*entity.NodeEntity, error) {
	var node entity.NodeEntity
	if err := d.db.WithContext(ctx).Where("id = ?", id).First(&node).Error; err != nil {
		return nil, mapNotFound(err)
	}
	return &node, nil
}

// ListAll lists all nodes with optional keyword filtering.
func (d *GormNodeDAO) ListAll(ctx context.Context, keyword ...string) ([]entity.NodeEntity, error) {
	var list []entity.NodeEntity
	query := d.db.WithContext(ctx)
	if len(keyword) > 0 && keyword[0] != "" {
		escaped := util.EscapeLike(keyword[0])
		query = query.Where("name LIKE ? ESCAPE '\\'", "%"+escaped+"%")
	}
	if err := query.Order("id DESC").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// Create inserts a new node record.
func (d *GormNodeDAO) Create(ctx context.Context, node *entity.NodeEntity) error {
	if node.ID == 0 {
		node.ID = generateID(node.ID)
	}
	return d.db.WithContext(ctx).Create(node).Error
}

// UpdateLastSeen updates node's last seen timestamp and optional last IP.
func (d *GormNodeDAO) UpdateLastSeen(ctx context.Context, id uint64, ip string) error {
	now := time.Now()
	updates := map[string]any{
		"last_seen_at": &now,
	}
	if ip != "" {
		updates["last_ip"] = ip
	}
	res := d.db.WithContext(ctx).Model(&entity.NodeEntity{}).Where("id = ?", id).Updates(updates)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return mapNotFound(gorm.ErrRecordNotFound)
	}
	return nil
}

// Delete removes a node record by ID.
func (d *GormNodeDAO) Delete(ctx context.Context, id uint64) error {
	res := d.db.WithContext(ctx).Where("id = ?", id).Delete(&entity.NodeEntity{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return mapNotFound(gorm.ErrRecordNotFound)
	}
	return nil
}
