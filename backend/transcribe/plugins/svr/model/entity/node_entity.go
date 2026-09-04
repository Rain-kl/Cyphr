// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package entity defines database entities for the transcribe platform.
package entity

import "time"

// NodeEntity represents an inference worker node registered in the system.
type NodeEntity struct {
	ID                 uint64     `json:"id,string" gorm:"primaryKey"`
	Name               string     `json:"name" gorm:"size:64;not null"`
	AgentToken         string     `json:"agent_token" gorm:"size:128;not null;default:''"`
	TokenHash          string     `json:"token_hash" gorm:"size:64;uniqueIndex;not null"`
	TokenPrefix        string     `json:"token_prefix" gorm:"size:16;not null"`
	IsActive           bool       `json:"is_active" gorm:"not null"`
	WorkMode           string     `json:"work_mode" gorm:"size:16;not null;default:'gpu'"`
	AllowAutoLoad      bool       `json:"allow_auto_load" gorm:"not null;default:true"`
	AutoUnloadMinutes  int        `json:"auto_unload_minutes" gorm:"not null;default:0"`
	MaxConcurrentJobs  int        `json:"max_concurrent_jobs" gorm:"not null;default:2"`
	ModelVramEstimates string     `json:"model_vram_estimates" gorm:"type:text;not null;default:'{}'"`
	LastIP             string     `json:"last_ip" gorm:"size:45"`
	LastSeenAt         *time.Time `json:"last_seen_at"`
	CreatedAt          time.Time  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt          time.Time  `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName returns the table name for NodeEntity.
func (NodeEntity) TableName() string {
	return "t_nodes"
}
