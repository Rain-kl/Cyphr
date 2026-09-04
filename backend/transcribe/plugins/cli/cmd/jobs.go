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
	"strconv"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"Wavelet/transcribe/plugins/cli/client"
)

const (
	defaultTabPadding = 3
	defaultPageSize   = 10
	allJobsPageSize   = 10000
	defaultPage       = 1
)

var (
	jobsStatus   string
	jobsPage     int
	jobsPageSize int
	jobsAll      bool
	followLogs   bool
)

// NewJobsCmd creates and returns the jobs command.
func NewJobsCmd() *cobra.Command {
	jobsCmd := &cobra.Command{
		Use:   "jobs",
		Short: "Manage and inspect transcription jobs",
	}

	jobsCmd.AddCommand(newJobsListCmd())
	jobsCmd.AddCommand(newJobsLogCmd())
	jobsCmd.AddCommand(newJobsGetCmd())

	return jobsCmd
}

func newJobsListCmd() *cobra.Command {
	listCmd := &cobra.Command{
		Use:   "ls",
		Short: "List transcription jobs",
		RunE: func(cmd *cobra.Command, _ []string) error {
			fetchPage := jobsPage
			fetchPageSize := jobsPageSize
			if jobsAll {
				fetchPage = 1
				if !cmd.Flags().Changed("page-size") {
					fetchPageSize = allJobsPageSize
				}
			}

			resp, err := appClient.ListJobs(cmd.Context(), fetchPage, fetchPageSize, jobsStatus)
			if err != nil {
				return fmt.Errorf("failed to list jobs: %w", err)
			}

			if len(resp.Items) == 0 {
				cmd.Println("No jobs found.")
				return nil
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, defaultTabPadding, ' ', 0)
			_, _ = fmt.Fprintln(w, "JOB ID\tFILE\tMODEL\tSTATUS\tPROGRESS\tDURATION\tCREATED AT")
			for _, job := range resp.Items {
				fileName := job.OriginalFileName
				if fileName == "" {
					fileName = "-"
				}
				durStr := "-"
				if job.Duration > 0 {
					durStr = fmt.Sprintf("%.2fs", job.Duration)
				}
				createdAtStr := job.CreatedAt.Local().Format("2006-01-02 15:04:05")
				progStr := fmt.Sprintf("%d%%", job.Progress)
				_, _ = fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\t%s\n",
					job.ID, fileName, job.Model, job.Status, progStr, durStr, createdAtStr)
			}
			_ = w.Flush()

			if jobsAll {
				cmd.Printf("\nShowing all %d jobs\n", len(resp.Items))
			} else {
				cmd.Printf("\nShowing %d of %d total jobs (page %d)\n", len(resp.Items), resp.Total, resp.Page)
			}
			return nil
		},
	}

	listCmd.Flags().StringVar(&jobsStatus, "status", "", "filter jobs by status (pending, running, completed, failed)")
	listCmd.Flags().IntVar(&jobsPage, "page", defaultPage, "page number")
	listCmd.Flags().IntVar(&jobsPageSize, "page-size", defaultPageSize, "number of jobs per page (default: 10)")
	listCmd.Flags().BoolVarP(&jobsAll, "all", "a", false, "show all jobs without pagination limit")

	return listCmd
}

func runFollowLog(cmd *cobra.Command, jobID uint64) error {
	streamCtx, stopSignal := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stopSignal()

	var finishEv *client.FinishEvent
	streamErr := appClient.StreamJobLogs(
		streamCtx,
		jobID,
		func(log client.LogEvent) {
			cmd.Printf("[Progress %3d%%] %s\n", log.Progress, log.Message)
		},
		func(finish client.FinishEvent) {
			finishEv = &finish
		},
	)

	if errors.Is(streamCtx.Err(), context.Canceled) {
		cmd.Printf("\n[Notice] Detached from job #%d logs.\n", jobID)
		return nil
	}

	if streamErr != nil {
		return fmt.Errorf("error streaming job logs: %w", streamErr)
	}

	if finishEv == nil {
		return nil
	}

	if finishEv.Status == client.StatusCompleted {
		cmd.Printf("\nJob #%d completed in %.2fs:\n", jobID, finishEv.Duration)
		if finishEv.ResultText != "" {
			cmd.Println("--------------------------------------------------")
			cmd.Println(finishEv.ResultText)
			cmd.Println("--------------------------------------------------")
		}
		return nil
	}

	cmd.Printf("\nJob #%d failed: %s\n", jobID, finishEv.ErrorMsg)
	return nil
}

