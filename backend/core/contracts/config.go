// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package contracts

import (
	"context"
	"time"
)

// SystemConfigDTO represents a system configuration key-value entry.
type SystemConfigDTO struct {
	Key         string    `json:"key" gorm:"primaryKey;column:key;size:255"`
	Value       string    `json:"value" gorm:"column:value;type:text"`
	Type        string    `json:"type" gorm:"column:type;size:50"`
	Visibility  int       `json:"visibility" gorm:"column:visibility"`
	Description string    `json:"description" gorm:"column:description;size:500"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"column:updated_at"`
	CreatedAt   time.Time `json:"created_at" gorm:"column:created_at"`
}

// TableName returns the default table name for SystemConfigDTO.
func (SystemConfigDTO) TableName() string {
	return "w_system_configs"
}

// SystemConfigService defines the unified contract for querying and mutating system configurations.
type SystemConfigService interface {
	GetByKey(ctx context.Context, key string) (SystemConfigDTO, error)
	ListByKeys(ctx context.Context, keys []string) (map[string]SystemConfigDTO, error)
	ListVisible(ctx context.Context) ([]SystemConfigDTO, error)
	ListByType(ctx context.Context, configType string) ([]SystemConfigDTO, error)
	GetIntByKey(ctx context.Context, key string) (int, error)
	GetBoolByKey(ctx context.Context, key string) (bool, error)
	SaveOrUpdate(ctx context.Context, key, value string) error
	InvalidateCache(ctx context.Context, key string) error
	InvalidateAllCaches(ctx context.Context) error
}
