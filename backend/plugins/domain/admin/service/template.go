// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"Wavelet/plugins/domain/admin/errs"
	"Wavelet/plugins/domain/admin/model"
	"Wavelet/plugins/domain/admin/repository"
	"context"
)

// CreateTemplate persists a new notification template after key collision and field checks.
func CreateTemplate(ctx context.Context, req model.CreateTemplateRequest) (model.Template, error) {
	exists, err := repository.TemplateExistsByKey(ctx, req.Key)
	if err != nil {
		return model.Template{}, err
	}
	if exists {
		return model.Template{}, errs.ErrTemplateKeyExists
	}

	tmpl := model.Template{
		Key:         req.Key,
		Name:        req.Name,
		Type:        req.Type,
		Subject:     req.Subject,
		Content:     req.Content,
		Description: req.Description,
		IsSystem:    false,
	}
	if err := tmpl.Validate(); err != nil {
		return model.Template{}, err
	}
	if err := repository.CreateTemplateRecord(ctx, &tmpl); err != nil {
		return model.Template{}, err
	}
	return tmpl, nil
}

// ListTemplates returns every notification template.
func ListTemplates(ctx context.Context) ([]model.Template, error) {
	return repository.ListTemplatesRecord(ctx)
}

// GetTemplate loads a template by its identifier.
func GetTemplate(ctx context.Context, key string) (model.Template, error) {
	tmpl, err := repository.GetTemplateByKey(ctx, key)
	if err != nil {
		return model.Template{}, translateNotFound(err, errs.ErrTemplateNotFound)
	}
	return tmpl, nil
}

// UpdateTemplate rewrites the mutable fields of an existing template.
func UpdateTemplate(ctx context.Context, key string, req model.UpdateTemplateRequest) (model.Template, error) {
	tmpl, err := GetTemplate(ctx, key)
	if err != nil {
		return model.Template{}, err
	}

	tmpl.Name = req.Name
	tmpl.Type = req.Type
	tmpl.Subject = req.Subject
	tmpl.Content = req.Content
	tmpl.Description = req.Description
	if err := tmpl.Validate(); err != nil {
		return model.Template{}, err
	}
	if err := repository.SaveTemplateRecord(ctx, &tmpl); err != nil {
		return model.Template{}, err
	}
	return tmpl, nil
}

// DeleteTemplate removes a custom template; system presets are protected.
func DeleteTemplate(ctx context.Context, key string) error {
	tmpl, err := GetTemplate(ctx, key)
	if err != nil {
		return err
	}
	if tmpl.IsSystem {
		return errs.ErrSystemTemplateCannotDelete
	}
	return repository.DeleteTemplateRecord(ctx, &tmpl)
}
