// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package media

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMediaDetection(t *testing.T) {
	tests := []struct {
		filename  string
		wantVideo bool
		wantAudio bool
	}{
		{"sample.mp4", true, false},
		{"sample.MP4", true, false},
		{"movie.mkv", true, false},
		{"clip.avi", true, false},
		{"video.MOV", true, false},
		{"stream.flv", true, false},
		{"web.webm", true, false},

		{"song.mp3", false, true},
		{"audio.wav", false, true},
		{"podcast.m4a", false, true},
		{"lossless.flac", false, true},
		{"record.aac", false, true},
		{"voice.ogg", false, true},
		{"voice.OGG", false, true},

		{"document.pdf", false, false},
		{"image.jpg", false, false},
		{"notes.txt", false, false},
		{"payload.json", false, false},
	}

	for _, tc := range tests {
		t.Run(tc.filename, func(t *testing.T) {
			assert.Equal(t, tc.wantVideo, IsVideo(tc.filename))
			assert.Equal(t, tc.wantAudio, IsAudio(tc.filename))
			assert.Equal(t, tc.wantVideo || tc.wantAudio, IsSupportedMedia(tc.filename))
		})
	}
}

func TestCheckFFmpeg(t *testing.T) {
	origLookPath := lookPathFunc
	defer func() { lookPathFunc = origLookPath }()

	t.Run("ffmpeg missing", func(t *testing.T) {
		lookPathFunc = func(file string) (string, error) {
			return "", errors.New("exec: executable file not found in $PATH")
		}

		err := CheckFFmpeg()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ffmpeg is not installed or not found in PATH")
	})

	t.Run("ffmpeg present", func(t *testing.T) {
		lookPathFunc = func(file string) (string, error) {
			return "/usr/local/bin/ffmpeg", nil
		}

		err := CheckFFmpeg()
		assert.NoError(t, err)
	})
}

