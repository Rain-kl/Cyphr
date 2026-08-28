// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package driver_http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Rain-kl/Wavelet/pkg/testhelper"
	"github.com/gin-gonic/gin"
)

func TestCORSMiddleware(t *testing.T) {
	dbConn, _, cleanup := testhelper.SetupTestEnvironment(t)
	defer cleanup()

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
