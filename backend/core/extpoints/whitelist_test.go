// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package extpoints_test

import (
	"Wavelet/core/extpoints"
	"testing"
)

// whitelistEquivalencePatterns and paths cover every matching rule MatchPathPattern
// implements, so PathWhitelist.Match can be pinned against the behaviour it replaces.
var (
	whitelistEquivalencePatterns = []string{
		"/api/v1/user/login",
		"/api/v1/oauth/*",
		"/api/v1/cap/:source/authorize",
		"/api/v1/files/*/download",
		"/",
		"login",
		"/api/v1/x/",
		"",
	}

	whitelistEquivalencePaths = []string{
		"/api/v1/user/login",
		"/api/v1/user/login/",
		"/api/v1/oauth/callback",
		"/api/v1/oauth",
		"/api/v1/oauth/a/b",
		"/api/v1/cap/github/authorize",
		"/api/v1/cap/:source/authorize",
		"/api/v1/files/abc/download",
		"/api/v1/files/a/b/download",
		"/",
		"login",
		"/login",
		"",
		"/api/v1/x",
		"/api/v1/x/y",
	}
)

// legacyMatch reproduces the per-request loop every whitelist caller used before
// PathWhitelist existed.
func legacyMatch(patterns []string, path string) bool {
	for _, pattern := range patterns {
		if extpoints.MatchPathPattern(pattern, path) {
			return true
		}
	}
	return false
}

func TestPathWhitelistMatchesLegacyLoop(t *testing.T) {
	for _, pattern := range whitelistEquivalencePatterns {
		wl := extpoints.NewPathWhitelist(pattern)
		for _, path := range whitelistEquivalencePaths {
			got := wl.Match(path)
			want := legacyMatch([]string{pattern}, path)
			if got != want {
				t.Errorf("pattern %q path %q: Match=%v, legacy=%v", pattern, path, got, want)
			}
		}
	}
}

func TestPathWhitelistAccumulatesAcrossRegistration(t *testing.T) {
	wl := extpoints.NewPathWhitelist("/api/v1/a")
	wl.Add("/api/v1/b/*")

	if !wl.Match("/api/v1/a") {
		t.Error("first registration lost")
	}
	if !wl.Match("/api/v1/b/deep") {
		t.Error("second registration lost")
	}
	if wl.Match("/api/v1/c") {
		t.Error("path outside both registrations matched")
	}

	got := wl.Patterns()
	want := []string{"/api/v1/a", "/api/v1/b/*"}
	if len(got) != len(want) {
		t.Fatalf("Patterns() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Patterns()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestPathWhitelistReplaceDropsPreviousPatterns(t *testing.T) {
	wl := extpoints.NewPathWhitelist("/api/v1/a")
	wl.Replace("/api/v1/b")

	if wl.Match("/api/v1/a") {
		t.Error("Replace kept a pattern it should have discarded")
	}
	if !wl.Match("/api/v1/b") {
		t.Error("Replace did not install the new pattern")
	}
}

// whitelistBenchPatterns mirrors a realistically sized auth whitelist.
var whitelistBenchPatterns = []string{
	"/api/v1/user/login",
	"/api/v1/auth/refresh",
	"/api/v1/oauth/*",
	"/api/v1/cap/*",
	"/api/v1/public/config",
	"/api/v1/health",
	"/api/v1/uploads/:id/file",
	"/api/v1/notify/webhook/:channel",
	"/login",
	"/api/v1/access-tokens/:id/revoke",
}

// TestPathWhitelistAllocationReduction asserts the point of pre-compiling: a single
// Match must allocate less than the legacy per-pattern loop it replaces.
func TestPathWhitelistAllocationReduction(t *testing.T) {
	wl := extpoints.NewPathWhitelist(whitelistBenchPatterns...)

	legacy := testing.Benchmark(func(b *testing.B) {
		for b.Loop() {
			_ = legacyMatch(whitelistBenchPatterns, "/api/v1/uploads/9/file")
		}
	})
	compiled := testing.Benchmark(func(b *testing.B) {
		for b.Loop() {
			_ = wl.Match("/api/v1/uploads/9/file")
		}
	})

	legacyAlloc := legacy.AllocsPerOp()
	compiledAlloc := compiled.AllocsPerOp()
	t.Logf("legacy %d allocs/op, PathWhitelist %d allocs/op", legacyAlloc, compiledAlloc)

	if compiledAlloc >= legacyAlloc {
		t.Errorf("PathWhitelist allocated %d/op, want fewer than legacy %d/op", compiledAlloc, legacyAlloc)
	}
}

func BenchmarkLegacyWhitelistMatch(b *testing.B) {
	for b.Loop() {
		_ = legacyMatch(whitelistBenchPatterns, "/api/v1/uploads/9/file")
	}
}

func BenchmarkPathWhitelistMatch(b *testing.B) {
	wl := extpoints.NewPathWhitelist(whitelistBenchPatterns...)
	b.ResetTimer()
	for b.Loop() {
		_ = wl.Match("/api/v1/uploads/9/file")
	}
}
