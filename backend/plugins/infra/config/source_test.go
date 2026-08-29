// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"Wavelet/plugins/infra/config"
)

const sampleYAML = "" +
	"app:\n  addr: \":8000\"\n  node_id: 1\n" +
	"database:\n  enabled: false\n  port: 5432\n  slow_threshold: 200ms\n" +
	"redis:\n  addrs:\n    - \"127.0.0.1:6379\"\n"

func writeConfig(t *testing.T, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	return path
}

func TestSourceLooksUpNestedPaths(t *testing.T) {
	src, err := config.NewSource(config.WithPath(writeConfig(t, sampleYAML)))
	require.NoError(t, err)

	value, ok := src.Lookup("database.port")
	require.True(t, ok)
	assert.Equal(t, 5432, value)

	_, ok = src.Lookup("database.missing")
	assert.False(t, ok)
}

func TestSourceKeepsZeroValuedKeysDistinctFromMissing(t *testing.T) {
	src, err := config.NewSource(config.WithPath(writeConfig(t, sampleYAML)))
	require.NoError(t, err)

	value, ok := src.Lookup("database.enabled")
	require.True(t, ok, "an explicitly set false must not look like a missing key")
	assert.Equal(t, false, value)
}

func TestSourceTreatsUnsetFileAsEnvOnly(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "absent.yaml")

	src, err := config.NewSource(config.WithPath(missing))
	require.NoError(t, err, "a missing configuration file must fall back to environment values")

	_, ok := src.Lookup("app.addr")
	assert.False(t, ok)
	assert.Equal(t, config.EnvOnlyOrigin, src.Describe())
}

func TestSourceRejectsMalformedFile(t *testing.T) {
	src, err := config.NewSource(config.WithPath(writeConfig(t, "app: [unclosed\n")))

	assert.Nil(t, src)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "infra/config")
}

func TestSourceLookupEnvReadsProcessEnvironment(t *testing.T) {
	t.Setenv("WAVELET_SOURCE_PROBE", "present")

	src, err := config.NewSource(config.WithPath(writeConfig(t, sampleYAML)))
	require.NoError(t, err)

	value, ok := src.LookupEnv("WAVELET_SOURCE_PROBE")
	require.True(t, ok)
	assert.Equal(t, "present", value)

	_, ok = src.LookupEnv("WAVELET_SOURCE_ABSENT")
	assert.False(t, ok)
}

func TestSourcePrefersConfigPathEnvironmentVariable(t *testing.T) {
	t.Setenv("CONFIG_PATH", writeConfig(t, "app:\n  addr: \":9100\"\n"))

	src, err := config.NewSource()
	require.NoError(t, err)

	value, ok := src.Lookup("app.addr")
	require.True(t, ok)
	assert.Equal(t, ":9100", value)
}
