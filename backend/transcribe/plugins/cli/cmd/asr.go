// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/spf13/cobra"

	"Wavelet/transcribe/plugins/cli/client"
	"Wavelet/transcribe/plugins/cli/config"
	"Wavelet/transcribe/plugins/cli/media"
)

const (
	msPerSecond = 1000
	secPerHour  = 3600
	secPerMin   = 60
	dirPerm     = 0o750
)

var (
	asrModel       string
	asrLanguage    string
	asrPrompt      string
	asrFormat      string
	asrOutputDir   string
	asrDetach      bool
	asrForceUpload bool
)

// NewAsrCmd creates and returns the asr command.
func NewAsrCmd() *cobra.Command {
	asrCmd := &cobra.Command{
		Use:   "asr [flags] <filepath>",
		Short: "Transcribe an audio or video media file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			filePath := args[0]
			modelName := resolveModelName()

			// Fast-path: Check if the original file's hash matches an existing completed job on the server
			if !asrForceUpload {
				if handled, err := tryFastPathByHash(cmd, filePath, modelName); handled {
					return err
				}
			}

			uploadPath, cleanup, err := prepareUploadMedia(cmd.Context(), cmd, filePath)
			if err != nil {
				return err
			}
			if cleanup != nil {
				defer cleanup()
			}

			cmd.Printf("Submitting %s for transcription (model: %s)...\n", filepath.Base(filePath), modelName)

			submitResp, err := submitMediaJob(cmd.Context(), uploadPath, modelName)
			if err != nil {
				return err
			}

			jobID := submitResp.JobID
			cmd.Printf("Job submitted successfully: ID #%d (status: %s)\n", jobID, submitResp.Status)

			// Record job placeholder under ~/.cyphr/jobs for resilient retrieval
			baseName := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
			absOutputDir := asrOutputDir
			if absOutputDir == "" {
				if pwd, perr := os.Getwd(); perr == nil {
					absOutputDir = pwd
				}
			} else if abs, aerr := filepath.Abs(absOutputDir); aerr == nil {
				absOutputDir = abs
			}
			_ = SaveJobPlaceholder(&JobPlaceholder{
				JobID:            jobID,
				OriginalFileName: filepath.Base(filePath),
				BaseName:         baseName,
				OutputDir:        absOutputDir,
				Model:            modelName,
				Format:           asrFormat,
				CreatedAt:        time.Now().UTC(),
			})

			if submitResp.Status == client.StatusCompleted {
				cmd.Printf("Transcription already completed on server! Fetching results directly...\n")
				jobDetail, gerr := appClient.GetJob(cmd.Context(), jobID)
				if gerr == nil && jobDetail != nil {
					finish := client.FinishEvent{
						Status:         jobDetail.Status,
						Duration:       jobDetail.Duration,
						ResultText:     jobDetail.ResultText,
						OpenAIResponse: jobDetail.OpenAIResponse,
					}
					return handleCompletedJob(cmd.Context(), cmd, filePath, jobID, &finish)
				}
			}

			if asrDetach {
				cmd.Printf("Job is running in background.\n")
				cmd.Printf("You can follow live logs with: cyphr jobs log %d -f\n", jobID)
				cmd.Printf("Or sync results when completed with: cyphr jobs get %d (or 'cyphr jobs get all')\n", jobID)
				return nil
			}

			cmd.Printf("Streaming logs (Press Ctrl+C to detach without canceling the job)...\n\n")

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
				cmd.Printf("You can check status or follow logs at any time with: cyphr jobs log %d -f\n", jobID)
				cmd.Printf("Or sync all pending job results with: cyphr jobs get all\n")
				return nil
			}
			if streamErr != nil {
				return fmt.Errorf("streaming job logs failed: %w", streamErr)
			}
			if finishEvent == nil {
				return nil
			}
			if finishEvent.Status != client.StatusCompleted {
				return fmt.Errorf("job #%d failed: %s", jobID, finishEvent.ErrorMsg)
			}

			return handleCompletedJob(cmd.Context(), cmd, filePath, jobID, finishEvent)
		},
	}

	asrCmd.Flags().StringVarP(&asrModel, "model", "m", "", "transcription model name (default: from config)")
	asrCmd.Flags().StringVarP(&asrLanguage, "lang", "l", "", "spoken audio language code (e.g. en, zh)")
	asrCmd.Flags().StringVar(&asrPrompt, "prompt", "", "optional prompt or vocabulary guidance")
	asrCmd.Flags().StringVar(&asrFormat, "format", "json", "response format (json, verbose_json, text, srt, vtt)")
	asrCmd.Flags().StringVarP(&asrOutputDir, "output", "o", "", "directory to save output files (default: current directory)")
	asrCmd.Flags().BoolVarP(&asrDetach, "detach", "d", false, "run job in background without blocking terminal")
	asrCmd.Flags().BoolVarP(&asrForceUpload, "force-upload", "f", false, "force upload audio file, bypassing hash deduplication")

	asrCmd.AddCommand(NewBatchCmd())

	return asrCmd
}