func TestConvertToStandardWav(t *testing.T) {
	origLookPath := lookPathFunc
	origRunCmd := runCmdFunc
	defer func() {
		lookPathFunc = origLookPath
		runCmdFunc = origRunCmd
	}()

	// Isolate the conversion cache from the real home directory.
	t.Setenv("CYPHR_MEDIA_CACHE_DIR", t.TempDir())

	tempDir := t.TempDir()
	dummyMedia := filepath.Join(tempDir, "video.mp4")
	require.NoError(t, os.WriteFile(dummyMedia, []byte("fake video content"), 0o600))

	t.Run("non-existent media file", func(t *testing.T) {
		_, _, err := ConvertToStandardWav(context.Background(), filepath.Join(tempDir, "non_existent.mp4"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "input media file does not exist")
	})

	t.Run("ffmpeg not available", func(t *testing.T) {
		lookPathFunc = func(file string) (string, error) {
			return "", errors.New("not found")
		}

		_, _, err := ConvertToStandardWav(context.Background(), dummyMedia)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ffmpeg is not installed")
	})

	t.Run("successful conversion", func(t *testing.T) {
		lookPathFunc = func(file string) (string, error) {
			return "/usr/bin/ffmpeg", nil
		}

		var capturedArgs []string
		runCmdFunc = func(ctx context.Context, name string, args ...string) ([]byte, error) {
			capturedArgs = args
			// Mock ffmpeg writing audio to the target output file (last arg)
			outPath := args[len(args)-1]
			if writeErr := os.WriteFile(outPath, []byte("RIFF mock wav bytes"), 0o600); writeErr != nil {
				return nil, writeErr
			}
			return []byte("ffmpeg version mock"), nil
		}

		outPath, cleanup, err := ConvertToStandardAudio(context.Background(), dummyMedia)
		require.NoError(t, err)
		require.NotEmpty(t, outPath)
		assert.True(t, strings.HasSuffix(outPath, ".mp3"))

		// Check arguments passed to ffmpeg
		assert.Contains(t, capturedArgs, "-vn")
		assert.Contains(t, capturedArgs, "-ac")
		assert.Contains(t, capturedArgs, "1")
		assert.Contains(t, capturedArgs, "-ar")
		assert.Contains(t, capturedArgs, "16000")
		assert.Contains(t, capturedArgs, "libmp3lame")
		assert.Contains(t, capturedArgs, "-b:a")
		assert.Contains(t, capturedArgs, "32k")

		// Verify file exists
		_, statErr := os.Stat(outPath)
		assert.NoError(t, statErr)

		// Verify cleanup keeps the cached file on disk
		cleanup()
		_, statAfterErr := os.Stat(outPath)
		assert.NoError(t, statAfterErr)
	})

	t.Run("cache hit skips ffmpeg", func(t *testing.T) {
		lookPathFunc = func(file string) (string, error) {
			return "/usr/bin/ffmpeg", nil
		}

		runCalls := 0
		runCmdFunc = func(ctx context.Context, name string, args ...string) ([]byte, error) {
			runCalls++
			outPath := args[len(args)-1]
			if writeErr := os.WriteFile(outPath, []byte("RIFF mock wav bytes"), 0o600); writeErr != nil {
				return nil, writeErr
			}
			return []byte("ffmpeg version mock"), nil
		}

		// Use unseen content so the first call really converts.
		repeated := filepath.Join(tempDir, "repeated.mp4")
		require.NoError(t, os.WriteFile(repeated, []byte("repeated media content"), 0o600))

		firstPath, _, err := ConvertToStandardAudio(context.Background(), repeated)
		require.NoError(t, err)
		require.Equal(t, 1, runCalls)

		// Second conversion of identical content must not invoke ffmpeg again.
		secondPath, _, err := ConvertToStandardAudio(context.Background(), repeated)
		require.NoError(t, err)
		assert.Equal(t, firstPath, secondPath)
		assert.Equal(t, 1, runCalls)

		// Lookup reports the cached conversion directly.
		cachedPath, ok := LookupCachedConversion(repeated)
		assert.True(t, ok)
		assert.Equal(t, firstPath, cachedPath)
	})

	t.Run("changed source content misses cache", func(t *testing.T) {
		lookPathFunc = func(file string) (string, error) {
			return "/usr/bin/ffmpeg", nil
		}

		runCalls := 0
		runCmdFunc = func(ctx context.Context, name string, args ...string) ([]byte, error) {
			runCalls++
			outPath := args[len(args)-1]
			if writeErr := os.WriteFile(outPath, []byte("RIFF mock wav bytes"), 0o600); writeErr != nil {
				return nil, writeErr
			}
			return []byte("ffmpeg version mock"), nil
		}

		altered := filepath.Join(tempDir, "altered.mp4")
		require.NoError(t, os.WriteFile(altered, []byte("different fake content"), 0o600))

		if _, ok := LookupCachedConversion(altered); ok {
			t.Fatalf("LookupCachedConversion() = hit, want miss for unseen content")
		}
		outPath, _, err := ConvertToStandardAudio(context.Background(), altered)
		require.NoError(t, err)
		require.NotEmpty(t, outPath)
		assert.Equal(t, 1, runCalls)
	})

	t.Run("ffmpeg execution fails", func(t *testing.T) {
		lookPathFunc = func(file string) (string, error) {
			return "/usr/bin/ffmpeg", nil
		}

		runCmdFunc = func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return []byte("corrupt input stream"), errors.New("exit status 1")
		}

		// Use unseen content so the failure path runs instead of a cache hit.
		broken := filepath.Join(tempDir, "broken.mp4")
		require.NoError(t, os.WriteFile(broken, []byte("broken media content"), 0o600))

		outPath, cleanup, err := ConvertToStandardWav(context.Background(), broken)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ffmpeg audio conversion failed")
		assert.Contains(t, err.Error(), "corrupt input stream")
		assert.Empty(t, outPath)
		assert.Nil(t, cleanup)
	})
}
