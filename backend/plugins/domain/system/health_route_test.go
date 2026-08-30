// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package system

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"Wavelet/core"

	"github.com/gin-gonic/gin"
)

func TestHealthRouteReturnsOKNil(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx := core.NewContext(context.Background())
	if err := New().Apply(ctx); err != nil {
		t.Fatal(err)
	}

	healthHandler := routeHandler(t, ctx, "GET", "/api/health")
	routeHandler(t, ctx, "GET", "/healthz")
	if !ctx.Router().IsWhitelisted("/api/health") {
		t.Fatal("GET /api/health is not whitelisted")
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/health", nil)
	healthHandler(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var body struct {
		ErrorMsg string `json:"error_msg"`
		Data     any    `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.ErrorMsg != "" {
		t.Fatalf("error_msg = %q, want empty", body.ErrorMsg)
	}
	if body.Data != nil {
		t.Fatalf("data = %#v, want null", body.Data)
	}
}

func TestHealthzRouteUnchanged(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx := core.NewContext(context.Background())
	if err := New().Apply(ctx); err != nil {
		t.Fatal(err)
	}

	handler := routeHandler(t, ctx, "GET", "/healthz")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/healthz", nil)
	handler(c)

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "ok" {
		t.Fatalf("body = %s, want {status: ok}", w.Body.String())
	}
}

func routeHandler(t *testing.T, ctx *core.Context, method, path string) gin.HandlerFunc {
	t.Helper()
	for _, rd := range ctx.Router().Routes() {
		if rd.Method != method || rd.Path != path {
			continue
		}
		if len(rd.Handlers) == 0 {
			t.Fatalf("%s %s has no handlers", method, path)
		}
		switch h := rd.Handlers[0].(type) {
		case gin.HandlerFunc:
			return h
		case func(*gin.Context):
			return h
		default:
			t.Fatalf("unexpected handler type %T", rd.Handlers[0])
		}
	}
	t.Fatalf("%s %s missing", method, path)
	return nil
}
