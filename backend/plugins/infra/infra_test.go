// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package infra_test

import (
	"Wavelet/core"
	"Wavelet/core/contracts"
	"Wavelet/plugins/infra/cache"
	"Wavelet/plugins/infra/database"
	"Wavelet/plugins/infra/logger"
	"Wavelet/plugins/infra/storage"
	"Wavelet/plugins/infra/storage/objectstore"
	"bytes"
	"context"
	"io"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type TestUser struct {
	ID   uint64 `gorm:"primaryKey"`
	Name string
}

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&TestUser{}))
	return db
}

func prepareTestContext(values map[string]any, declarers ...core.Plugin) *core.Context {
	ctx := core.NewContext(context.Background())
	ctx.Config().SetSource(core.NewMapSource(values))
	for _, p := range declarers {
		if d, ok := p.(interface{ DeclareConfig() []core.ConfigBinding }); ok {
			for _, b := range d.DeclareConfig() {
				_ = ctx.Config().Declare(p.Name(), b)
			}
		}
	}
	_ = ctx.Config().Resolve()
	return ctx
}

func TestDatabasePlugin(t *testing.T) {
	testDB := setupTestDB(t)
	p := database.New(database.WithDB(testDB))
	ctx := prepareTestContext(nil, p)

	require.Equal(t, "database", p.Name())
	require.NoError(t, p.Apply(ctx))

	dbSvc, err := core.Inject[contracts.DBService](ctx)
	require.NoError(t, err)
	require.NotNil(t, dbSvc)

	// Verify GORM and DB methods
	assert.NotNil(t, dbSvc.GORM())
	assert.NotNil(t, dbSvc.DB(context.Background()))

	// Test CRUD via service
	user := TestUser{ID: 1, Name: "Alice"}
	require.NoError(t, dbSvc.DB(context.Background()).Create(&user).Error)

	var fetched TestUser
	require.NoError(t, dbSvc.GORM().First(&fetched, 1).Error)
	assert.Equal(t, "Alice", fetched.Name)

	// Test NamedDB fallback
	assert.NotNil(t, dbSvc.Named("replica"))
}

func TestCachePluginRAMOnly(t *testing.T) {
	p := cache.New()
	ctx := prepareTestContext(nil, p)

	require.Equal(t, "cache", p.Name())
	require.NoError(t, p.Apply(ctx))

	cacheSvc, err := core.Inject[contracts.CacheService](ctx)
	require.NoError(t, err)
	require.NotNil(t, cacheSvc)

	type CacheItem struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}

	testCtx := context.Background()

	// 1. Get non-existing key
	var notFound CacheItem
	err = cacheSvc.Get(testCtx, "missing:key", &notFound)
	assert.ErrorIs(t, err, contracts.ErrCacheMiss)

	// 2. Set and Get
	item := CacheItem{Name: "item1", Count: 42}
	require.NoError(t, cacheSvc.Set(testCtx, "test:item1", item, time.Minute))

	var retrieved CacheItem
	require.NoError(t, cacheSvc.Get(testCtx, "test:item1", &retrieved))
	assert.Equal(t, item, retrieved)

	// 3. GetOrSet
	var getOrSetTarget CacheItem
	var loaderCalled bool
	err = cacheSvc.GetOrSet(testCtx, "test:item1", &getOrSetTarget, time.Minute, func() (any, error) {
		loaderCalled = true
		return CacheItem{Name: "never_called", Count: 0}, nil
	})
	require.NoError(t, err)
	assert.False(t, loaderCalled)
	assert.Equal(t, item, getOrSetTarget)

	// GetOrSet with cache miss
	var newItem CacheItem
	err = cacheSvc.GetOrSet(testCtx, "test:item2", &newItem, time.Minute, func() (any, error) {
		loaderCalled = true
		return CacheItem{Name: "loaded", Count: 99}, nil
	})
	require.NoError(t, err)
	assert.True(t, loaderCalled)
	assert.Equal(t, "loaded", newItem.Name)
	assert.Equal(t, 99, newItem.Count)

	// 4. Delete
	require.NoError(t, cacheSvc.Delete(testCtx, "test:item1"))
	err = cacheSvc.Get(testCtx, "test:item1", &retrieved)
	assert.ErrorIs(t, err, contracts.ErrCacheMiss)

	// 5. Invalidate alias
	require.NoError(t, cacheSvc.Invalidate(testCtx, "test:item2"))
	err = cacheSvc.Get(testCtx, "test:item2", &retrieved)
	assert.ErrorIs(t, err, contracts.ErrCacheMiss)
}

func TestCachePluginWithRedisAndPubSub(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	defer func() { _ = rdb.Close() }()

	p1 := cache.New(cache.WithRedis(rdb), cache.WithKeyPrefix("test:"))
	p2 := cache.New(cache.WithRedis(rdb), cache.WithKeyPrefix("test:"))

	ctx1 := prepareTestContext(map[string]any{"redis.enabled": true}, p1)
	ctx2 := prepareTestContext(map[string]any{"redis.enabled": true}, p2)

	require.NoError(t, p1.Apply(ctx1))
	require.NoError(t, p2.Apply(ctx2))

	cache1, err := core.Inject[contracts.CacheService](ctx1)
	require.NoError(t, err)
	cache2, err := core.Inject[contracts.CacheService](ctx2)
	require.NoError(t, err)

	testCtx := context.Background()

	// Node 1 writes to cache
	type UserCache struct {
		Name string `json:"name"`
	}
	require.NoError(t, cache1.Set(testCtx, "user:100", UserCache{Name: "Bob"}, 10*time.Minute))

	// Node 2 reads from cache (misses Node 2's RAM, hits Redis, backfills Node 2's RAM)
	var u2 UserCache
	require.NoError(t, cache2.Get(testCtx, "user:100", &u2))
	assert.Equal(t, "Bob", u2.Name)

	// Node 1 deletes cache (evicts Node 1 RAM, Redis, and broadcasts to Node 2)
	require.NoError(t, cache1.Delete(testCtx, "user:100"))

	// Verify Redis is deleted
	var uRedis UserCache
	err = cache1.Get(testCtx, "user:100", &uRedis)
	assert.ErrorIs(t, err, contracts.ErrCacheMiss)

	// Clean up contexts
	require.NoError(t, ctx1.Dispose())
	require.NoError(t, ctx2.Dispose())
}

