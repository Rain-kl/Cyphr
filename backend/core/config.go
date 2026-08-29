// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"fmt"
	"strings"

	"Wavelet/core/extpoints"
)

// ConfigGet reads one resolved configuration value with its declared type. It is the
// generic counterpart of the fallback accessors on ConfigView, used when a caller must
// distinguish "unset" from "set to the zero value".
func ConfigGet[T any](view extpoints.ConfigView, key string) (T, error) {
	var zero T
	if view == nil {
		return zero, extpoints.ErrConfigNotResolved
	}

	raw, ok := view.Value(key)
	if !ok {
		return zero, fmt.Errorf("%w: %s", extpoints.ErrConfigUnknownKey, key)
	}

	value, ok := raw.(T)
	if !ok {
		return zero, fmt.Errorf("%w: key %q holds %T, want %T", extpoints.ErrConfigType, key, raw, zero)
	}
	return value, nil
}

// MapSource implements ConfigSource backed by an in-memory map, ideal for unit tests.
type MapSource struct {
	values map[string]any
	env    map[string]string
}

// NewMapSource creates a new MapSource with the provided key-value mappings.
func NewMapSource(values map[string]any) *MapSource {
	vals := make(map[string]any, len(values))
	for k, v := range values {
		vals[k] = v
	}
	return &MapSource{
		values: vals,
		env:    make(map[string]string),
	}
}

// Lookup returns the value at the given path, supporting both flat keys and nested maps.
func (m *MapSource) Lookup(path string) (any, bool) {
	if m == nil || m.values == nil {
		return nil, false
	}
	if v, ok := m.values[path]; ok {
		return v, true
	}
	parts := strings.Split(path, ".")
	var cur any = m.values
	for _, part := range parts {
		mCur, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = mCur[part]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}

// LookupEnv returns the environment variable value.
func (m *MapSource) LookupEnv(name string) (string, bool) {
	if m == nil || m.env == nil {
		return "", false
	}
	v, ok := m.env[name]
	return v, ok
}

// SetEnv sets an environment variable for testing.
func (m *MapSource) SetEnv(name, value string) {
	if m.env == nil {
		m.env = make(map[string]string)
	}
	m.env[name] = value
}

// Describe describes the MapSource.
func (m *MapSource) Describe() string {
	return "<map source>"
}

// WithConfigValues returns an AppOption that installs a MapSource with the given key-value mappings.
func WithConfigValues(values map[string]any) AppOption {
	return WithConfigSource(NewMapSource(values))
}
