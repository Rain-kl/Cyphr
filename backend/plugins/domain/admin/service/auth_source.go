// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"Wavelet/core/contracts"
	"Wavelet/pkg/logger"
	"Wavelet/plugins/domain/admin/errs"
	"context"
	"errors"
	"fmt"
)

// ListAuthSources returns every configured authentication source.
func ListAuthSources(ctx context.Context) ([]contracts.AuthSourceViewDTO, error) {
	authSvc, err := requireAuthService(ctx)
	if err != nil {
		return nil, err
	}

	views, err := authSvc.ListAuthSources(ctx)
	if err != nil {
		logger.ErrorF(ctx, "List auth sources failed: %v", err)
		return nil, errors.New(errs.ListAuthSourcesFailed)
	}
	return views, nil
}

// CreateAuthSource registers a new authentication source.
func CreateAuthSource(ctx context.Context, source contracts.AuthSourceDTO) (*contracts.AuthSourceDTO, error) {
	authSvc, err := requireAuthService(ctx)
	if err != nil {
		return nil, err
	}

	created, err := authSvc.CreateAuthSource(ctx, source)
	if err != nil {
		return nil, fmt.Errorf("%s%w", errs.CreateAuthSourceFailed, err)
	}
	return created, nil
}

// UpdateAuthSource rewrites an existing authentication source.
func UpdateAuthSource(
	ctx context.Context,
	id uint64,
	source contracts.AuthSourceDTO,
) (*contracts.AuthSourceDTO, error) {
	authSvc, err := requireAuthService(ctx)
	if err != nil {
		return nil, err
	}

	updated, err := authSvc.UpdateAuthSource(ctx, id, source)
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// ToggleAuthSource flips the active state of an authentication source.
func ToggleAuthSource(ctx context.Context, id uint64) (*contracts.AuthSourceDTO, error) {
	authSvc, err := requireAuthService(ctx)
	if err != nil {
		return nil, err
	}

	toggled, err := authSvc.ToggleAuthSource(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("%s%w", errs.ToggleAuthSourceFailed, err)
	}
	return toggled, nil
}

// DeleteAuthSource removes an authentication source.
func DeleteAuthSource(ctx context.Context, id uint64) error {
	authSvc, err := requireAuthService(ctx)
	if err != nil {
		return err
	}

	if err := authSvc.DeleteAuthSource(ctx, id); err != nil {
		return fmt.Errorf("%s%w", errs.DeleteAuthSourceFailed, err)
	}
	return nil
}
