// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package storage_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Rain-kl/Wavelet/backend/core"
	"github.com/Rain-kl/Wavelet/backend/core/contracts"
	"github.com/Rain-kl/Wavelet/backend/plugins/infra/storage"
	"github.com/Rain-kl/Wavelet/backend/plugins/infra/storage/objectstore"
)

type mockStorageBackend struct {
	mu    sync.RWMutex
	files map[string][]byte
}

func newMockStorageBackend() *mockStorageBackend {
	return &mockStorageBackend{files: make(map[string][]byte)}
}

func (m *mockStorageBackend) Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) (objectstore.PutResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, err := io.ReadAll(body)
	if err != nil {
		return objectstore.PutResult{}, err
	}
	m.files[key] = data
	return objectstore.PutResult{Key: key, Bucket: "mock-bucket"}, nil
}

func (m *mockStorageBackend) Get(ctx context.Context, key string) (*objectstore.Object, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	data, ok := m.files[key]
	if !ok {
		return nil, errors.New("file not found")
	}
	return &objectstore.Object{
		Body:          io.NopCloser(bytes.NewReader(data)),
		ContentLength: int64(len(data)),
		ContentType:   "application/octet-stream",
	}, nil
}

func (m *mockStorageBackend) Delete(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.files, key)
	return nil
}

func (m *mockStorageBackend) Test(ctx context.Context) error {
	return nil
}

func TestStoragePluginOperations(t *testing.T) {
	backend := newMockStorageBackend()
	p := storage.New(storage.WithBackend(backend))
	assert.Equal(t, "storage", p.Name())

	ctx := core.NewContext(context.Background())
	require.NoError(t, p.Apply(ctx))

	svc, err := core.Inject[contracts.StorageService](ctx)
	require.NoError(t, err)
	require.NotNil(t, svc)

	testCtx := context.Background()

	// Put
	content := []byte("wavelet storage content")
	putRes, err := svc.Put(testCtx, "avatar/user1.png", bytes.NewReader(content), int64(len(content)), "image/png")
	require.NoError(t, err)
	assert.Equal(t, "avatar/user1.png", putRes.Key)
	assert.Equal(t, "mock-bucket", putRes.Bucket)

	// Get
	obj, err := svc.Get(testCtx, "avatar/user1.png")
	require.NoError(t, err)
	data, err := io.ReadAll(obj.Body)
	require.NoError(t, err)
	assert.Equal(t, content, data)

	// Delete
	require.NoError(t, svc.Delete(testCtx, "avatar/user1.png"))
	_, err = svc.Get(testCtx, "avatar/user1.png")
	assert.Error(t, err)
}