func TestLoggerPlugin(t *testing.T) {
	ctx := core.NewContext(context.Background())
	p := logger.New()
	require.Equal(t, "logger", p.Name())
	require.NoError(t, p.Apply(ctx))

	logSvc, err := core.Inject[contracts.LoggerService](ctx)
	require.NoError(t, err)
	require.NotNil(t, logSvc)

	testCtx := context.Background()

	// Should not panic on any log call
	logSvc.Debug(testCtx, "debug message", "key1", "val1")
	logSvc.Info(testCtx, "info message", "userID", 123)
	logSvc.Warn(testCtx, "warn message", "warning", true)
	logSvc.Error(testCtx, "error message", "err", "something broke")

	logSvc.Debugf(testCtx, "formatted debug %d", 1)
	logSvc.Infof(testCtx, "formatted info %s", "test")
	logSvc.Warnf(testCtx, "formatted warn %v", map[string]int{"a": 1})
	logSvc.Errorf(testCtx, "formatted error %s", "fatal")

	childLog := logSvc.With("module", "test_module")
	require.NotNil(t, childLog)
	childLog.Info(testCtx, "child log message", "action", "run")
}

type memoryBackend struct {
	mu      sync.RWMutex
	storage map[string][]byte
}

func newMemoryBackend() *memoryBackend {
	return &memoryBackend{
		storage: make(map[string][]byte),
	}
}

func (m *memoryBackend) Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) (objectstore.PutResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, err := io.ReadAll(body)
	if err != nil {
		return objectstore.PutResult{}, err
	}
	m.storage[key] = data
	return objectstore.PutResult{Key: key, Bucket: "test-bucket"}, nil
}

func (m *memoryBackend) Get(ctx context.Context, key string) (*objectstore.Object, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	data, ok := m.storage[key]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return &objectstore.Object{
		Body:          io.NopCloser(bytes.NewReader(data)),
		ContentLength: int64(len(data)),
		ContentType:   "application/octet-stream",
	}, nil
}

func (m *memoryBackend) Delete(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.storage, key)
	return nil
}

func (m *memoryBackend) Test(ctx context.Context) error {
	return nil
}

func TestStoragePlugin(t *testing.T) {
	ctx := core.NewContext(context.Background())
	backend := newMemoryBackend()

	p := storage.New(storage.WithBackend(backend))
	require.Equal(t, "storage", p.Name())
	require.NoError(t, p.Apply(ctx))

	storageSvc, err := core.Inject[contracts.StorageService](ctx)
	require.NoError(t, err)
	require.NotNil(t, storageSvc)

	testCtx := context.Background()

	// 1. Put
	content := []byte("Hello, Wavelet Storage Plugin!")
	putRes, err := storageSvc.Put(testCtx, "uploads/hello.txt", bytes.NewReader(content), int64(len(content)), "text/plain")
	require.NoError(t, err)
	assert.Equal(t, "uploads/hello.txt", putRes.Key)
	assert.Equal(t, "test-bucket", putRes.Bucket)

	// 2. Get
	obj, err := storageSvc.Get(testCtx, "uploads/hello.txt")
	require.NoError(t, err)
	require.NotNil(t, obj)
	data, err := io.ReadAll(obj.Body)
	require.NoError(t, err)
	assert.Equal(t, content, data)
	assert.Equal(t, int64(len(content)), obj.ContentLength)

	// 3. Delete
	require.NoError(t, storageSvc.Delete(testCtx, "uploads/hello.txt"))
	_, err = storageSvc.Get(testCtx, "uploads/hello.txt")
	assert.Error(t, err)
}

func TestAllInfraPluginsCombined(t *testing.T) {
	testDB := setupTestDB(t)
	memBackend := newMemoryBackend()

	dbP := database.New(database.WithDB(testDB))
	cacheP := cache.New()
	logP := logger.New()
	storageP := storage.New(storage.WithBackend(memBackend))

	ctx := prepareTestContext(nil, dbP, cacheP, logP, storageP)

	require.NoError(t, dbP.Apply(ctx))
	require.NoError(t, cacheP.Apply(ctx))
	require.NoError(t, logP.Apply(ctx))
	require.NoError(t, storageP.Apply(ctx))

	// Using3 to resolve dependencies concurrently
	var resolved bool
	err := core.Using3(ctx, func(db contracts.DBService, c contracts.CacheService, l contracts.LoggerService) {
		resolved = true
		assert.NotNil(t, db)
		assert.NotNil(t, c)
		assert.NotNil(t, l)
	})
	require.NoError(t, err)
	assert.True(t, resolved)

	// Using storage
	err = core.Using(ctx, func(s contracts.StorageService) {
		assert.NotNil(t, s)
	})
	require.NoError(t, err)
}
