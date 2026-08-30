// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package driver_http

import (
	"testing"

	"Wavelet/core/extpoints"
)

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

func TestBindAppConfigDefaultKeepsTrailingSlashRedirect(t *testing.T) {
	cfg := bindAppConfig(t, map[string]any{}, map[string]string{})
	if cfg.RedirectTrailingSlash != nil {
		t.Fatal("absent redirect_trailing_slash must leave *bool nil")
	}
	eng, err := BuildEngineWithConfig(cfg, httpRedisConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if !eng.RedirectTrailingSlash {
		t.Fatal("default RedirectTrailingSlash must be true after Bind")
	}
}

func TestBindAppConfigCanDisableTrailingSlashRedirect(t *testing.T) {
	cfg := bindAppConfig(t, map[string]any{"app.redirect_trailing_slash": false}, nil)
	if cfg.RedirectTrailingSlash == nil || *cfg.RedirectTrailingSlash {
		t.Fatal("yaml false must bind *false")
	}
	eng, err := BuildEngineWithConfig(cfg, httpRedisConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if eng.RedirectTrailingSlash {
		t.Fatal("RedirectTrailingSlash must honor bound false")
	}
}

func TestBindAppConfigEnvCanDisableTrailingSlashRedirect(t *testing.T) {
	cfg := bindAppConfig(t, nil, map[string]string{"APP_REDIRECT_TRAILING_SLASH": "false"})
	if cfg.RedirectTrailingSlash == nil || *cfg.RedirectTrailingSlash {
		t.Fatal("env false must bind *false")
	}
	eng, err := BuildEngineWithConfig(cfg, httpRedisConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if eng.RedirectTrailingSlash {
		t.Fatal("RedirectTrailingSlash must honor env-bound false")
	}
}

type slashConfigSource struct {
	values map[string]any
	env    map[string]string
}

func (s slashConfigSource) Lookup(path string) (any, bool) {
	v, ok := s.values[path]
	return v, ok
}

func (s slashConfigSource) LookupEnv(name string) (string, bool) {
	v, ok := s.env[name]
	return v, ok
}

func (s slashConfigSource) Describe() string { return "slash-test" }

func bindAppConfig(t *testing.T, values map[string]any, env map[string]string) httpAppConfig {
	t.Helper()
	if values == nil {
		values = map[string]any{}
	}
	if env == nil {
		env = map[string]string{}
	}
	r := extpoints.NewConfigRegistry(slashConfigSource{values: values, env: env})
	if err := r.Declare("driver_http", extpoints.ConfigBinding{Prefix: "app", Target: &httpAppConfig{}}); err != nil {
		t.Fatal(err)
	}
	if err := r.Resolve(); err != nil {
		t.Fatal(err)
	}
	var cfg httpAppConfig
	if err := r.Bind("app", &cfg); err != nil {
		t.Fatal(err)
	}
	return cfg
}

func boolPtr(v bool) *bool { return &v }
