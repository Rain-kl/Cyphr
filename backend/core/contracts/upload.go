// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package contracts

import (
	"context"
	"io"
	"time"
)

// UploadMetadataDTO represents upload metadata JSON.
type UploadMetadataDTO struct {
	Width        int            `json:"width,omitempty"`
	Height       int            `json:"height,omitempty"`
	Duration     float64        `json:"duration,omitempty"`
	OriginalMime string         `json:"original_mime,omitempty"`
	UserAgent    string         `json:"user_agent,omitempty"`
	ClientIP     string         `json:"client_ip,omitempty"`
	Bucket       string         `json:"bucket,omitempty"`
	Extra        map[string]any `json:"extra,omitempty"`
}

// UploadDTO represents an uploaded file record.
type UploadDTO struct {
	ID        uint64            `json:"id" gorm:"primaryKey"`
	UserID    uint64            `json:"user_id" gorm:"index"`
	FileName  string            `json:"file_name" gorm:"size:255"`
	FilePath  string            `json:"file_path" gorm:"size:500"`
	MimeType  string            `json:"mime_type" gorm:"size:100"`
	Size      int64             `json:"size"`
	Hash      string            `json:"hash" gorm:"size:64"`
	Status    string            `json:"status" gorm:"type:varchar(20)"`
	Type      string            `json:"type" gorm:"size:50;index"`
	Metadata  UploadMetadataDTO `json:"metadata" gorm:"serializer:json;type:jsonb"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// TableName returns the default table name for UploadDTO.
func (UploadDTO) TableName() string {
	return "w_uploads"
}

// OpenedUploadDTO encapsulates the retrieved object stream and its metadata.
type OpenedUploadDTO struct {
	Upload        UploadDTO
	Body          io.ReadCloser
	ContentType   string
	ContentLength int64
}

// UploadService defines the unified contract for managed file uploads and media entities.
type UploadService interface {
	GetByID(ctx context.Context, id uint64) (*UploadDTO, error)
	OpenStoredUpload(ctx context.Context, id uint64) (*OpenedUploadDTO, error)
	Remove(ctx context.Context, id uint64) error
	RemoveOwned(ctx context.Context, id uint64, userID uint64) error
	FindByHash(ctx context.Context, hash string, size int64) (*UploadDTO, error)
	RebuildStats(ctx context.Context) error
}
