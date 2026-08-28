// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package shared

import (
	"Wavelet/core/contracts"
	"Wavelet/pkg/testhelper"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// MockDBService is a mock implementation of contracts.DBService for unit testing.
type MockDBService struct {
	DBInstance *gorm.DB
}

// GORM returns the underlying GORM instance.
func (m *MockDBService) GORM() *gorm.DB {
	return m.DBInstance
}

// DB returns the GORM instance bound to context.
func (m *MockDBService) DB(ctx context.Context) *gorm.DB {
	return m.DBInstance.WithContext(ctx)
}

// Named returns the named GORM instance.
func (m *MockDBService) Named(_ string) *gorm.DB {
	return m.DBInstance
}

// MockCacheService is an in-memory mock implementation of contracts.CacheService for unit testing.
type MockCacheService struct {
	mu   sync.RWMutex
	data map[string][]byte
}

// NewMockCacheService creates a new MockCacheService.
func NewMockCacheService() *MockCacheService {
	return &MockCacheService{
		data: make(map[string][]byte),
	}
}

// Get retrieves a cached value.
func (m *MockCacheService) Get(_ context.Context, key string, val any) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	b, ok := m.data[key]
	if !ok {
		return contracts.ErrCacheMiss
	}
	return json.Unmarshal(b, val)
}

// Set stores a key-value pair in cache.
func (m *MockCacheService) Set(_ context.Context, key string, val any, _ time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, err := json.Marshal(val)
	if err != nil {
		return err
	}
	m.data[key] = b
	return nil
}

// Delete removes a key from cache.
func (m *MockCacheService) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, key)
	return nil
}

// GetOrSet retrieves or populates a cache entry.
func (m *MockCacheService) GetOrSet(ctx context.Context, key string, target any, ttl time.Duration, loader func() (any, error)) error {
	err := m.Get(ctx, key, target)
	if err == nil {
		return nil
	}
	val, err := loader()
	if err != nil {
		return err
	}
	return m.Set(ctx, key, val, ttl)
}

// Invalidate invalidates a cache tag or prefix.
func (m *MockCacheService) Invalidate(_ context.Context, _ string) error {
	return nil
}

// MockStorageService is an in-memory mock implementation of contracts.StorageService for unit testing.
type MockStorageService struct {
	mu      sync.RWMutex
	objects map[string][]byte
}

// NewMockStorageService creates a new MockStorageService.
func NewMockStorageService() *MockStorageService {
	return &MockStorageService{
		objects: make(map[string][]byte),
	}
}

// Put uploads an object into mock storage.
func (m *MockStorageService) Put(_ context.Context, key string, body io.Reader, size int64, contentType string) (contracts.StoragePutResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, err := io.ReadAll(body)
	if err != nil {
		return contracts.StoragePutResult{}, err
	}
	m.objects[key] = data
	if strings.HasPrefix(key, "uploads/") {
		_ = os.MkdirAll(filepath.Dir(key), 0o755)
		_ = os.WriteFile(key, data, 0o644)
	}
	return contracts.StoragePutResult{Key: key, Bucket: "test-bucket"}, nil
}

// Get retrieves an object from mock storage.
func (m *MockStorageService) Get(_ context.Context, key string) (*contracts.StorageObject, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	data, ok := m.objects[key]
	if ok {
		return &contracts.StorageObject{
			Key:           key,
			Body:          io.NopCloser(bytes.NewReader(data)),
			ContentLength: int64(len(data)),
			ContentType:   "application/octet-stream",
		}, nil
	}
	if f, err := os.Open(key); err == nil {
		info, _ := f.Stat()
		return &contracts.StorageObject{
			Key:           key,
			Body:          f,
			ContentLength: info.Size(),
			ContentType:   "application/octet-stream",
		}, nil
	}
	return nil, gorm.ErrRecordNotFound
}

// Delete removes an object from mock storage.
func (m *MockStorageService) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.objects, key)
	return nil
}

// Ingest handles programmatic file ingestion for mock storage.
func (m *MockStorageService) Ingest(_ context.Context, _ io.Reader, _ contracts.IngestOptions) (*contracts.IngestResult, error) {
	return &contracts.IngestResult{ID: 1, Key: "test.png", Created: true, Stored: true}, nil
}

// MockAuthService is a mock implementation of contracts.AuthService for unit testing.
type MockAuthService struct {
	DB *gorm.DB
}

// RequireAuthMiddleware returns a dummy auth middleware.
func (a *MockAuthService) RequireAuthMiddleware() any {
	return func(c *gin.Context) { c.Next() }
}

