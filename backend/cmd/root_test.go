// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfigSource(t *testing.T) {
	origConfigPath := configPath
	t.Cleanup(func() {
		configPath = origConfigPath
	})

	t.Run("default loads config.yaml in current directory", func(t *testing.T) {
		tempDir := t.TempDir()
		configFile := filepath.Join(tempDir, "config.yaml")
		require.NoError(t, os.WriteFile(configFile, []byte("app:\n  app_name: custom-app\n"), 0o600))

		origWd, err := os.Getwd()
		require.NoError(t, err)
		require.NoError(t, os.Chdir(tempDir))
		t.Cleanup(func() {
			_ = os.Chdir(origWd)
		})

		configPath = ""
		t.Setenv("CONFIG_PATH", "")

		src, err := loadConfigSource()
		require.NoError(t, err)
		val, ok := src.Lookup("app.app_name")
		require.True(t, ok)
		assert.Equal(t, "custom-app", val)
	})

	t.Run("explicit --config flag loads targeted file", func(t *testing.T) {
		tempDir := t.TempDir()
		customFile := filepath.Join(tempDir, "my-special-config.yaml")
		require.NoError(t, os.WriteFile(customFile, []byte("app:\n  app_name: flag-app\n"), 0o600))

		configPath = customFile
		src, err := loadConfigSource()
		require.NoError(t, err)
		val, ok := src.Lookup("app.app_name")
		require.True(t, ok)
		assert.Equal(t, "flag-app", val)
	})

	t.Run("explicit --config with non-existent file returns error", func(t *testing.T) {
		configPath = filepath.Join(t.TempDir(), "non-existent.yaml")
		_, err := loadConfigSource()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}
