// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"Wavelet/core/contracts"
	"Wavelet/plugins/domain/upload/shared"
)

// StorageMigrationTask is the task name for storage migration.
const StorageMigrationTask = "storage:migrate"

// LatestMigrationExecution returns the most recent storage migration task execution.
func LatestMigrationExecution(ctx context.Context) (*contracts.TaskExecutionDTO, bool, error) {
	db := shared.GetDB(ctx)
	if db == nil {
		return nil, false, nil
	}
	var exec contracts.TaskExecutionDTO
	err := db.Table("w_task_executions").Where("task_type = ?", StorageMigrationTask).Order("id DESC").First(&exec).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return &exec, true, nil
}

// ParseMigrationTargetConfig parses and validates a storage migration target payload.
func ParseMigrationTargetConfig(ctx context.Context, payload []byte) (contracts.StorageConfigDTO, error) {
	if strings.TrimSpace(string(payload)) == "" {
		return contracts.StorageConfigDTO{}, errors.New("storage migration target payload is required")
	}

	var raw struct {
		Target json.RawMessage `json:"target"`
	}
	if err := json.Unmarshal(payload, &raw); err != nil {
		return contracts.StorageConfigDTO{}, fmt.Errorf("parse storage migration payload envelope: %w", err)
	}

	if len(raw.Target) == 0 {
		return contracts.StorageConfigDTO{}, errors.New("storage migration target payload is required")
	}

	var targetBytes []byte
	var targetStr string
	if err := json.Unmarshal(raw.Target, &targetStr); err == nil {
		targetBytes = []byte(targetStr)
	} else {
		targetBytes = raw.Target
	}

	var target contracts.StorageConfigDTO
	if err := json.Unmarshal(targetBytes, &target); err != nil {
		return contracts.StorageConfigDTO{}, fmt.Errorf("parse target storage config: %w", err)
	}

	return target, nil
}

// NormalizeMigrationPayload validates and normalizes a storage migration payload.
func NormalizeMigrationPayload(ctx context.Context, payload []byte) ([]byte, contracts.StorageConfigDTO, error) {
	target, err := ParseMigrationTargetConfig(ctx, payload)
	if err != nil {
		return nil, contracts.StorageConfigDTO{}, err
	}
	raw, err := json.Marshal(struct {
		Target contracts.StorageConfigDTO `json:"target"`
	}{Target: target})
	if err != nil {
		return nil, contracts.StorageConfigDTO{}, fmt.Errorf("serialize normalized payload: %w", err)
	}
	return raw, target, nil
}

// SaveActiveConfig persists the active storage configuration to w_system_configs.
func SaveActiveConfig(ctx context.Context, cfg contracts.StorageConfigDTO) error {
	db := shared.GetDB(ctx)
	if db == nil {
		return errors.New("database not available")
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	return db.Table("w_system_configs").Where("key = ?", "storage_config").Update("value", string(data)).Error
}

// LoadStorageConfig loads the current storage configuration from w_system_configs.
func LoadStorageConfig(ctx context.Context) (contracts.StorageConfigDTO, error) {
	db := shared.GetDB(ctx)
	if db == nil {
		return contracts.StorageConfigDTO{}, errors.New("database not available")
	}
	var row struct {
		Value string
	}
	if err := db.Table("w_system_configs").Where("key = ?", "storage_config").First(&row).Error; err != nil {
		return contracts.StorageConfigDTO{}, err
	}
	var cfg contracts.StorageConfigDTO
	if err := json.Unmarshal([]byte(row.Value), &cfg); err != nil {
		return contracts.StorageConfigDTO{}, err
	}
	return cfg, nil
}
