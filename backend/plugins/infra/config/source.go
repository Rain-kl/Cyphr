// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package config adapts viper to the kernel configuration source contract. It is a
// runtime adapter rather than a core.Plugin: it owns no routes, services or tasks and
// therefore never appears in app.Use. Keeping viper here preserves the micro-kernel
// rule against importing concrete runtime dependencies.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/spf13/viper"
)

// DefaultFileName is the configuration file looked up when CONFIG_PATH is unset.
const DefaultFileName = "config.yaml"

// DefaultCandidates defines the prioritized list of configuration file paths looked up
// when CONFIG_PATH is unset.
var DefaultCandidates = []string{
	"manifest/config/config.yaml",
	DefaultFileName,
}

// EnvOnlyOrigin is reported by Describe when no configuration file was loaded.
const EnvOnlyOrigin = "<env only>"

// maxSearchDepth bounds the upward directory walk so a misconfigured working directory
// cannot make the loader scan the whole filesystem.
const maxSearchDepth = 5

// Option configures a Source.
type Option func(*Source)

// WithPath pins the configuration file, bypassing CONFIG_PATH and the upward search.
func WithPath(path string) Option {
	return func(s *Source) {
		s.path = path
	}
}

// Source implements core.ConfigSource over a configuration file plus the process environment.
type Source struct {
	v     *viper.Viper
	path  string
	found bool
}

// NewSource loads the configuration file. A missing file is not an error: the source
// then serves environment values only, matching the behaviour the previous pkg/config
// loader had for deployments that configure everything through the environment.
func NewSource(opts ...Option) (*Source, error) {
	s := &Source{}
	for _, opt := range opts {
		opt(s)
	}

	if s.path == "" {
		s.path = os.Getenv("CONFIG_PATH")
	}
	if s.path == "" {
		s.path = findConfigPath(DefaultCandidates...)
	}

	v := viper.New()
	v.SetConfigFile(s.path)

	err := v.ReadInConfig()
	switch {
	case err == nil:
		s.found = true
	case isNotFound(err):
		// No file: fall through to environment-only lookups.
	default:
		if _, statErr := os.Stat(s.path); statErr == nil { //nolint:gosec // s.path comes from CONFIG_PATH or a bounded upward search
			return nil, fmt.Errorf("infra/config: read %s: %w", s.path, err)
		}
	}

	s.v = v
	return s, nil
}

// isNotFound reports whether the loader failed only because the file is absent.
func isNotFound(err error) bool {
	var notFound viper.ConfigFileNotFoundError
	return errors.As(err, &notFound) || errors.Is(err, fs.ErrNotExist)
}

// Lookup returns the raw value stored at a dotted path, or false when the file was not
// loaded or the path is absent. Declared defaults therefore stay distinguishable from
// values explicitly set to a zero.
func (s *Source) Lookup(path string) (any, bool) {
	if !s.found || !s.v.IsSet(path) {
		return nil, false
	}
	return s.v.Get(path), true
}

// LookupEnv reads a process environment variable.
func (s *Source) LookupEnv(name string) (string, bool) {
	return os.LookupEnv(name)
}

// Describe returns the loaded file path, or EnvOnlyOrigin when running on environment values.
func (s *Source) Describe() string {
	if !s.found {
		return EnvOnlyOrigin
	}
	return s.path
}

// findConfigPath searches upward from the working directory through candidate paths so
// tests and binaries run from subdirectories still find configuration files in manifest/config/
// or the repository root.
func findConfigPath(candidates ...string) string {
	if len(candidates) == 0 {
		return DefaultFileName
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	dir := "."
	for range maxSearchDepth {
		dir += "/.."
		for _, candidate := range candidates {
			path := dir + "/" + candidate
			if _, err := os.Stat(path); err == nil {
				return path
			}
		}
	}
	return candidates[0]
}
