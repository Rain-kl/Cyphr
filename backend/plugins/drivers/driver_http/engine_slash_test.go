// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package driver_http

import "testing"

func TestBuildEngineDefaultRedirectsTrailingSlash(t *testing.T) {
	eng, err := BuildEngineWithConfig(httpAppConfig{}, httpRedisConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if !eng.RedirectTrailingSlash {
		t.Fatal("default RedirectTrailingSlash must be true")
	}
}

func TestBuildEngineCanDisableRedirectTrailingSlash(t *testing.T) {
	eng, err := BuildEngineWithConfig(httpAppConfig{RedirectTrailingSlash: boolPtr(false)}, httpRedisConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if eng.RedirectTrailingSlash {
		t.Fatal("RedirectTrailingSlash must honor false")
	}
}

func boolPtr(v bool) *bool { return &v }
