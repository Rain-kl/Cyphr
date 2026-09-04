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

func setupPurgeCache(t *testing.T, contents ...string) string {
	t.Helper()
	cacheDir := t.TempDir()
	t.Setenv("CYPHR_MEDIA_CACHE_DIR", cacheDir)
	for i, c := range contents {
		require.NoError(t, os.WriteFile(
			filepath.Join(cacheDir, "cached_"+string(rune('a'+i))+".mp3"),
			[]byte(c),
			0o600,
		))
	}
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	return cfgPath
}

func TestPurgeDryRunKeepsFiles(t *testing.T) {
	cfgPath := setupPurgeCache(t, "audio-one", "audio-two")

	root := NewRootCmd()
	out, err := executeCommand(root, "purge", "--dry-run", "--config", cfgPath)
	require.NoError(t, err)
	assert.Contains(t, out, "2 file(s)")
	assert.Contains(t, out, "dry-run")

	entries, readErr := os.ReadDir(os.Getenv("CYPHR_MEDIA_CACHE_DIR"))
	require.NoError(t, readErr)
	assert.Len(t, entries, 2)
}

func TestPurgeForceDeletesFiles(t *testing.T) {
	cfgPath := setupPurgeCache(t, "audio-one", "audio-two")

	root := NewRootCmd()
	out, err := executeCommand(root, "purge", "--force", "--config", cfgPath)
	require.NoError(t, err)
	assert.Contains(t, out, "Purged 2 file(s)")

	entries, readErr := os.ReadDir(os.Getenv("CYPHR_MEDIA_CACHE_DIR"))
	require.NoError(t, readErr)
	assert.Empty(t, entries)
}

func TestPurgeInteractiveConfirm(t *testing.T) {
	t.Run("yes deletes", func(t *testing.T) {
		cfgPath := setupPurgeCache(t, "audio-one")

		root := NewRootCmd()
		out, err := executeCommandWithInput(root, "y\n", "purge", "--config", cfgPath)
		require.NoError(t, err)
		assert.Contains(t, out, "Purged 1 file(s)")

		entries, readErr := os.ReadDir(os.Getenv("CYPHR_MEDIA_CACHE_DIR"))
		require.NoError(t, readErr)
		assert.Empty(t, entries)
	})

	t.Run("no aborts", func(t *testing.T) {
		cfgPath := setupPurgeCache(t, "audio-one")

		root := NewRootCmd()
		out, err := executeCommandWithInput(root, "n\n", "purge", "--config", cfgPath)
		require.NoError(t, err)
		assert.Contains(t, out, "Aborted.")

		entries, readErr := os.ReadDir(os.Getenv("CYPHR_MEDIA_CACHE_DIR"))
		require.NoError(t, readErr)
		assert.Len(t, entries, 1)
	})
}

func TestPurgeMissingCacheDir(t *testing.T) {
	cacheDir := filepath.Join(t.TempDir(), "does-not-exist")
	t.Setenv("CYPHR_MEDIA_CACHE_DIR", cacheDir)
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")

	root := NewRootCmd()
	out, err := executeCommand(root, "purge", "--force", "--config", cfgPath)
	require.NoError(t, err)
	assert.Contains(t, out, "nothing to purge")
}
