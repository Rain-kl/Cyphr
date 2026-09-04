// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"Wavelet/transcribe/plugins/cli/client"
	"Wavelet/transcribe/plugins/cli/media"
)

const defaultBatchPollInterval = 5 * time.Second

var (
	batchInputDir  string
	batchOutputDir string
	batchDetach    bool
	batchModel     string
	batchLanguage  string
	batchPrompt    string
	batchFormat    string
	batchForceUp   bool
	batchRecursive bool
	batchPollEvery time.Duration
)

// batchItem tracks one input file and its server-side job.
type batchItem struct {
	filePath string
	baseName string
	jobID    uint64
}

// NewBatchCmd creates and returns the batch command.
func NewBatchCmd() *cobra.Command {
	batchCmd := &cobra.Command{
		Use:   "batch",
		Short: "Transcribe all media files in a directory with job-list progress",
		Long: `Submit every supported media file in the input directory as a transcription job.

Batch mode shows a compact job-list progress table instead of per-job logs.
Press Ctrl+C at any time to detach: submitted jobs keep running on the server
and their placeholders allow later retrieval with 'cyphr jobs get all'.
Without -D, the command waits in the foreground until all jobs finish and
then downloads every result.`,
		RunE: runBatch,
	}

	batchCmd.Flags().StringVarP(&batchInputDir, "input", "i", "", "input directory containing media files (required)")
	batchCmd.Flags().StringVarP(&batchOutputDir, "output", "O", "", "directory to save output files (default: current directory)")
	batchCmd.Flags().BoolVarP(&batchDetach, "detach", "D", false, "submit jobs and exit without waiting for completion")
	batchCmd.Flags().StringVarP(&batchModel, "model", "m", "", "transcription model name (default: from config)")
	batchCmd.Flags().StringVarP(&batchLanguage, "lang", "l", "", "spoken audio language code (e.g. en, zh)")
	batchCmd.Flags().StringVar(&batchPrompt, "prompt", "", "optional prompt or vocabulary guidance")
	batchCmd.Flags().StringVar(&batchFormat, "format", "json", "response format (json, verbose_json, text, srt, vtt)")
	batchCmd.Flags().BoolVarP(&batchForceUp, "force-upload", "f", false, "force upload audio files, bypassing hash deduplication")
	batchCmd.Flags().BoolVarP(&batchRecursive, "recursive", "r", false, "also scan subdirectories of the input directory")
	batchCmd.Flags().DurationVar(&batchPollEvery, "poll-interval", defaultBatchPollInterval, "interval between progress refreshes (e.g. 5s)")
	_ = batchCmd.MarkFlagRequired("input")

	return batchCmd
}

func runBatch(cmd *cobra.Command, _ []string) error {
	info, err := os.Stat(batchInputDir)
	if err != nil {
		return fmt.Errorf("input directory not accessible: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("input path is not a directory: %s", batchInputDir)
	}

	outDir, err := resolveBatchOutputDir(batchOutputDir)
	if err != nil {
		return err
	}

	files, skipped, err := scanBatchInputs(batchInputDir, batchRecursive)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no supported media files found in %s", batchInputDir)
	}

	modelName := batchModel
	if modelName == "" {
		modelName = resolveModelName()
	}
	if batchPollEvery <= 0 {
		batchPollEvery = defaultBatchPollInterval
	}

	cmd.Printf("Found %d media file(s) in %s", len(files), batchInputDir)
	if skipped > 0 {
		cmd.Printf(" (%d unsupported file(s) skipped)", skipped)
	}
	cmd.Printf("; output: %s\n\n", outDir)

	var pending []batchItem
	var cached int
	for _, f := range files {
		item, done, submitErr := submitBatchFile(cmd.Context(), cmd, f, modelName, outDir)
		if submitErr != nil {
			cmd.Printf("✗ %s: %v\n", filepath.Base(f), submitErr)
			continue
		}
		if done {
			cached++
			continue
		}
		pending = append(pending, *item)
	}

	cmd.Printf("\nBatch submitted: %d new job(s), %d already completed (cached), %d failed to submit.\n",
		len(pending), cached, len(files)-len(pending)-cached)

	if len(pending) == 0 {
		return nil
	}

	if batchDetach {
		cmd.Printf("Jobs are running in background.\n")
		cmd.Printf("Sync results when completed with: cyphr jobs get all\n")
		return nil
	}

	cmd.Printf("Waiting for completion (Press Ctrl+C to detach; placeholders allow 'cyphr jobs get all' later)...\n")
	if werr := watchBatchProgress(cmd, pending); werr != nil {
		return werr
	}

	return syncBatchResults(cmd, pending, outDir)
}

