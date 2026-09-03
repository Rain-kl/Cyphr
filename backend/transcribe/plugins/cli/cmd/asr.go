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
				uploadPath = extractedPath
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

			cmd.Printf("Uploading %s and submitting transcription job (model: %s)...\n", filepath.Base(filePath), modelName)

			submitReq := client.TranscriptionRequest{
				FilePath:         uploadPath,
				OriginalFileName: filepath.Base(filePath),
				Model:            modelName,
				Language:         asrLanguage,
				Prompt:           asrPrompt,
				ResponseFormat:   asrFormat,
			}

			submitResp, err := appClient.SubmitTranscription(cmd.Context(), submitReq)
			if err != nil {
				return fmt.Errorf("submission failed: %w", err)
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
