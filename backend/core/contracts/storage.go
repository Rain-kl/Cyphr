// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package contracts defines unified service interfaces and DTOs for cross-plugin communication.
package contracts

import (
	"context"
	"io"
)

// StorageObject represents a retrieved file object from the storage backend.
type StorageObject struct {
	Key           string
	CachePath     string
	Body          io.ReadCloser
	ContentLength int64
	ContentType   string
}

// StoragePutResult describes the output of a successful Put operation.
type StoragePutResult struct {
	Key    string
	Bucket string
}

// IngestOptions configures programmatic ingest of files into the platform storage.
type IngestOptions struct {
	UserID    uint64
	Type      string
	FileName  string
	MimeType  string
	Extension string
	Size      int64
	Policy    int
	Metadata  map[string]any
}

// IngestResult reports the outcome of a programmatic file ingest operation.
type IngestResult struct {
	ID       uint64
	Key      string
	URL      string
	Created  bool
	Stored   bool
	Resolved bool
}

// StorageDriver identifies a supported storage backend.
type StorageDriver string

const (
	StorageDriverLocal  StorageDriver = "local"
	StorageDriverS3     StorageDriver = "s3"
	StorageDriverR2     StorageDriver = "r2"
	StorageDriverMinIO  StorageDriver = "minio"
	StorageDriverOSS    StorageDriver = "oss"
	StorageDriverWebDAV StorageDriver = "webdav"
)

// LocalStorageConfigDTO configures local filesystem storage.
type LocalStorageConfigDTO struct {
	Root string `json:"root"`
}

// ObjectStorageConfigDTO configures S3-compatible or OSS object storage.
type ObjectStorageConfigDTO struct {
	Endpoint        string `json:"endpoint"`
	Region          string `json:"region"`
	Bucket          string `json:"bucket"`
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
	AccountID       string `json:"account_id,omitempty"`
	PathStyle       bool   `json:"path_style"`
	KeyPrefix       string `json:"key_prefix"`
	CDNURL          string `json:"cdn_url"`
}

// WebDAVStorageConfigDTO configures WebDAV storage.
type WebDAVStorageConfigDTO struct {
	URL      string `json:"url"`
	Username string `json:"username"`
	Password string `json:"password"`
	Root     string `json:"root"`
}

// StorageConfigDTO encapsulates full storage configuration across all backends.
type StorageConfigDTO struct {
	Driver StorageDriver          `json:"driver"`
	Local  LocalStorageConfigDTO  `json:"local"`
	S3     ObjectStorageConfigDTO `json:"s3"`
	R2     ObjectStorageConfigDTO `json:"r2"`
	MinIO  ObjectStorageConfigDTO `json:"minio"`
	OSS    ObjectStorageConfigDTO `json:"oss"`
	WebDAV WebDAVStorageConfigDTO `json:"webdav"`
}

// StorageService defines the contract for unified object storage and managed file ingestion.
type StorageService interface {
	// Put writes an object to storage.
	Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) (StoragePutResult, error)

	// Get retrieves an object from storage.
	Get(ctx context.Context, key string) (*StorageObject, error)

	// Delete removes an object from storage.
	Delete(ctx context.Context, key string) error

	// Ingest performs managed file ingestion into the platform storage domain with deduplication and metadata tracking.
	Ingest(ctx context.Context, reader io.Reader, opts IngestOptions) (*IngestResult, error)
}