// resolveBatchOutputDir defaults empty output to the current directory and
// ensures the directory exists.
func resolveBatchOutputDir(outDir string) (string, error) {
	if strings.TrimSpace(outDir) == "" {
		pwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to resolve current directory: %w", err)
		}
		return pwd, nil
	}
	abs, err := filepath.Abs(outDir)
	if err != nil {
		return "", fmt.Errorf("invalid output directory: %w", err)
	}
	if merr := os.MkdirAll(abs, dirPerm); merr != nil {
		return "", fmt.Errorf("failed to create output directory %s: %w", abs, merr)
	}
	return abs, nil
}

// scanBatchInputs returns sorted absolute paths of supported media files.
// Unsupported files are counted as skipped, not as errors.
func scanBatchInputs(inputDir string, recursive bool) (files []string, skipped int, err error) {
	collect := func(path string, d os.DirEntry) {
		if d.IsDir() {
			return
		}
		if !media.IsSupportedMedia(path) {
			skipped++
			return
		}
		if abs, aerr := filepath.Abs(path); aerr == nil {
			files = append(files, abs)
		}
	}

	if recursive {
		walkErr := filepath.WalkDir(inputDir, func(path string, d os.DirEntry, werr error) error {
			if werr != nil {
				skipped++
				return nil //nolint:nilerr // unreadable entries are counted as skipped, not fatal for the batch scan
			}
			collect(path, d)
			return nil
		})
		if walkErr != nil {
			return nil, 0, fmt.Errorf("failed to scan input directory: %w", walkErr)
		}
	} else {
		entries, rerr := os.ReadDir(inputDir)
		if rerr != nil {
			return nil, 0, fmt.Errorf("failed to read input directory: %w", rerr)
		}
		for _, e := range entries {
			collect(filepath.Join(inputDir, e.Name()), e)
		}
	}

	sort.Slice(files, func(i, j int) bool {
		bi, bj := filepath.Base(files[i]), filepath.Base(files[j])
		if bi == bj {
			return files[i] < files[j]
		}
		return bi < bj
	})
	return files, skipped, nil
}

// submitBatchFile uploads one file (with hash fast-path) and records its
// placeholder. done=true means results were already fetched from the server.
func submitBatchFile(ctx context.Context, cmd *cobra.Command, filePath, modelName, outDir string) (*batchItem, bool, error) {
	baseName := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))

	if !batchForceUp {
		if handled, herr := tryBatchFastPath(ctx, cmd, filePath, baseName, modelName, outDir); handled {
			return nil, herr == nil, herr
		}
	}

	uploadPath, cleanup, err := prepareUploadMedia(ctx, cmd, filePath)
	if err != nil {
		return nil, false, err
	}
	if cleanup != nil {
		defer cleanup()
	}

	submitResp, err := submitBatchUpload(ctx, uploadPath, modelName)
	if err != nil {
		return nil, false, err
	}

	jobID := submitResp.JobID
	cmd.Printf("✓ Queued #%d  %s\n", jobID, filepath.Base(filePath))
	_ = SaveJobPlaceholder(&JobPlaceholder{
		JobID:            jobID,
		OriginalFileName: filepath.Base(filePath),
		BaseName:         baseName,
		OutputDir:        outDir,
		Model:            modelName,
		Format:           batchFormat,
		CreatedAt:        time.Now().UTC(),
	})
	return &batchItem{filePath: filePath, baseName: baseName, jobID: jobID}, false, nil
}

