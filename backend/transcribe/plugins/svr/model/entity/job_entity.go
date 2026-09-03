// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package entity defines database entities for the transcribe platform.
package entity

import "time"

// JobEntity represents a transcription job.
type JobEntity struct {
	ID               uint64     `json:"id" gorm:"primaryKey"`
	UserID           uint64     `json:"user_id" gorm:"not null;index:idx_t_jobs_user_status,priority:1"`
	NodeID           *uint64    `json:"node_id"`
	ModelName        string     `json:"model_name" gorm:"size:64;not null"`
	TaskType         string     `json:"task_type" gorm:"size:32;not null;default:'asr'"`
	Status           string     `json:"status" gorm:"size:32;not null;default:'pending';index:idx_t_jobs_user_status,priority:2"`
	Progress         int        `json:"progress" gorm:"not null;default:0"`
	AudioStoragePath string     `json:"audio_storage_path" gorm:"type:text;not null"`
	OriginalFileName string     `json:"original_file_name" gorm:"size:255;not null"`
	DurationSeconds  float64    `json:"duration_seconds" gorm:"not null;default:0"`
	ResultText       string     `json:"result_text" gorm:"type:text"`
	ResultJSON       string     `json:"result_json" gorm:"type:text"`
	ErrorMsg         string     `json:"error_msg" gorm:"type:text"`
	CreatedAt        time.Time  `json:"created_at" gorm:"autoCreateTime"`
	StartedAt        *time.Time `json:"started_at"`
	CompletedAt      *time.Time `json:"completed_at"`
	UpdatedAt        time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName returns the table name for JobEntity.
func (JobEntity) TableName() string {
	return "t_jobs"
}
