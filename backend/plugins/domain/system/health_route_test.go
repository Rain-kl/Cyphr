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

func TestHealthzIsTheOnlyHealthRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx := core.NewContext(context.Background())
	if err := New().Apply(ctx); err != nil {
		t.Fatal(err)
	}

	var hasHealthz bool
	for _, rd := range ctx.Router().Routes() {
		key := rd.Method + " " + rd.Path
		switch key {
		case "GET /api/healthz":
			hasHealthz = true
		case "GET /healthz", "GET /api/health":
			t.Errorf("removed health route still registered: %s", key)
		}
	}
	if !hasHealthz {
		t.Fatal("GET /api/healthz missing")
	}
	if !ctx.Router().IsWhitelisted("/api/healthz") {
		t.Fatal("GET /api/healthz not whitelisted")
	}

	handler := routeHandler(t, ctx, "GET", "/api/healthz")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/healthz", nil)
	handler(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

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