func runStaticLog(cmd *cobra.Command, job *client.JobInfo) error {
	jobID := job.ID

	if job.Status == client.StatusCompleted || job.Status == client.StatusFailed {
		var finishEv *client.FinishEvent
		_ = appClient.StreamJobLogs(
			cmd.Context(),
			jobID,
			func(log client.LogEvent) {
				cmd.Printf("[Progress %3d%%] %s\n", log.Progress, log.Message)
			},
			func(finish client.FinishEvent) {
				finishEv = &finish
			},
		)
		if finishEv != nil && finishEv.ResultText != "" {
			cmd.Println("--------------------------------------------------")
			cmd.Println(finishEv.ResultText)
			cmd.Println("--------------------------------------------------")
		}
		return nil
	}

	drainCtx, cancel := context.WithTimeout(cmd.Context(), 1*time.Second)
	defer cancel()

	_ = appClient.StreamJobLogs(
		drainCtx,
		jobID,
		func(log client.LogEvent) {
			cmd.Printf("[Progress %3d%%] %s\n", log.Progress, log.Message)
		},
		nil,
	)

	cmd.Printf("\n(Job #%d is currently %s [progress: %d%%]. Use -f / --follow to stream live logs.)\n", jobID, job.Status, job.Progress)
	return nil
}

func newJobsLogCmd() *cobra.Command {
	logCmd := &cobra.Command{
		Use:   "log [flags] <job_id>",
		Short: "View logs for a transcription job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			jobID, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid job ID '%s': must be a positive integer", args[0])
			}

			if followLogs {
				return runFollowLog(cmd, jobID)
			}

			job, err := appClient.GetJob(cmd.Context(), jobID)
			if err != nil {
				return fmt.Errorf("failed to get job #%d: %w", jobID, err)
			}

			return runStaticLog(cmd, job)
		},
	}

	logCmd.Flags().BoolVarP(&followLogs, "follow", "f", false, "follow live log stream until completion")

	return logCmd
}

func newJobsGetCmd() *cobra.Command {
	var getOutputDir string
	var getAll bool
	getCmd := &cobra.Command{
		Use:   "get [flags] <job_id | all>",
		Short: "Download completed job results (txt and srt) to local directory",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if getAll || (len(args) == 1 && strings.EqualFold(args[0], "all")) {
				return runJobsGetAll(cmd, getOutputDir)
			}

			if len(args) == 0 {
				return fmt.Errorf("requires either a job ID or 'all' (e.g. 'cyphr jobs get all')")
			}

			jobID, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid job ID '%s': must be a positive integer or 'all'", args[0])
			}

			return runJobsGetOne(cmd, jobID, getOutputDir)
		},
	}

	getCmd.Flags().StringVarP(&getOutputDir, "output-dir", "d", "", "directory to save output files (default: current directory or placeholder output dir)")
	getCmd.Flags().BoolVarP(&getAll, "all", "a", false, "sync and download all pending jobs from local placeholders")

	return getCmd
}