func prepareUploadMedia(ctx context.Context, cmd *cobra.Command, filePath string) (string, func(), error) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return "", nil, fmt.Errorf("media file not found: %w", err)
	}
	if fileInfo.IsDir() {
		return "", nil, fmt.Errorf("input path is a directory, not a file: %s", filePath)
	}

	if !media.IsSupportedMedia(filePath) {
		return "", nil, fmt.Errorf("unsupported media file format '%s' (supported audio: mp3, wav, m4a, flac, aac, ogg; video: mp4, mkv, avi, mov, flv, webm)", filepath.Ext(filePath))
	}

	if cachedPath, ok := media.LookupCachedConversion(filePath); ok {
		cmd.Printf("Reusing cached audio conversion (%s), skipping ffmpeg...\n", filepath.Base(cachedPath))
	} else {
		cmd.Printf("Pre-processing media file with ffmpeg (compressing to 16kHz mono MP3)...\n")
	}
	convertedPath, _, convErr := media.ConvertToStandardAudio(ctx, filePath)
	if convErr != nil {
		return "", nil, convErr
	}

	// Stage a pretty-named temporary copy for upload; the cached conversion stays intact.
	origBase := filepath.Base(filePath)
	prettyName := origBase[:len(origBase)-len(filepath.Ext(origBase))] + ".mp3"
	stageDir, err := os.MkdirTemp("", "cyphr_upload_*")
	if err != nil {
		return "", nil, fmt.Errorf("failed to create upload staging directory: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(stageDir) }
	prettyPath := filepath.Join(stageDir, prettyName)
	if copyErr := copyFile(convertedPath, prettyPath); copyErr != nil {
		cleanup()
		return "", nil, copyErr
	}
	return prettyPath, cleanup, nil
}

// copyFile copies src to dst with owner-only permissions.
func copyFile(src, dst string) error {
	in, err := os.Open(filepath.Clean(src)) //nolint:gosec // G304: Local CLI staging files
	if err != nil {
		return fmt.Errorf("failed to open converted audio: %w", err)
	}
	defer func() { _ = in.Close() }()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, resultFilePerm) //nolint:gosec // G304: dst is built internally under os.MkdirTemp staging dir, not raw user input
	if err != nil {
		return fmt.Errorf("failed to stage upload file: %w", err)
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return fmt.Errorf("failed to stage upload file: %w", err)
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("failed to stage upload file: %w", err)
	}
	return nil
}

func resolveModelName() string {
	if asrModel != "" {
		return asrModel
	}
	if appConfig != nil && appConfig.DefaultModel != "" {
		return appConfig.DefaultModel
	}
	return config.DefaultModel
}

func submitMediaJob(ctx context.Context, uploadPath, modelName string) (*client.TranscriptionSubmitResponse, error) {
	if !asrForceUpload {
		fileHash, fileSize, err := client.SHA256FileHex(uploadPath)
		if err != nil {
			return nil, fmt.Errorf("hash media file: %w", err)
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

		submitResp, err := appClient.SubmitTranscriptionByHash(ctx, hashReq)
		if err == nil {
			return submitResp, nil
		}
		if !client.IsNotFoundError(err) {
			return nil, fmt.Errorf("submission failed: %w", err)
		}
	}

	submitResp, err := appClient.SubmitTranscription(ctx, client.TranscriptionRequest{
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
		return nil, fmt.Errorf("submission failed: %w", err)
	}
	return submitResp, nil
}

func handleCompletedJob(ctx context.Context, cmd *cobra.Command, filePath string, jobID uint64, finishEvent *client.FinishEvent) error {
	result := finishEvent.ResultText
	totalChars := utf8.RuneCountInString(result)

	preview := result
	if totalChars > 100 { //nolint:mnd
		runes := []rune(result)
		preview = string(runes[:100]) + "..."
	}

	cmd.Printf("\nTranscription completed in %.2fs\n", finishEvent.Duration)
	cmd.Printf("Total characters: %d\n", totalChars)
	cmd.Println("--------------------------------------------------")
	cmd.Println(preview)
	cmd.Println("--------------------------------------------------")

	baseName := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
	openAIResp := finishEvent.OpenAIResponse
	if openAIResp == nil {
		if jobDetail, gerr := appClient.GetJob(ctx, jobID); gerr == nil && jobDetail != nil {
			openAIResp = jobDetail.OpenAIResponse
		}
	}

	if err := saveJobResults(cmd, baseName, asrOutputDir, finishEvent.ResultText, openAIResp, finishEvent.Duration); err != nil {
		return err
	}
	_ = RemoveJobPlaceholder(jobID)
	return nil
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

func tryFastPathByHash(cmd *cobra.Command, filePath, modelName string) (bool, error) {
	fileHash, fileSize, err := client.SHA256FileHex(filePath)
	if err != nil || fileHash == "" {
		// Cannot compute file hash; proceed to standard upload path
		return false, nil //nolint:nilerr
	}

	hashReq := client.HashSubmitRequest{
		FileHash:         fileHash,
		FileSize:         fileSize,
		OriginalFileName: filepath.Base(filePath),
		Model:            modelName,
		Language:         asrLanguage,
		Prompt:           asrPrompt,
		ResponseFormat:   asrFormat,
	}

	submitResp, herr := appClient.SubmitTranscriptionByHash(cmd.Context(), hashReq)
	if herr != nil || submitResp == nil {
		// Server does not have matching hash or hash check failed; proceed to upload
		return false, nil //nolint:nilerr
	}

	jobID := submitResp.JobID
	cmd.Printf("Matched existing media: Job ID #%d (status: %s)\n", jobID, submitResp.Status)
	if submitResp.Status != client.StatusCompleted {
		return false, nil
	}

	cmd.Printf("Transcription already completed on server! Fetching results directly...\n")
	jobDetail, gerr := appClient.GetJob(cmd.Context(), jobID)
	if gerr != nil || jobDetail == nil {
		// Could not fetch completed job detail; fallback to standard upload
		return false, nil //nolint:nilerr
	}

	finish := client.FinishEvent{
		Status:         jobDetail.Status,
		Duration:       jobDetail.Duration,
		ResultText:     jobDetail.ResultText,
		OpenAIResponse: jobDetail.OpenAIResponse,
	}
	return true, handleCompletedJob(cmd.Context(), cmd, filePath, jobID, &finish)
}
