// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package filesrv

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/Rain-kl/Wavelet/core/contracts"
	"github.com/Rain-kl/Wavelet/pkg/response"
	"github.com/Rain-kl/Wavelet/pkg/testhelper"
	"github.com/Rain-kl/Wavelet/plugins/domain/upload/cache"
	"github.com/Rain-kl/Wavelet/plugins/domain/upload/models"
	"github.com/Rain-kl/Wavelet/plugins/domain/upload/shared"
	uploadutil "github.com/Rain-kl/Wavelet/plugins/domain/upload/util"
	"github.com/Rain-kl/Wavelet/plugins/infra/storage/diskcache"
	"github.com/Rain-kl/Wavelet/plugins/infra/storage/objectstore"
)

func init() {
	testhelper.RegisterCleanup(cache.ResetUploadMetaCacheForTest)
}

func TestServeFileByIDAccessControl(t *testing.T) {
	dbConn, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()
	cache.ResetAccessCaches()

	tempDir := t.TempDir()
	configureLocalStorageRoot(t, dbConn, tempDir)

	// Create a user in DB
	user := contracts.UserDTO{
		ID:       12345,
		Username: "file_test_user",
		IsActive: true,
	}
	if err := dbConn.Table("w_users").Create(&user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// Create an access token for this user
	tokenStr := "test-secret-token-123"
	tokenHash := fmt.Sprintf("%x", sha256.Sum256([]byte(tokenStr)))
	tokenRecord := map[string]any{
		"user_id":      user.ID,
		"name":         "test_token",
		"token_hash":   tokenHash,
		"masked_token": "test-***",
	}
	if err := dbConn.Table("w_access_tokens").Create(&tokenRecord).Error; err != nil {
		t.Fatalf("failed to create token: %v", err)
	}

	// Create two files: one in whitelist (avatar), one not in whitelist (attachment)
	avatarFile := models.Upload{
		ID:         8001,
		UserID:     user.ID,
		FileName:   "avatar.png",
		FilePath:   "avatar.png",
		FileSize:   5,
		MimeType:   "image/png",
		Extension:  "png",
		Type:       "avatar",
		Status:     models.UploadStatusUsed,
		AccessMode: 1,
	}
	attachmentFile := models.Upload{
		ID:         8002,
		UserID:     user.ID,
		FileName:   "doc.pdf",
		FilePath:   "doc.pdf",
		FileSize:   5,
		MimeType:   "application/pdf",
		Extension:  "pdf",
		Type:       "attachment",
		Status:     models.UploadStatusUsed,
		AccessMode: 1,
	}

	if err := os.WriteFile(filepath.Join(tempDir, "avatar.png"), []byte("image"), 0644); err != nil {
		t.Fatalf("failed to write avatar file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "doc.pdf"), []byte("bytes"), 0644); err != nil {
		t.Fatalf("failed to write attachment file: %v", err)
	}

	dbConn.Create(&avatarFile)
	dbConn.Create(&attachmentFile)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(response.ErrorHandlerMiddleware())
	store := cookie.NewStore([]byte("secret"))
	r.Use(sessions.Sessions("wavelet_session_id", store))
	r.GET("/f/:id", ServeFileByID)

	t.Run("public access allowed for whitelist type (avatar)", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/f/8001", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200 for public file, got %d", w.Code)
		}
		if w.Body.String() != "image" {
			t.Fatalf("expected body 'image', got '%s'", w.Body.String())
		}
	})

	t.Run("public access rejected for non-whitelist type (attachment)", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/f/8002", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected status 401 for private file without auth, got %d", w.Code)
		}
	})

	t.Run("authenticated access allowed for non-whitelist type (attachment)", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/f/8002", nil)
		req.Header.Set("Authorization", "Bearer "+tokenStr)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200 for authenticated request, got %d", w.Code)
		}
		if w.Body.String() != "bytes" {
			t.Fatalf("expected body 'bytes', got '%s'", w.Body.String())
		}
	})

	t.Run("non-existent file returns 404", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/f/99999", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("expected status 404 for non-existent file, got %d", w.Code)
		}
	})

	t.Run("invalid id format returns 400", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/f/invalid-id", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400 for invalid ID, got %d", w.Code)
		}
	})
}

