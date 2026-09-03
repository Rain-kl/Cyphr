// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package entity defines database entities for the transcribe platform.
package entity

import "time"

// ModelEntity represents a transcription model registered in the system.
type ModelEntity struct {
	ID          uint64    `json:"id,string" gorm:"primaryKey"`
	Name        string    `json:"name" gorm:"size:64;uniqueIndex;not null"`
	TaskType    string    `json:"task_type" gorm:"size:32;not null;default:'asr'"`
	Description string    `json:"description" gorm:"type:text"`
	IsActive    bool      `json:"is_active" gorm:"not null"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName returns the table name for ModelEntity.
func (ModelEntity) TableName() string {
	return "t_models"
}
