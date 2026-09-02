// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cap

import (
	"Wavelet/pkg/response"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestVerifyMiddlewareMissingTokenIsBadRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	restore := InstallTestRuntimeSettings(RuntimeSettings{LoginEnabled: true})
	t.Cleanup(restore)

	engine := gin.New()
	engine.Use(response.ErrorHandlerMiddleware())
	engine.POST("/register", VerifyMiddleware(GetDefaultManager(), "register"), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/register", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("VerifyMiddleware() status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}
