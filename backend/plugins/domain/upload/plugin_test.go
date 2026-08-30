// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package upload

import (
	"context"
	"testing"

	"github.com/gin-gonic/gin"

	"Wavelet/core"
	"Wavelet/core/contracts"
)

type stubDBService struct{ contracts.DBService }

type stubStorageService struct{ contracts.StorageService }

type stubAuthService struct{ contracts.AuthService }

func (stubAuthService) RequireAuthMiddleware() any {
	return gin.HandlerFunc(func(c *gin.Context) { c.Next() })
}

func TestUserUploadRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx := core.NewContext(context.Background())
	core.Provide[contracts.DBService](ctx, stubDBService{})
	core.Provide[contracts.StorageService](ctx, stubStorageService{})
	core.Provide[contracts.AuthService](ctx, stubAuthService{})
	if err := New().Apply(ctx); err != nil {
		t.Fatal(err)
	}

	want := []string{
		"GET /api/v1/upload/my",
		"PUT /api/v1/upload/:id",
		"GET /api/v1/upload/download/:id",
		"POST /api/v1/upload/download/batch",
		"GET /api/v1/upload",
		"POST /api/v1/upload/batch-download",
	}
	found := make(map[string]bool, len(want))
	for _, rd := range ctx.Router().Routes() {
		found[rd.Method+" "+rd.Path] = true
	}
	for _, key := range want {
		if !found[key] {
			t.Errorf("missing route %s", key)
		}
	}
}