// tryBatchFastPath reuses a completed server-side job matched by content hash.
// It reports whether the fast path handled the file.
func tryBatchFastPath(ctx context.Context, cmd *cobra.Command, filePath, baseName, modelName, outDir string) (bool, error) {
	fileHash, fileSize, err := client.SHA256FileHex(filePath)
	if err != nil || fileHash == "" {
		return false, nil //nolint:nilerr
	}

	submitResp, herr := appClient.SubmitTranscriptionByHash(ctx, client.HashSubmitRequest{
		FileHash:         fileHash,
		FileSize:         fileSize,
		OriginalFileName: filepath.Base(filePath),
		Model:            modelName,
		Language:         batchLanguage,
		Prompt:           batchPrompt,
		ResponseFormat:   batchFormat,
	})
	if herr != nil || submitResp == nil {
		return false, nil //nolint:nilerr
	}
	if submitResp.Status != client.StatusCompleted {
		return false, nil
	}

	jobDetail, gerr := appClient.GetJob(ctx, submitResp.JobID)
	if gerr != nil || jobDetail == nil {
		return false, nil //nolint:nilerr
	}
	cmd.Printf("✓ Cached #%d  %s (already completed, downloading)\n", submitResp.JobID, filepath.Base(filePath))
	if serr := saveJobResults(cmd, baseName, outDir, jobDetail.ResultText, jobDetail.OpenAIResponse, jobDetail.Duration); serr != nil {
		return true, serr
	}
	return true, nil
}

// submitBatchUpload submits the converted audio with hash deduplication.
func submitBatchUpload(ctx context.Context, uploadPath, modelName string) (*client.TranscriptionSubmitResponse, error) {
	if !batchForceUp {
		fileHash, fileSize, err := client.SHA256FileHex(uploadPath)
		if err != nil {
			return nil, fmt.Errorf("hash media file: %w", err)
		}
		submitResp, err := appClient.SubmitTranscriptionByHash(ctx, client.HashSubmitRequest{
			FileHash:         fileHash,
			FileSize:         fileSize,
			OriginalFileName: filepath.Base(uploadPath),
			Model:            modelName,
			Language:         batchLanguage,
			Prompt:           batchPrompt,
			ResponseFormat:   batchFormat,
		})
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
		Language:         batchLanguage,
		Prompt:           batchPrompt,
		ResponseFormat:   batchFormat,
	})
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return nil, fmt.Errorf("submission failed: %w", err)
	}
	return submitResp, nil
}

// watchBatchProgress polls job statuses and renders a job-list progress
// table. Ctrl+C detaches gracefully, keeping placeholders for later sync.
func watchBatchProgress(cmd *cobra.Command, pending []batchItem) error {
	watchCtx, stopSignal := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stopSignal()

	last := make(map[uint64]*client.JobInfo, len(pending))
	ticker := time.NewTicker(batchPollEvery)
	defer ticker.Stop()

	tty := isStdoutTerminal()
	var lastSig string
	first := true

	// The full table prints on the first poll and on status transitions only;
	// between changes a one-line counter refreshes in place.
	refresh := func() bool {
		infos := queryBatchStatus(cmd.Context(), pending, last)
		completed, running, failed := countBatchStatus(infos)
		if sig := batchSnapshotSig(infos); first || sig != lastSig {
			cmd.Printf("\n--- Batch progress [%s] %d/%d completed ---\n", time.Now().Format("15:04:05"), completed, len(pending))
			cmd.Print(formatJobsTable(infos, false))
			lastSig = sig
			first = false
		} else {
			printBatchLiveLine(cmd, tty, completed, running, failed, len(pending))
		}
		return running == 0 && completed+failed == len(pending)
	}

	if refresh() {
		return nil
	}
	for {
		select {
		case <-watchCtx.Done():
			cmd.Printf("\n[Notice] Detached from batch. %d job(s) keep running on the server.\n", len(pending))
			cmd.Printf("Sync results later with: cyphr jobs get all\n")
			return nil
		case <-ticker.C:
			if refresh() {
				if tty {
					cmd.Printf("\n")
				}
				return nil
			}
		}
	}
}

