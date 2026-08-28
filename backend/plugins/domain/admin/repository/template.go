// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"Wavelet/plugins/domain/admin/model"
	"context"
	"errors"

	"gorm.io/gorm"
)

// ListTemplatesRecord returns all templates ordered by system flag and creation time.
func ListTemplatesRecord(ctx context.Context) ([]model.Template, error) {
	var templates []model.Template
	if err := GetDB(ctx).Order("is_system DESC, created_at DESC").Find(&templates).Error; err != nil {
		return nil, err
	}
	return templates, nil
}

// GetTemplateByKey loads a template by its key.
func GetTemplateByKey(ctx context.Context, key string) (model.Template, error) {
	var tmpl model.Template
	if err := GetDB(ctx).Where("key = ?", key).First(&tmpl).Error; err != nil {
		return model.Template{}, err
	}
	return tmpl, nil
}

// TemplateExistsByKey reports whether a template key is already taken.
func TemplateExistsByKey(ctx context.Context, key string) (bool, error) {
	var existing model.Template
	err := GetDB(ctx).Where("key = ?", key).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// CreateTemplateRecord persists a new template.
func CreateTemplateRecord(ctx context.Context, tmpl *model.Template) error {
	return GetDB(ctx).Create(tmpl).Error
}

// SaveTemplateRecord updates an existing template.
func SaveTemplateRecord(ctx context.Context, tmpl *model.Template) error {
	return GetDB(ctx).Save(tmpl).Error
}

// DeleteTemplateRecord removes a template record.
func DeleteTemplateRecord(ctx context.Context, tmpl *model.Template) error {
	return GetDB(ctx).Delete(tmpl).Error
}
