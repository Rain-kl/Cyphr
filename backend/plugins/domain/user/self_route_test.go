// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package user_test

import (
	"context"
	"testing"

	"Wavelet/core"
	"Wavelet/core/extpoints"
	"Wavelet/plugins/domain/user"
)

func TestSelfRouteRegisteredWithLoginProtection(t *testing.T) {
	ctx := core.NewContext(context.Background())
	if err := user.New().Apply(ctx); err != nil {
		t.Fatal(err)
	}

	routes := ctx.Router().Routes()
	self, ok := findRoute(routes, "GET", "/api/v1/user/self")
	if !ok {
		t.Fatal("GET /api/v1/user/self missing")
	}
	profile, ok := findRoute(routes, "PUT", "/api/v1/user/profile")
	if !ok {
		t.Fatal("PUT /api/v1/user/profile missing")
	}

	selfCount := len(self.Handlers) + len(self.Middlewares)
	profileCount := len(profile.Handlers) + len(profile.Middlewares)
	if selfCount < profileCount {
		t.Fatalf("GET /api/v1/user/self handler/middleware count = %d, want >= %d (profile)", selfCount, profileCount)
	}
}

func findRoute(routes []extpoints.RouteDefinition, method, path string) (extpoints.RouteDefinition, bool) {
	for _, rd := range routes {
		if rd.Method == method && rd.Path == path {
			return rd, true
		}
	}
	return extpoints.RouteDefinition{}, false
}
