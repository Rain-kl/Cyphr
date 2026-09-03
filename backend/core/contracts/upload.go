// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package contracts

import (
	"context"
	"io"
	"time"
)

// UploadDTO represents an uploaded file record.
type UploadDTO struct {
	ID        uint64    `json:"id"`
	UserID    uint64    `json:"user_id"`
	FileName  string    `json:"file_name"`
	FilePath  string    `json:"file_path"`
	MimeType  string    `json:"mime_type"`
	Size      int64     `json:"size"`
	Hash      string    `json:"hash"`
	Status    string    `json:"status"`
	Type      string    `json:"type"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
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
