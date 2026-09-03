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
	defaultPageSize   = 20
	defaultPage       = 1
)

var (
	jobsStatus   string
	jobsPage     int
	jobsPageSize int
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
			resp, err := appClient.ListJobs(cmd.Context(), jobsPage, jobsPageSize, jobsStatus)
			if err != nil {
				return fmt.Errorf("failed to list jobs: %w", err)
			}

			if len(resp.Items) == 0 {
				cmd.Println("No jobs found.")
				return nil
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, defaultTabPadding, ' ', 0)
			_, _ = fmt.Fprintln(w, "JOB ID\tMODEL\tSTATUS\tPROGRESS\tDURATION\tCREATED AT")
			for _, job := range resp.Items {
				durStr := "-"
				if job.Duration > 0 {
					durStr = fmt.Sprintf("%.2fs", job.Duration)
				}
				createdAtStr := job.CreatedAt.Local().Format("2006-01-02 15:04:05")
				progStr := fmt.Sprintf("%d%%", job.Progress)
				_, _ = fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\n",
					job.ID, job.Model, job.Status, progStr, durStr, createdAtStr)
			}
			_ = w.Flush()

			cmd.Printf("\nShowing %d of %d total jobs (page %d)\n", len(resp.Items), resp.Total, resp.Page)
			return nil
		},
	}

	listCmd.Flags().StringVar(&jobsStatus, "status", "", "filter jobs by status (pending, running, completed, failed)")
	listCmd.Flags().IntVar(&jobsPage, "page", defaultPage, "page number")
	listCmd.Flags().IntVar(&jobsPageSize, "page-size", defaultPageSize, "number of jobs per page")

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
	getCmd := &cobra.Command{
		Use:   "get <job_id>",
		Short: "Download completed job results (txt and srt) to local directory",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			jobID, err := strconv.ParseUint(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid job ID '%s': must be a positive integer", args[0])
			}

			job, err := appClient.GetJob(cmd.Context(), jobID)
			if err != nil {
				return fmt.Errorf("failed to get job #%d: %w", jobID, err)
			}

			if job.Status != client.StatusCompleted {
				if job.Status == client.StatusFailed {
					return fmt.Errorf("job #%d failed with error: %s", jobID, job.ErrorMsg)
				}
				return fmt.Errorf("job #%d is not completed yet (current status: %s, progress: %d%%)", jobID, job.Status, job.Progress)
			}

			baseName := ""
			if job.OriginalFileName != "" {
				baseName = strings.TrimSuffix(filepath.Base(job.OriginalFileName), filepath.Ext(job.OriginalFileName))
			}
			if baseName == "" {
				baseName = fmt.Sprintf("job_%d", jobID)
			}

			cmd.Printf("Downloading results for job #%d (%s)...\n", jobID, baseName)
			return saveJobResults(cmd, baseName, getOutputDir, job.ResultText, job.OpenAIResponse, job.Duration)
		},
	}

	getCmd.Flags().StringVarP(&getOutputDir, "output-dir", "d", "", "directory to save output files (default: current directory)")

	return getCmd
}
