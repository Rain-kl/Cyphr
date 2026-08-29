// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package driver_http

import (
	"Wavelet/core/contracts"
	"Wavelet/pkg/testhelper"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type mockDBService struct {
	db *gorm.DB
}

func (m *mockDBService) GORM() *gorm.DB {
	return m.db
}

func (m *mockDBService) DB(ctx context.Context) *gorm.DB {
	return m.db.WithContext(ctx)
}

func (m *mockDBService) Named(_ string) *gorm.DB {
	return m.db
}

func TestCORSMiddleware(t *testing.T) {
	dbConn, _, cleanup := testhelper.SetupTestEnvironment(t)
	setDBService(&mockDBService{db: dbConn})
	defer func() {
		setDBService(nil)
		cleanup()
	}()

	gin.SetMode(gin.TestMode)

	clearConfigCache := func() {}

	t.Run("missing server_address configuration returns no CORS headers", func(t *testing.T) {
		clearConfigCache()
		if err := dbConn.Table("w_system_configs").Where("key = ?", "server_address").Update("value", "").Error; err != nil {
			t.Fatalf("failed to update config: %v", err)
		}

		r := gin.New()
		r.Use(corsMiddleware())
		r.GET("/test", func(c *gin.Context) {
			c.String(http.StatusOK, "ok")
		})

		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Origin", "http://attacker.com")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Errorf("expected no Access-Control-Allow-Origin, got %s", w.Header().Get("Access-Control-Allow-Origin"))
		}
	})

	t.Run("configured server_address allows exact origin match", func(t *testing.T) {
		if err := dbConn.Table("w_system_configs").Where("key = ?", "server_address").Update("value", "http://trusted.com").Error; err != nil {
			t.Fatalf("failed to update config: %v", err)
		}

		r := gin.New()
		r.Use(corsMiddleware())
		r.GET("/test", func(c *gin.Context) {
			c.String(http.StatusOK, "ok")
		})

		// Trusted origin
		req, _ := http.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Origin", "http://trusted.com")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Header().Get("Access-Control-Allow-Origin") != "http://trusted.com" {
			t.Errorf("expected Access-Control-Allow-Origin http://trusted.com, got %s", w.Header().Get("Access-Control-Allow-Origin"))
		}
		if w.Header().Get("Access-Control-Allow-Credentials") != "true" {
			t.Errorf("expected Access-Control-Allow-Credentials true, got %s", w.Header().Get("Access-Control-Allow-Credentials"))
		}

		// Untrusted origin
		req, _ = http.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("Origin", "http://attacker.com")
		w = httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Errorf("expected no Access-Control-Allow-Origin for attacker, got %s", w.Header().Get("Access-Control-Allow-Origin"))
		}
	})

	t.Run("preflight OPTIONS request responds with 204", func(t *testing.T) {
		if err := dbConn.Table("w_system_configs").Where("key = ?", "server_address").Update("value", "http://trusted.com").Error; err != nil {
			t.Fatalf("failed to update config: %v", err)
		}

		r := gin.New()
		r.Use(corsMiddleware())

		req, _ := http.NewRequest(http.MethodOptions, "/test", nil)
		req.Header.Set("Origin", "http://trusted.com")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusNoContent {
			t.Errorf("expected status 204, got %d", w.Code)
		}
		if w.Header().Get("Access-Control-Allow-Methods") == "" {
			t.Error("expected Access-Control-Allow-Methods header")
		}
	})
}

// memoryCache 是一个可工作的内存缓存，用于统计 loader（即数据库查询）实际执行次数。
type memoryCache struct {
	values map[string]string
	loads  int
}

func (c *memoryCache) Get(_ context.Context, key string, target any) error {
	v, ok := c.values[key]
	if !ok {
		return contracts.ErrCacheMiss
	}
	dst, ok := target.(*string)
	if !ok {
		return contracts.ErrCacheMiss
	}
	*dst = v
	return nil
}

func (c *memoryCache) Set(_ context.Context, key string, value any, _ time.Duration) error {
	v, ok := value.(string)
	if !ok {
		return nil
	}
	c.values[key] = v
	return nil
}

func (c *memoryCache) Delete(_ context.Context, key string) error {
	delete(c.values, key)
	return nil
}

func (c *memoryCache) GetOrSet(_ context.Context, key string, target any, _ time.Duration, loader func() (any, error)) error {
	if err := c.Get(context.Background(), key, target); err == nil {
		return nil
	}
	raw, err := loader()
	if err != nil {
		return err
	}
	c.loads++
	v, ok := raw.(string)
	if !ok {
		return nil
	}
	c.values[key] = v
	if dst, ok := target.(*string); ok {
		*dst = v
	}
	return nil
}

func (c *memoryCache) Invalidate(ctx context.Context, key string) error {
	return c.Delete(ctx, key)
}

// TestCORSAllowedOriginReadsConfigOncePerCacheWindow 回归：CORS 的来源校验不得在
// 每个请求上都查询主库，缓存有效期内 loader 只应执行一次。
func TestCORSAllowedOriginReadsConfigOncePerCacheWindow(t *testing.T) {
	dbConn, _, cleanup := testhelper.SetupTestEnvironment(t)
	setDBService(&mockDBService{db: dbConn})
	cache := &memoryCache{values: map[string]string{}}
	setCacheService(cache)
	defer func() {
		setCacheService(nil)
		setDBService(nil)
		cleanup()
	}()

	ctx := context.Background()
	const origin = "http://localhost:8000" // testhelper 预置的 server_address
	for range 3 {
		if !isOriginAllowed(ctx, origin) {
			t.Fatalf("origin %q should be allowed", origin)
		}
	}

	if cache.loads != 1 {
		t.Errorf("expected 1 config load across 3 requests, got %d", cache.loads)
	}
}
