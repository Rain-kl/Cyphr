// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"context"
	"errors"

	"Wavelet/core/contracts"
	"Wavelet/pkg/logger"
	"Wavelet/plugins/domain/upload/models"
	"Wavelet/plugins/domain/upload/shared"
)

// ReadOnly checks if the storage system is in read-only maintenance mode.
func ReadOnly(ctx context.Context) bool {
	state := LoadMigrationAccessState(ctx)
	if state.LoadErr != nil {
		logger.ErrorF(ctx, "读取存储维护状态失败: %v", state.LoadErr)
		return true
	}
	return state.ReadOnly
}

// OpenStoredObject opens a stored upload object from the active storage backend.
func OpenStoredObject(ctx context.Context, upload *models.Upload) (*contracts.StorageObject, error) {
	storageSvc := shared.GetStorage(ctx)
	if storageSvc == nil {
		return nil, errors.New("storage service not available")
	}
	return storageSvc.Get(ctx, upload.FilePath)
}
