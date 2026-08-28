// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package models 提供上传域核心数据模型。
package models

import (
	"time"
)

// UploadStatus 上传状态
type UploadStatus string

// 上传状态
const (
	UploadStatusPending UploadStatus = "pending" // 待使用
	UploadStatusUsed    UploadStatus = "used"    // 已使用
	UploadStatusDeleted UploadStatus = "deleted" // 已删除
)

// UploadMetadata 自定义可扩展的 JSON 字段存储非核心或可选的文件元数据
type UploadMetadata struct {
	Width        int            `json:"width,omitempty"`
	Height       int            `json:"height,omitempty"`
	Duration     float64        `json:"duration,omitempty"`
	OriginalMime string         `json:"original_mime,omitempty"`
	UserAgent    string         `json:"user_agent,omitempty"`
	ClientIP     string         `json:"client_ip,omitempty"`
	Bucket       string         `json:"bucket,omitempty"`
	Extra        map[string]any `json:"extra,omitempty"`
}

// Upload 上传文件记录
type Upload struct {
	ID         uint64         `json:"id,string" gorm:"primaryKey"`
	UserID     uint64         `json:"user_id,string" gorm:"index;not null"`
	FileName   string         `json:"file_name" gorm:"size:255;not null"`
	FilePath   string         `json:"file_path" gorm:"size:500;not null;index"`
	FileSize   int64          `json:"file_size" gorm:"not null"`
	MimeType   string         `json:"mime_type" gorm:"size:100;not null"`
	Extension  string         `json:"extension" gorm:"size:50;not null"`
	Hash       string         `json:"hash" gorm:"size:64;index"`
	Type       string         `json:"type" gorm:"column:type;size:50;not null;index"`
	Status     UploadStatus   `json:"status" gorm:"type:varchar(20);not null"`
	AccessMode int            `json:"access_mode" gorm:"column:access_mode;not null;default:0"`
	Metadata   UploadMetadata `json:"metadata" gorm:"serializer:json;type:jsonb"`
	CreatedAt  time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt  time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName 表名
func (Upload) TableName() string {
	return "w_uploads"
}

// Upload stats dimension keys stored in w_upload_stats.dimension.
const (
	UploadStatDimensionTotal    = "total"
	UploadStatDimensionType     = "type"
	UploadStatDimensionCategory = "category"
	UploadStatDimensionTrend    = "trend"
)

// UploadStat 聚合统计记录
type UploadStat struct {
	Dimension string    `json:"dimension" gorm:"primaryKey;size:32;not null"`
	StatKey   string    `json:"stat_key" gorm:"primaryKey;size:64;not null;default:''"`
	FileCount int64     `json:"file_count" gorm:"not null;default:0"`
	FileSize  int64     `json:"file_size" gorm:"not null;default:0"`
	UpdatedAt time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// TableName 表名
func (UploadStat) TableName() string {
	return "w_upload_stats"
}
