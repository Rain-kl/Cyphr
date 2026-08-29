// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package core_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"Wavelet/core"
	"Wavelet/core/extpoints"
)

// mapSource implements extpoints.ConfigSource over static maps.
type mapSource struct {
	values map[string]any
	env    map[string]string
}

func (m *mapSource) Lookup(path string) (any, bool) {
	v, ok := m.values[path]
	return v, ok
}

func (m *mapSource) LookupEnv(name string) (string, bool) {
	v, ok := m.env[name]
	return v, ok
}

func (m *mapSource) Describe() string { return "map" }

type otelConfig struct {
	SamplingRate float64 `config:"sampling_rate" env:"OTEL_SAMPLING_RATE"`
}

// newOtelRegistry declares the otel section against a source carrying the given file values.
func newOtelRegistry(t *testing.T, values map[string]any) extpoints.ConfigExtension {
	t.Helper()

	r := extpoints.NewConfigRegistry(&mapSource{values: values, env: map[string]string{}})
	require.NoError(t, r.Declare("host", extpoints.ConfigBinding{Prefix: "otel", Target: &otelConfig{}}))
	require.NoError(t, r.Resolve())
	return r
}

func TestConfigGetReturnsDeclaredType(t *testing.T) {
	view := newOtelRegistry(t, map[string]any{"otel.sampling_rate": 0.25})

	rate, err := core.ConfigGet[float64](view, "otel.sampling_rate")
	require.NoError(t, err)
	assert.Equal(t, 0.25, rate)
}

func TestConfigGetRejectsTypeMismatch(t *testing.T) {
	view := newOtelRegistry(t, map[string]any{"otel.sampling_rate": 0.25})

	text, err := core.ConfigGet[string](view, "otel.sampling_rate")
	require.ErrorIs(t, err, extpoints.ErrConfigType)
	assert.Empty(t, text)
}

func TestConfigGetRejectsUndeclaredKey(t *testing.T) {
	view := newOtelRegistry(t, nil)

	_, err := core.ConfigGet[float64](view, "otel.unregistered")
	require.ErrorIs(t, err, extpoints.ErrConfigUnknownKey)
}

func TestConfigGetRejectsNilView(t *testing.T) {
	_, err := core.ConfigGet[float64](nil, "otel.sampling_rate")
	require.ErrorIs(t, err, extpoints.ErrConfigNotResolved)
}
