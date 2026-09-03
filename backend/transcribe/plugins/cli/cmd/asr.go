// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"Wavelet/transcribe/plugins/cli/client"
	"Wavelet/transcribe/plugins/cli/config"
	"Wavelet/transcribe/plugins/cli/media"
)

var (
	asrModel    string
	asrLanguage string
	asrPrompt   string
	asrFormat   string
)

func newAsrCmd() *cobra.Command {
	asrCmd := &cobra.Command{
		Use:   "asr [flags] <filepath>",
		Short: "Transcribe an audio or video media file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			filePath := args[0]
			fileInfo, err := os.Stat(filePath)
			if err != nil {
				return fmt.Errorf("media file not found: %w", err)
			}
			if fileInfo.IsDir() {
				return fmt.Errorf("input path is a directory, not a file: %s", filePath)
			}

			// Format check and optional extraction
			var uploadPath string
			switch {
			case media.IsVideo(filePath):
				cmd.Printf("Video file detected (%s). Extracting 16kHz mono audio with ffmpeg...\n", filepath.Base(filePath))
				extractedPath, cleanup, err := media.ExtractAudio(cmd.Context(), filePath)
				if err != nil {
					return err
				}
				defer cleanup()
				// Rename the temp output to <name>.mp3 so the uploaded/stored file carries a real audio extension.
				origBase := filepath.Base(filePath)
				prettyName := origBase[:len(origBase)-len(filepath.Ext(origBase))] + ".mp3"
				prettyPath := filepath.Join(filepath.Dir(extractedPath), prettyName)
				if err := os.Rename(extractedPath, prettyPath); err != nil {
					return fmt.Errorf("failed to rename extracted audio: %w", err)
				}
				uploadPath = prettyPath
			case media.IsAudio(filePath):
				uploadPath = filePath
			default:
				return fmt.Errorf("unsupported media file format '%s' (supported audio: mp3, wav, m4a, flac, aac, ogg; video: mp4, mkv, avi, mov, flv, webm)", filepath.Ext(filePath))
			}

			modelName := asrModel
			if modelName == "" && appConfig != nil {
				modelName = appConfig.DefaultModel
			}
			if modelName == "" {
				modelName = config.DefaultModel
			}

			cmd.Printf("Submitting %s for transcription (model: %s)...\n", filepath.Base(filePath), modelName)

			fileHash, fileSize, err := client.SHA256FileHex(uploadPath)
			if err != nil {
				return fmt.Errorf("hash media file: %w", err)
			}

			hashReq := client.HashSubmitRequest{
				FileHash:         fileHash,
				FileSize:         fileSize,
				OriginalFileName: filepath.Base(uploadPath),
				Model:            modelName,
				Language:         asrLanguage,
				Prompt:           asrPrompt,
				ResponseFormat:   asrFormat,
			}

			submitResp, err := appClient.SubmitTranscriptionByHash(cmd.Context(), hashReq)
			if err != nil {
				if !client.IsNotFoundError(err) {
					return fmt.Errorf("submission failed: %w", err)
				}
				submitResp, err = appClient.SubmitTranscription(cmd.Context(), client.TranscriptionRequest{
					FilePath:         uploadPath,
					OriginalFileName: filepath.Base(uploadPath),
					Model:            modelName,
					Language:         asrLanguage,
					Prompt:           asrPrompt,
					ResponseFormat:   asrFormat,
					OnProgress:       printUploadProgress,
				})
				fmt.Fprintln(os.Stderr)
				if err != nil {
					return fmt.Errorf("submission failed: %w", err)
				}
			} else {
				cmd.Printf("File already exists on server, skipped upload (hash match).\n")
			}

			jobID := submitResp.JobID
			cmd.Printf("Job submitted successfully: ID #%d (status: %s)\n", jobID, submitResp.Status)
			cmd.Printf("Streaming logs (Press Ctrl+C to detach without canceling the job)...\n\n")

			// Setup graceful detach on interrupt/termination signal
			streamCtx, stopSignal := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stopSignal()

			var finishEvent *client.FinishEvent
			streamErr := appClient.StreamJobLogs(
				streamCtx,
				jobID,
				func(log client.LogEvent) {
					cmd.Printf("[Progress %3d%%] %s\n", log.Progress, log.Message)
				},
				func(finish client.FinishEvent) {
					finishEvent = &finish
				},
			)

			if errors.Is(streamCtx.Err(), context.Canceled) {
				cmd.Printf("\n[Notice] Detached from job #%d. The job will continue running on the server.\n", jobID)
				cmd.Printf("You can check status or follow logs at any time with: transcribe jobs log %d -f\n", jobID)
				return nil
			}

			if streamErr != nil {
				return fmt.Errorf("streaming job logs failed: %w", streamErr)
			}

			if finishEvent != nil {
				if finishEvent.Status == client.StatusCompleted {
					cmd.Printf("\nTranscription completed in %.2fs:\n", finishEvent.Duration)
					cmd.Println("--------------------------------------------------")
					cmd.Println(finishEvent.ResultText)
					cmd.Println("--------------------------------------------------")
					resultPath := resultFilePath(filePath, asrFormat)
					if werr := os.WriteFile(resultPath, []byte(finishEvent.ResultText), resultFilePerm); werr != nil {
						return fmt.Errorf("write result file: %w", werr)
					}
					cmd.Printf("Result saved to %s\n", resultPath)
					return nil
				}
				return fmt.Errorf("job #%d failed: %s", jobID, finishEvent.ErrorMsg)
			}

			return nil
		},
	}

	asrCmd.Flags().StringVarP(&asrModel, "model", "m", "", "transcription model name (default: from config)")
	asrCmd.Flags().StringVarP(&asrLanguage, "lang", "l", "", "spoken audio language code (e.g. en, zh)")
	asrCmd.Flags().StringVar(&asrPrompt, "prompt", "", "optional prompt or vocabulary guidance")
	asrCmd.Flags().StringVar(&asrFormat, "format", "json", "response format (json, verbose_json, text, srt, vtt)")

	return asrCmd
}

// printUploadProgress renders a single-line upload progress bar on stderr.
// It is invoked from the upload goroutine, so it must stay allocation-light.
func printUploadProgress(written, total int64) {
	pct := 0.0
	if total > 0 {
		pct = float64(written) / float64(total) * 100
	}
	fmt.Fprintf(os.Stderr, "\rUploading... %5.1f%% (%s/%s)", pct, formatBytes(written), formatBytes(total))
}

// formatBytes renders a byte count in human-readable units.
func formatBytes(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}

// resultFilePath derives the result file path in the current directory:
// same base name as the input with an extension matching the response format.
func resultFilePath(inputPath, format string) string {
	ext := ".txt"
	switch format {
	case "srt":
		ext = ".srt"
	case "vtt":
		ext = ".vtt"
	}
	base := filepath.Base(inputPath)
	return strings.TrimSuffix(base, filepath.Ext(base)) + ext
}