// batchSnapshotSig captures job status transitions (not progress percentages)
// so the full table only reprints on meaningful change.
func batchSnapshotSig(infos []client.JobInfo) string {
	var sb strings.Builder
	for _, info := range infos {
		sb.WriteString(strconv.FormatUint(info.ID, 10))
		sb.WriteString(":")
		sb.WriteString(info.Status)
		sb.WriteString(";")
	}
	return sb.String()
}

func isStdoutTerminal() bool {
	fi, err := os.Stdout.Stat()
	return err == nil && (fi.Mode()&os.ModeCharDevice) != 0
}

// printBatchLiveLine refreshes a one-line counter: in place on terminals,
// as a log line otherwise.
func printBatchLiveLine(cmd *cobra.Command, tty bool, completed, running, failed, total int) {
	line := fmt.Sprintf("⏳ [%s] %d/%d completed · %d running · %d failed",
		time.Now().Format("15:04:05"), completed, total, running, failed)
	if !tty {
		cmd.Println(line)
		return
	}
	cmd.Printf("\r%-80s", line)
}

// queryBatchStatus fetches the latest status for each tracked job, retaining
// the last known status when a query fails.
func queryBatchStatus(ctx context.Context, pending []batchItem, last map[uint64]*client.JobInfo) []client.JobInfo {
	infos := make([]client.JobInfo, 0, len(pending))
	for _, item := range pending {
		job, err := appClient.GetJob(ctx, item.jobID)
		if err != nil || job == nil {
			if prev, ok := last[item.jobID]; ok && prev != nil {
				infos = append(infos, *prev)
				continue
			}
			infos = append(infos, client.JobInfo{
				ID:               item.jobID,
				Model:            "",
				Status:           client.StatusPending,
				OriginalFileName: filepath.Base(item.filePath),
			})
			continue
		}
		last[item.jobID] = job
		infos = append(infos, *job)
	}
	return infos
}

// countBatchStatus tallies completed / running / failed jobs.
func countBatchStatus(infos []client.JobInfo) (completed, running, failed int) {
	for _, info := range infos {
		switch info.Status {
		case client.StatusCompleted:
			completed++
		case client.StatusFailed:
			failed++
		default:
			running++
		}
	}
	return completed, running, failed
}

// syncBatchResults downloads results for every tracked job via placeholders.
func syncBatchResults(cmd *cobra.Command, pending []batchItem, outDir string) error {
	cmd.Printf("\nAll jobs reached terminal state. Downloading results...\n\n")

	var completedCount, runningCount, failedCount int
	for _, item := range pending {
		ph, err := GetJobPlaceholder(item.jobID)
		if err != nil || ph == nil {
			completedCount += syncBatchOrphan(cmd, item, outDir)
			continue
		}
		done, running, failed := syncOnePlaceholder(cmd, ph, outDir)
		if done {
			completedCount++
		}
		if running {
			runningCount++
		}
		if failed {
			failedCount++
		}
	}

	cmd.Printf("\nBatch summary: %d synced, %d still running, %d failed\n", completedCount, runningCount, failedCount)
	if runningCount > 0 {
		cmd.Printf("(Hint: Run 'cyphr jobs get all' again later once running jobs finish)\n")
	}
	return nil
}

// syncBatchOrphan handles a tracked job whose placeholder is missing by
// querying the server directly. It returns 1 when results were synced.
func syncBatchOrphan(cmd *cobra.Command, item batchItem, outDir string) int {
	job, err := appClient.GetJob(cmd.Context(), item.jobID)
	if err != nil || job == nil {
		cmd.Printf("✗ Job #%d (%s): failed to query server: %v\n", item.jobID, item.baseName, err)
		return 0
	}
	if job.Status != client.StatusCompleted {
		cmd.Printf("! Job #%d (%s): status %s, placeholder already gone; run 'cyphr jobs get all' later.\n", item.jobID, item.baseName, job.Status)
		return 0
	}
	if serr := saveJobResults(cmd, item.baseName, outDir, job.ResultText, job.OpenAIResponse, job.Duration); serr != nil {
		cmd.Printf("✗ Job #%d (%s): failed to save results: %v\n", item.jobID, item.baseName, serr)
		return 0
	}
	cmd.Printf("✓ Job #%d (%s): results saved.\n", item.jobID, item.baseName)
	return 1
}
