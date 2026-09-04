// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package media provides utilities for media type detection and audio extraction via ffmpeg.
package media

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	// mediaCacheDirEnv overrides the converted-audio cache directory (used by tests).
	mediaCacheDirEnv = "CYPHR_MEDIA_CACHE_DIR"
	// mediaCacheDirPerm keeps the cache directory owner-only, matching resultFilePerm.
	mediaCacheDirPerm  = 0o750
	mediaCacheFilePerm = 0o600
)

var (
	videoExtensions = map[string]struct{}{
		".mp4":  {},
		".mkv":  {},
		".avi":  {},
		".mov":  {},
		".flv":  {},
		".webm": {},
	}

	audioExtensions = map[string]struct{}{
		".mp3":  {},
		".wav":  {},
		".m4a":  {},
		".flac": {},
		".aac":  {},
		".ogg":  {},
	}

	// Pluggable runners to facilitate deterministic unit testing.
	lookPathFunc = exec.LookPath
	runCmdFunc   = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		//nolint:gosec // G204: Subprocess invocation required for ffmpeg audio extraction
		cmd := exec.CommandContext(ctx, name, args...)
		return cmd.CombinedOutput()
	}
)

// SetFFmpegRunnerForTest allows tests in other packages to stub ffmpeg behavior.
func SetFFmpegRunnerForTest(
	lookPath func(string) (string, error),
	runCmd func(context.Context, string, ...string) ([]byte, error),
) func() {
	origLookPath := lookPathFunc
	origRunCmd := runCmdFunc
	if lookPath != nil {
		lookPathFunc = lookPath
	}
	if runCmd != nil {
		runCmdFunc = runCmd
	}
	return func() {
		lookPathFunc = origLookPath
		runCmdFunc = origRunCmd
	}
}

// IsVideo reports whether the path ends with a supported video extension (case-insensitive).
func IsVideo(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	_, ok := videoExtensions[ext]
	return ok
}

// IsAudio reports whether the path ends with a supported audio extension (case-insensitive).
func IsAudio(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	_, ok := audioExtensions[ext]
	return ok
}

// IsSupportedMedia reports whether the path ends with a supported video or audio extension.
func IsSupportedMedia(filename string) bool {
	return IsVideo(filename) || IsAudio(filename)
}

// CheckFFmpeg checks if ffmpeg executable is available in PATH.
func CheckFFmpeg() error {
	_, err := lookPathFunc("ffmpeg")
	if err != nil {
		return fmt.Errorf("ffmpeg is not installed or not found in PATH; required for audio/video conversion")
	}
	return nil
}

