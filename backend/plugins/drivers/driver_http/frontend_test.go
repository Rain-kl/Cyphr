// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package driver_http

import (
	"slices"
	"testing"
)

func TestAssetCandidates(t *testing.T) {
	tests := []struct {
		name     string
		urlPath  string
		expected []string
	}{
		{
			name:     "root path",
			urlPath:  "/",
			expected: []string{"index.html"},
		},
		{
			name:     "static asset with extension",
			urlPath:  "/static/css/app.css",
			expected: []string{"static/css/app.css", "index.html"},
		},
		{
			name:    "clean url route",
			urlPath: "/asr",
			expected: []string{
				"asr",
				"asr.html",
				"asr/index.html",
				"index.html",
			},
		},
		{
			name:    "dynamic route with parameter",
			urlPath: "/asr/jobs/100248215353298944",
			expected: []string{
				"asr/jobs/100248215353298944",
				"asr/jobs/100248215353298944.html",
				"asr/jobs/100248215353298944/index.html",
				"asr/jobs/0.html",
				"asr/jobs/[id].html",
				"asr/jobs/[...slug].html",
				"asr/jobs/index.html",
				"asr/0.html",
				"asr/[id].html",
				"asr/[...slug].html",
				"asr/index.html",
				"index.html",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := assetCandidates(tt.urlPath)
			for _, exp := range tt.expected {
				if !slices.Contains(got, exp) {
					t.Errorf("assetCandidates(%q) missing expected candidate %q, got: %v", tt.urlPath, exp, got)
				}
			}
		})
	}
}
