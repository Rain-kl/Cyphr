// Package storage provides the object storage and ingestion infrastructure plugin for Cordis.
package storage

import (
	"context"
	"fmt"
	"io"

	"github.com/Rain-kl/Wavelet/core"
	"github.com/Rain-kl/Wavelet/core/contracts"
	"github.com/Rain-kl/Wavelet/internal/infra/objectstore"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/plugins/domain/upload/ingest"
)

// Option configures the storage plugin.
type Option func(*Plugin)

// WithBackend sets an explicit storage backend instance (useful for testing or custom engines).
func WithBackend(b objectstore.Backend) Option {
	return func(p *Plugin) {
		p.backend = b
	}
}

// Plugin implements core.Plugin to provide contracts.StorageService.
type Plugin struct {
	backend objectstore.Backend
}

// New creates a new storage infrastructure plugin.
func New(opts ...Option) *Plugin {
	p := &Plugin{}
	for _, opt := range opts {
		if opt != nil {
			opt(p)
		}
	}
	return p
}

// Name returns the unique identifier of the storage plugin.
func (p *Plugin) Name() string {
	return "storage"
}

// Apply mounts the storage service into the Context.
func (p *Plugin) Apply(ctx *core.Context) error {
	svc := &storageServiceImpl{
		backend: p.backend,
	}
	core.Provide[contracts.StorageService](ctx, svc)
	return nil
}

type storageServiceImpl struct {
	backend objectstore.Backend
}

func (s *storageServiceImpl) getBackend(ctx context.Context) (objectstore.Backend, error) {
	if s.backend != nil {
		return s.backend, nil
	}
	_, b, err := objectstore.Active(ctx)
	if err != nil {
		return nil, fmt.Errorf("storage: get active backend failed: %w", err)
	}
	return b, nil
}

func (s *storageServiceImpl) Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) (contracts.StoragePutResult, error) {
	b, err := s.getBackend(ctx)
	if err != nil {
		return contracts.StoragePutResult{}, err
	}

	res, err := b.Put(ctx, key, body, size, contentType)
	if err != nil {
		return contracts.StoragePutResult{}, err
	}

	return contracts.StoragePutResult{
		Key:    res.Key,
		Bucket: res.Bucket,
	}, nil
}

func (s *storageServiceImpl) Get(ctx context.Context, key string) (*contracts.StorageObject, error) {
	b, err := s.getBackend(ctx)
	if err != nil {
		return nil, err
	}

	obj, err := b.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	return &contracts.StorageObject{
		Key:           key,
		CachePath:     obj.CachePath,
		Body:          obj.Body,
		ContentLength: obj.ContentLength,
		ContentType:   obj.ContentType,
	}, nil
}

func (s *storageServiceImpl) Delete(ctx context.Context, key string) error {
	b, err := s.getBackend(ctx)
	if err != nil {
		return err
	}
	return b.Delete(ctx, key)
}

func (s *storageServiceImpl) Ingest(ctx context.Context, reader io.Reader, opts contracts.IngestOptions) (*contracts.IngestResult, error) {
	meta := model.UploadMetadata{
		Extra: opts.Metadata,
	}

	req := ingest.Request{
		UserID:    opts.UserID,
		Type:      opts.Type,
		FileName:  opts.FileName,
		MimeType:  opts.MimeType,
		Extension: opts.Extension,
		Size:      opts.Size,
		Reader:    reader,
		Policy:    ingest.Policy(opts.Policy),
		Metadata:  meta,
	}

	res, err := ingest.Ingest(ctx, req)
	if err != nil {
		return nil, err
	}

	return &contracts.IngestResult{
		ID:       res.Upload.ID,
		Key:      res.Upload.FilePath,
		Created:  res.Created,
		Stored:   res.Stored,
		Resolved: res.Resolved,
	}, nil
}