// MediaCacheDir resolves the directory holding converted-audio cache files.
// It honors CYPHR_MEDIA_CACHE_DIR and defaults to ~/.cyphr/cache/media.
func MediaCacheDir() (string, error) {
	if env := strings.TrimSpace(os.Getenv(mediaCacheDirEnv)); env != "" {
		return filepath.Clean(env), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	return filepath.Join(home, ".cyphr", "cache", "media"), nil
}

// hashMediaFile returns the hex-encoded SHA-256 digest of a file.
func hashMediaFile(path string) (string, error) {
	f, err := os.Open(path) //nolint:gosec // G304: CLI input path supplied by the user
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// LookupCachedConversion returns the cached conversion output for a source
// file when present. The cache key is the source content hash, so identical
// files share one conversion regardless of filename.
func LookupCachedConversion(inputPath string) (string, bool) {
	sum, err := hashMediaFile(inputPath)
	if err != nil {
		return "", false
	}
	dir, err := MediaCacheDir()
	if err != nil {
		return "", false
	}
	cachedPath := filepath.Join(dir, sum+".mp3")
	info, err := os.Stat(cachedPath)
	if err != nil || info.IsDir() || info.Size() == 0 {
		return "", false
	}
	return cachedPath, true
}

// noopCleanup keeps cached conversions on disk; callers should still defer
// the returned cleanup for API compatibility.
func noopCleanup() {}

// ConvertToStandardAudio converts any audio or video file into a compressed 16kHz mono MP3 (32kbps) using ffmpeg.
// This significantly reduces upload bandwidth and storage overhead (~8x smaller than uncompressed WAV)
// while perfectly matching the 16kHz mono input requirement for speech recognition models (e.g. Qwen3-ASR).
// Converted output is cached under the media cache directory keyed by source
// content hash; repeated conversions of the same file reuse the cache and
// skip ffmpeg. The returned cleanup is a no-op retained for compatibility.
func ConvertToStandardAudio(ctx context.Context, inputPath string) (outputPath string, cleanup func(), err error) {
	if _, statErr := os.Stat(inputPath); statErr != nil {
		return "", nil, fmt.Errorf("input media file does not exist: %w", statErr)
	}

	if cachedPath, ok := LookupCachedConversion(inputPath); ok {
		return cachedPath, noopCleanup, nil
	}

	if err := CheckFFmpeg(); err != nil {
		return "", nil, err
	}

	cacheDir, err := MediaCacheDir()
	if err != nil {
		return "", nil, err
	}
	if err := os.MkdirAll(cacheDir, mediaCacheDirPerm); err != nil {
		return "", nil, fmt.Errorf("failed to create media cache directory: %w", err)
	}

	sum, err := hashMediaFile(inputPath)
	if err != nil {
		return "", nil, fmt.Errorf("failed to hash media file: %w", err)
	}
	cachedPath := filepath.Join(cacheDir, sum+".mp3")
	if info, statErr := os.Stat(cachedPath); statErr == nil && !info.IsDir() && info.Size() > 0 {
		return cachedPath, noopCleanup, nil
	}

	tmpFile, err := os.CreateTemp(cacheDir, "transcribe_audio_*.mp3")
	if err != nil {
		return "", nil, fmt.Errorf("failed to create temporary audio file: %w", err)
	}
	tmpPath := tmpFile.Name()
	_ = tmpFile.Close()

	// Command: ffmpeg -y -i <input> -vn -ac 1 -ar 16000 -c:a libmp3lame -b:a 32k <tmp.mp3>
	args := []string{
		"-y",
		"-i", inputPath,
		"-vn",
		"-ac", "1",
		"-ar", "16000",
		"-c:a", "libmp3lame",
		"-b:a", "32k",
		tmpPath,
	}

	out, err := runCmdFunc(ctx, "ffmpeg", args...)
	if err != nil {
		_ = os.Remove(tmpPath)
		return "", nil, fmt.Errorf("ffmpeg audio conversion failed (%w): %s", err, strings.TrimSpace(string(out)))
	}

	if rerr := os.Rename(tmpPath, cachedPath); rerr != nil {
		// Another process may have populated the cache concurrently.
		_ = os.Remove(tmpPath)
		if info, statErr := os.Stat(cachedPath); statErr == nil && !info.IsDir() && info.Size() > 0 {
			return cachedPath, noopCleanup, nil
		}
		return "", nil, fmt.Errorf("failed to publish converted audio to cache: %w", rerr)
	}
	_ = os.Chmod(cachedPath, mediaCacheFilePerm)

	return cachedPath, noopCleanup, nil
}

// ConvertToStandardWav is kept for compatibility, converting to compressed standard audio.
func ConvertToStandardWav(ctx context.Context, inputPath string) (outputPath string, cleanup func(), err error) {
	return ConvertToStandardAudio(ctx, inputPath)
}

// ExtractAudio extracts audio from a video file into a temporary audio file using ffmpeg.
//
// Deprecated: Use ConvertToStandardAudio instead.
func ExtractAudio(ctx context.Context, videoPath string) (outputPath string, cleanup func(), err error) {
	return ConvertToStandardAudio(ctx, videoPath)
}