func runJobsGetOne(cmd *cobra.Command, jobID uint64, overrideOutDir string) error {
	job, err := appClient.GetJob(cmd.Context(), jobID)
	if err != nil {
		return fmt.Errorf("failed to get job #%d: %w", jobID, err)
	}

	if job.Status != client.StatusCompleted {
		if job.Status == client.StatusFailed {
			_ = RemoveJobPlaceholder(jobID)
			return fmt.Errorf("job #%d failed with error: %s", jobID, job.ErrorMsg)
		}
		return fmt.Errorf("job #%d is not completed yet (current status: %s, progress: %d%%)", jobID, job.Status, job.Progress)
	}

	ph, _ := GetJobPlaceholder(jobID)

	baseName := ""
	if job.OriginalFileName != "" {
		baseName = strings.TrimSuffix(filepath.Base(job.OriginalFileName), filepath.Ext(job.OriginalFileName))
	} else if ph != nil && ph.BaseName != "" {
		baseName = ph.BaseName
	}
	if baseName == "" {
		baseName = fmt.Sprintf("job_%d", jobID)
	}

	cmd.Printf("Downloading results for job #%d (%s)...\n", jobID, baseName)
	if err := saveJobResults(cmd, baseName, overrideOutDir, job.ResultText, job.OpenAIResponse, job.Duration); err != nil {
		return err
	}

	_ = RemoveJobPlaceholder(jobID)
	return nil
}

func runJobsGetAll(cmd *cobra.Command, overrideOutDir string) error {
	placeholders, err := ListJobPlaceholders()
	if err != nil {
		return fmt.Errorf("failed to list job placeholders: %w", err)
	}

	if len(placeholders) == 0 {
		cmd.Println("No pending job placeholders found in ~/.cyphr/jobs.")
		return nil
	}

	cmd.Printf("Found %d pending job placeholder(s). Checking status on server...\n\n", len(placeholders))

	var completedCount, runningCount, failedCount int
	for _, ph := range placeholders {
		done, running, failed := syncOnePlaceholder(cmd, ph, overrideOutDir)
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

	cmd.Printf("Summary: %d synced, %d still running, %d failed/removed\n", completedCount, runningCount, failedCount)
	if runningCount > 0 {
		cmd.Printf("(Hint: Run 'cyphr jobs get all' again later once running jobs finish)\n")
	}

	return nil
}

func syncOnePlaceholder(cmd *cobra.Command, ph *JobPlaceholder, overrideOutDir string) (bool, bool, bool) {
	jobID := ph.JobID
	baseName := ph.BaseName
	if baseName == "" {
		baseName = fmt.Sprintf("job_%d", jobID)
	}

	job, err := appClient.GetJob(cmd.Context(), jobID)
	if err != nil {
		if client.IsNotFoundError(err) {
			cmd.Printf("✗ Job #%d (%s): not found on server (deleted or expired). Removing placeholder.\n", jobID, baseName)
			_ = RemoveJobPlaceholder(jobID)
			return false, false, true
		}
		cmd.Printf("! Job #%d (%s): failed to query server: %v (placeholder retained)\n", jobID, baseName, err)
		return false, false, false
	}

	switch job.Status {
	case client.StatusCompleted:
		outDir := overrideOutDir
		if outDir == "" && ph.OutputDir != "" {
			outDir = ph.OutputDir
		}
		cmd.Printf("▶ Job #%d (%s): Completed (%.2fs)! Downloading results...\n", jobID, baseName, job.Duration)
		if saveErr := saveJobResults(cmd, baseName, outDir, job.ResultText, job.OpenAIResponse, job.Duration); saveErr != nil {
			cmd.Printf("  ✗ Failed to save results: %v (placeholder retained)\n\n", saveErr)
			return false, false, false
		}
		_ = RemoveJobPlaceholder(jobID)
		cmd.Printf("  ✓ Results saved and placeholder removed.\n\n")
		return true, false, false

	case client.StatusRunning, client.StatusPending:
		cmd.Printf("⏳ Job #%d (%s): currently %s (%d%%). Placeholder retained.\n", jobID, baseName, job.Status, job.Progress)
		return false, true, false

	case client.StatusFailed:
		cmd.Printf("✗ Job #%d (%s): failed on server (%s). Removing placeholder.\n", jobID, baseName, job.ErrorMsg)
		_ = RemoveJobPlaceholder(jobID)
		return false, false, true

	default:
		cmd.Printf("? Job #%d (%s): unknown status '%s'. Placeholder retained.\n", jobID, baseName, job.Status)
		return false, false, false
	}
}