func TestServeFileByIDImageCompression(t *testing.T) {
	dbConn, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()
	cache.ResetAccessCaches()

	tempDir := t.TempDir()
	configureLocalStorageRoot(t, dbConn, tempDir)

	cache := diskcache.GetGlobalCache()
	if err := cache.Clear(); err != nil {
		t.Fatalf("failed to clear disk cache before test: %v", err)
	}

	defer func() {
		if err := cache.Clear(); err != nil {
			t.Errorf("failed to clear disk cache after test: %v", err)
		}
	}()

	// Create test user
	user := contracts.UserDTO{
		ID:       555,
		Username: "compress_tester",
		IsActive: true,
	}
	dbConn.Table("w_users").Create(&user)

	// Create a 1x1 pixel PNG image
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{R: 255, G: 0, B: 0, A: 255})
	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, img); err != nil {
		t.Fatalf("failed to encode test png: %v", err)
	}

	filePath := filepath.Join(tempDir, "test_image.png")
	if err := os.WriteFile(filePath, pngBuf.Bytes(), 0644); err != nil {
		t.Fatalf("failed to write test png: %v", err)
	}

	// Save upload record to DB
	uploadRecord := models.Upload{
		ID:         3001,
		UserID:     user.ID,
		FileName:   "test_image.png",
		FilePath:   "test_image.png",
		FileSize:   int64(pngBuf.Len()),
		MimeType:   "image/png",
		Extension:  "png",
		Type:       "avatar", // Whitelisted by default
		Status:     models.UploadStatusUsed,
		AccessMode: 1,
	}
	dbConn.Create(&uploadRecord)

	// Setup Router
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/f/:id", ServeFileByID)

	t.Run("serve original file without compress parameter", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/f/3001", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}
		// Content-Type should be image/png (default local serving type)
		if w.Header().Get("Content-Type") != "image/png" {
			t.Errorf("expected Content-Type image/png, got %s", w.Header().Get("Content-Type"))
		}
		if w.Header().Get("X-Cache") != "" {
			t.Errorf("expected no X-Cache header for original file, got %s", w.Header().Get("X-Cache"))
		}
		if w.Header().Get("ETag") == "" {
			t.Errorf("expected ETag header for original file")
		}
	})

	t.Run("first request with quality=medium produces cache MISS and converts to WebP", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/f/3001?quality=medium", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}
		if w.Header().Get("Content-Type") != "image/webp" {
			t.Errorf("expected Content-Type image/webp, got %s", w.Header().Get("Content-Type"))
		}
		if w.Header().Get("X-Cache") != "MISS" {
			t.Errorf("expected X-Cache MISS on first compress request, got %s", w.Header().Get("X-Cache"))
		}
		if w.Header().Get("ETag") == "" {
			t.Errorf("expected ETag header")
		}
		if len(w.Body.Bytes()) == 0 {
			t.Errorf("expected non-empty body")
		}
	})

	t.Run("second request with quality=medium produces cache HIT", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/f/3001?quality=medium", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}
		if w.Header().Get("Content-Type") != "image/webp" {
			t.Errorf("expected Content-Type image/webp, got %s", w.Header().Get("Content-Type"))
		}
		if w.Header().Get("X-Cache") != "HIT" {
			t.Errorf("expected X-Cache HIT on second compress request, got %s", w.Header().Get("X-Cache"))
		}
	})

	t.Run("request with quality=origin behaves like original request", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/f/3001?quality=origin", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}
		if w.Header().Get("Content-Type") != "image/png" {
			t.Errorf("expected Content-Type image/png, got %s", w.Header().Get("Content-Type"))
		}
		if w.Header().Get("X-Cache") != "" {
			t.Errorf("expected no X-Cache header for origin quality, got %s", w.Header().Get("X-Cache"))
		}
	})

	t.Run("conditional GET with matching If-None-Match returns 304", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/f/3001?quality=medium", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		etag := w.Header().Get("ETag")
		if etag == "" {
			t.Fatalf("expected ETag header from initial request")
		}

		// Second request with If-None-Match
		req2, _ := http.NewRequest("GET", "/f/3001?quality=medium", nil)
		req2.Header.Set("If-None-Match", etag)
		w2 := httptest.NewRecorder()
		r.ServeHTTP(w2, req2)

		if w2.Code != http.StatusNotModified {
			t.Fatalf("expected status 304 Not Modified, got %d", w2.Code)
		}
		if w2.Body.Len() != 0 {
			t.Errorf("expected empty body on 304 response, got %d bytes", w2.Body.Len())
		}
	})
}

func TestNormalizeImageQuality(t *testing.T) {
	tests := []struct {
		name    string
		quality string
		want    string
	}{
		{name: "empty quality returns origin", quality: "", want: shared.ImageQualityOrigin},
		{name: "origin returns origin", quality: "origin", want: shared.ImageQualityOrigin},
		{name: "ORIGIN case-insensitive returns origin", quality: "ORIGIN", want: shared.ImageQualityOrigin},
		{name: "low returns low", quality: "low", want: shared.ImageQualityLow},
		{name: "LOW returns low", quality: "LOW", want: shared.ImageQualityLow},
		{name: "medium returns medium", quality: "medium", want: shared.ImageQualityMedium},
		{name: "high returns high", quality: "high", want: shared.ImageQualityHigh},
		{name: "unknown quality defaults to origin", quality: "ultra_hd", want: shared.ImageQualityOrigin},
		{name: "whitespace padded quality is trimmed", quality: "  medium  ", want: shared.ImageQualityMedium},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := uploadutil.NormalizeImageQuality(tt.quality); got != tt.want {
				t.Errorf("NormalizeImageQuality(%q) = %q, want %q", tt.quality, got, tt.want)
			}
		})
	}
}

func configureLocalStorageRoot(t *testing.T, dbConn *gorm.DB, tempDir string) {
	var sc struct {
		Key   string
		Value string
	}
	if err := dbConn.Table("w_system_configs").Where("key = ?", "storage_config").First(&sc).Error; err != nil {
		t.Fatalf("failed to find storage config: %v", err)
	}
	var cfg objectstore.Config
	if err := json.Unmarshal([]byte(sc.Value), &cfg); err != nil {
		t.Fatalf("failed to unmarshal storage config: %v", err)
	}
	cfg.Local.Root = tempDir
	newVal, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("failed to marshal storage config: %v", err)
	}
	sc.Value = string(newVal)
	if err := dbConn.Table("w_system_configs").Where("key = ?", "storage_config").Update("value", sc.Value).Error; err != nil {
		t.Fatalf("failed to save storage config: %v", err)
	}
	objectstore.ResetCache()
}
