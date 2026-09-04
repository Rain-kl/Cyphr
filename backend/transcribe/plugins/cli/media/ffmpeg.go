// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package media provides utilities for media type detection and audio extraction via ffmpeg.
package media

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

// ConvertToStandardWav converts any audio or video file into 16kHz mono 16-bit PCM WAV using ffmpeg.
// It returns the path to the temporary WAV file and a cleanup function to delete it when done.
func ConvertToStandardWav(ctx context.Context, inputPath string) (outputPath string, cleanup func(), err error) {
	if _, statErr := os.Stat(inputPath); statErr != nil {
		return "", nil, fmt.Errorf("input media file does not exist: %w", statErr)
	}

	if err := CheckFFmpeg(); err != nil {
		return "", nil, err
	}

	tmpFile, err := os.CreateTemp("", "transcribe_standard_*.wav")
	if err != nil {
		return "", nil, fmt.Errorf("failed to create temporary wav file: %w", err)
	}
	tmpPath := tmpFile.Name()
	_ = tmpFile.Close()

	cleanup = func() {
		_ = os.Remove(tmpPath)
	}

	// Command: ffmpeg -y -i <input> -vn -ac 1 -ar 16000 -c:a pcm_s16le <tmp.wav>
	args := []string{
		"-y",
		"-i", inputPath,
		"-vn",
		"-ac", "1",
		"-ar", "16000",
		"-c:a", "pcm_s16le",
		tmpPath,
	}

	out, err := runCmdFunc(ctx, "ffmpeg", args...)
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("ffmpeg audio conversion failed (%w): %s", err, strings.TrimSpace(string(out)))
	}

	return tmpPath, cleanup, nil
}

// ExtractAudio extracts 16kHz mono audio from a video file into a temporary MP3 file using ffmpeg.
//
// Deprecated: Use ConvertToStandardWav instead for direct 16kHz mono WAV conversion.
func ExtractAudio(ctx context.Context, videoPath string) (outputPath string, cleanup func(), err error) {
	return ConvertToStandardWav(ctx, videoPath)
}
