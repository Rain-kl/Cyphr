// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package entity defines database entities for the transcribe platform.
package entity

import "time"

// JobLogEntity represents an execution log entry for a transcription job.
type JobLogEntity struct {
	ID        uint64    `json:"id,string" gorm:"primaryKey"`
	JobID     uint64    `json:"job_id,string" gorm:"not null;index:idx_t_job_logs_job_id_seq,priority:1"`
	Seq       int       `json:"seq" gorm:"not null;index:idx_t_job_logs_job_id_seq,priority:2"`
	Progress  int       `json:"progress" gorm:"not null;default:0"`
	Message   string    `json:"message" gorm:"type:text;not null"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
}

// TableName returns the table name for JobLogEntity.
func (JobLogEntity) TableName() string {
	return "t_job_logs"
}
