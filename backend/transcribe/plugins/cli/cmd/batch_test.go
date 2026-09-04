// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"Wavelet/transcribe/plugins/cli/client"
)

func writeBatchTestFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("failed to create test dir: %v", err)
	}
	if err := os.WriteFile(path, []byte("fake media"), 0o644); err != nil {
		t.Fatalf("failed to write test file %s: %v", path, err)
	}
}

func baseNames(paths []string) []string {
	names := make([]string, len(paths))
	for i, p := range paths {
		names[i] = filepath.Base(p)
	}
	return names
}

func TestAsrBatchSubcommand(t *testing.T) {
	root := NewRootCmd()
	var asrCmd *cobra.Command
	for _, c := range root.Commands() {
		if c.Name() == "asr" {
			asrCmd = c
			break
		}
	}
	if asrCmd == nil {
		t.Fatalf("root commands = %v, want an 'asr' command", commandNames(root))
	}
	found := false
	for _, c := range asrCmd.Commands() {
		if c.Name() == "batch" {
			found = true
		}
	}
	if !found {
		t.Errorf("asr subcommands = %v, want a 'batch' subcommand", commandNames(asrCmd))
	}
}

func commandNames(cmd *cobra.Command) []string {
	names := make([]string, 0)
	for _, c := range cmd.Commands() {
		names = append(names, c.Name())
	}
	return names
}

func TestScanBatchInputs(t *testing.T) {
	dir := t.TempDir()
	writeBatchTestFile(t, filepath.Join(dir, "b.wav"))
	writeBatchTestFile(t, filepath.Join(dir, "a.mp3"))
	writeBatchTestFile(t, filepath.Join(dir, "notes.txt"))
	writeBatchTestFile(t, filepath.Join(dir, "sub", "c.mp4"))

	t.Run("non-recursive scan filters and sorts", func(t *testing.T) {
		files, skipped, err := scanBatchInputs(dir, false)
		if err != nil {
			t.Fatalf("scanBatchInputs() error = %v", err)
		}
		if skipped != 1 {
			t.Errorf("scanBatchInputs() skipped = %d, want %d (notes.txt)", skipped, 1)
		}
		got, want := baseNames(files), []string{"a.mp3", "b.wav"}
		if len(got) != len(want) {
			t.Fatalf("scanBatchInputs() files = %v, want %v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("scanBatchInputs() files[%d] = %q, want %q", i, got[i], want[i])
			}
		}
	})

	t.Run("recursive scan includes subdirectories", func(t *testing.T) {
		files, skipped, err := scanBatchInputs(dir, true)
		if err != nil {
			t.Fatalf("scanBatchInputs() error = %v", err)
		}
		if skipped != 1 {
			t.Errorf("scanBatchInputs() skipped = %d, want %d", skipped, 1)
		}
		got, want := baseNames(files), []string{"a.mp3", "b.wav", "c.mp4"}
		if len(got) != len(want) {
			t.Fatalf("scanBatchInputs() files = %v, want %v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("scanBatchInputs() files[%d] = %q, want %q", i, got[i], want[i])
			}
		}
	})

	t.Run("missing directory returns error", func(t *testing.T) {
		if _, _, err := scanBatchInputs(filepath.Join(dir, "nope"), false); err == nil {
			t.Errorf("scanBatchInputs(missing) error = nil, want non-nil")
		}
	})
}

func TestBatchSnapshotSig(t *testing.T) {
	infos := []client.JobInfo{
		{ID: 1, Status: client.StatusRunning, Progress: 10},
		{ID: 2, Status: client.StatusPending, Progress: 0},
	}
	base := batchSnapshotSig(infos)

	t.Run("progress-only change keeps signature", func(t *testing.T) {
		infos[0].Progress = 74
		if got := batchSnapshotSig(infos); got != base {
			t.Errorf("batchSnapshotSig() changed on progress update: %q vs %q", got, base)
		}
	})

	t.Run("status change flips signature", func(t *testing.T) {
		infos[0].Status = client.StatusCompleted
		if got := batchSnapshotSig(infos); got == base {
			t.Errorf("batchSnapshotSig() unchanged after status transition: %q", got)
		}
	})
}

func TestFormatJobsTableMatchesRender(t *testing.T) {
	items := []client.JobInfo{
		{ID: 7, OriginalFileName: "a.mp3", Model: "m", Status: client.StatusRunning, Progress: 42, Duration: 1.5, CreatedAt: time.Date(2026, 9, 4, 20, 11, 26, 0, time.UTC)},
		{ID: 8, OriginalFileName: "", Model: "m", Status: client.StatusPending, CreatedAt: time.Date(2026, 9, 4, 20, 11, 31, 0, time.UTC)},
	}
	renderCmd := &cobra.Command{}
	var buf strings.Builder
	renderCmd.SetOut(&buf)
	renderJobsTable(renderCmd, items, false)
	if got, want := buf.String(), formatJobsTable(items, false); got != want {
		t.Errorf("renderJobsTable output mismatch:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestCountBatchStatus(t *testing.T) {
	infos := []client.JobInfo{
		{ID: 1, Status: client.StatusCompleted},
		{ID: 2, Status: client.StatusRunning},
		{ID: 3, Status: client.StatusPending},
		{ID: 4, Status: client.StatusFailed},
		{ID: 5, Status: "unknown-status"},
	}
	completed, running, failed := countBatchStatus(infos)
	if completed != 1 {
		t.Errorf("countBatchStatus() completed = %d, want %d", completed, 1)
	}
	if running != 3 {
		t.Errorf("countBatchStatus() running = %d, want %d (running+pending+unknown)", running, 3)
	}
	if failed != 1 {
		t.Errorf("countBatchStatus() failed = %d, want %d", failed, 1)
	}
}

func TestResolveBatchOutputDir(t *testing.T) {
	t.Run("empty output resolves to working directory", func(t *testing.T) {
		got, err := resolveBatchOutputDir("")
		if err != nil {
			t.Fatalf("resolveBatchOutputDir() error = %v", err)
		}
		want, _ := os.Getwd()
		if got != want {
			t.Errorf("resolveBatchOutputDir() = %q, want %q (cwd)", got, want)
		}
	})

	t.Run("custom output directory is created", func(t *testing.T) {
		target := filepath.Join(t.TempDir(), "nested", "out")
		got, err := resolveBatchOutputDir(target)
		if err != nil {
			t.Fatalf("resolveBatchOutputDir() error = %v", err)
		}
		if got != target {
			t.Errorf("resolveBatchOutputDir() = %q, want %q", got, target)
		}
		if info, serr := os.Stat(target); serr != nil || !info.IsDir() {
			t.Errorf("resolveBatchOutputDir() did not create directory %q", target)
		}
	})
}