// RequireAdminMiddleware returns a dummy admin middleware.
func (a *MockAuthService) RequireAdminMiddleware() any {
	return func(c *gin.Context) { c.Next() }
}

// DisallowTokenAuthMiddleware returns a dummy disallow token middleware.
func (a *MockAuthService) DisallowTokenAuthMiddleware() any {
	return func(c *gin.Context) { c.Next() }
}

// GetCurrentUser returns the user associated with the request context.
func (a *MockAuthService) GetCurrentUser(ctx context.Context) (*contracts.UserDTO, error) {
	if c, ok := ctx.(*gin.Context); ok {
		authHeader := c.GetHeader("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
			tokenHash := fmt.Sprintf("%x", sha256.Sum256([]byte(tokenStr)))
			var tokenRecord struct {
				UserID uint64
			}
			if err := a.DB.Table("w_access_tokens").Where("token_hash = ?", tokenHash).First(&tokenRecord).Error; err == nil {
				return &contracts.UserDTO{ID: tokenRecord.UserID, IsActive: true}, nil
			}
		}
	}
	return nil, errors.New("unauthorized")
}

// GetCurrentUserID returns the current user ID.
func (a *MockAuthService) GetCurrentUserID(ctx context.Context) (uint64, error) {
	u, err := a.GetCurrentUser(ctx)
	if err != nil {
		return 0, err
	}
	return u.ID, nil
}

// VerifyToken verifies an access token.
func (a *MockAuthService) VerifyToken(_ context.Context, token string) (*contracts.UserDTO, error) {
	tokenHash := fmt.Sprintf("%x", sha256.Sum256([]byte(token)))
	var tokenRecord struct {
		UserID uint64
	}
	if err := a.DB.Table("w_access_tokens").Where("token_hash = ?", tokenHash).First(&tokenRecord).Error; err == nil {
		return &contracts.UserDTO{ID: tokenRecord.UserID, IsActive: true}, nil
	}
	return nil, errors.New("unauthorized")
}

// Authenticate verifies credentials.
func (a *MockAuthService) Authenticate(_ context.Context, _, _ string) (*contracts.UserDTO, error) {
	return nil, nil
}

// CreateSession creates a login session.
func (a *MockAuthService) CreateSession(_ context.Context, _ uint64, _ map[string]any) (string, error) {
	return "test-session", nil
}

// RevokeToken revokes an access token.
func (a *MockAuthService) RevokeToken(_ context.Context, _ string) error {
	return nil
}

// RevokeUserSessions revokes all sessions for a user.
func (a *MockAuthService) RevokeUserSessions(_ context.Context, _ uint64) error {
	return nil
}

// InvalidateCachedUser invalidates cached user profile.
func (a *MockAuthService) InvalidateCachedUser(_ context.Context, _ uint64) {}

// InvalidateCachedToken invalidates cached access token.
func (a *MockAuthService) InvalidateCachedToken(_ context.Context, _ string) {}

// ListAuthSources lists configured authentication sources.
func (a *MockAuthService) ListAuthSources(_ context.Context) ([]contracts.AuthSourceViewDTO, error) {
	return nil, nil
}

// CreateAuthSource creates an authentication source.
func (a *MockAuthService) CreateAuthSource(_ context.Context, _ contracts.AuthSourceDTO) (*contracts.AuthSourceDTO, error) {
	return nil, nil
}

// UpdateAuthSource updates an authentication source.
func (a *MockAuthService) UpdateAuthSource(_ context.Context, _ uint64, _ contracts.AuthSourceDTO) (*contracts.AuthSourceDTO, error) {
	return nil, nil
}

// DeleteAuthSource deletes an authentication source.
func (a *MockAuthService) DeleteAuthSource(_ context.Context, _ uint64) error {
	return nil
}

// ToggleAuthSource toggles an authentication source active state.
func (a *MockAuthService) ToggleAuthSource(_ context.Context, _ uint64) (*contracts.AuthSourceDTO, error) {
	return nil, nil
}

// SetupTestEnv initializes test helper environment and binds DB, Cache, Storage, Auth mocks to shared services.
func SetupTestEnv(t *testing.T) (*gorm.DB, func()) {
	dbConn, _, cleanup := testhelper.SetupTestEnvironment(t)
	dbSvc := &MockDBService{DBInstance: dbConn}
	cacheSvc := NewMockCacheService()
	storageSvc := NewMockStorageService()
	authSvc := &MockAuthService{DB: dbConn}

	SetDBService(dbSvc)
	SetCacheService(cacheSvc)
	SetStorageService(storageSvc)
	SetAuthService(authSvc)

	return dbConn, func() {
		ResetServices()
		cleanup()
	}
}
