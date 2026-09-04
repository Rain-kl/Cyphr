// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"Wavelet/transcribe/plugins/cli/media"
)

var (
	purgeDryRun bool
	purgeForce  bool
	purgeYes    bool
)

// NewPurgeCmd creates and returns the purge command.
func NewPurgeCmd() *cobra.Command {
	purgeCmd := &cobra.Command{
		Use:   "purge",
		Short: "Clear local media conversion cache",
		Long: `Delete cached audio conversions under the media cache directory
(default ~/.cyphr/cache/media, overridable via CYPHR_MEDIA_CACHE_DIR).

By default the command lists the file count and total size, then asks
for confirmation before deleting. Job placeholders (~/.cyphr/jobs) and
the configuration file are never touched.`,
		RunE: runPurge,
	}

	purgeCmd.Flags().BoolVar(&purgeDryRun, "dry-run", false, "show what would be deleted without deleting anything")
	purgeCmd.Flags().BoolVarP(&purgeForce, "force", "f", false, "delete without asking for confirmation")
	purgeCmd.Flags().BoolVarP(&purgeYes, "yes", "y", false, "delete without asking for confirmation")

	return purgeCmd
}

func runPurge(cmd *cobra.Command, _ []string) error {
	cacheDir, err := media.MediaCacheDir()
	if err != nil {
		return err
	}

	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			cmd.Printf("No media cache found in %s (nothing to purge).\n", cacheDir)
			return nil
		}
		return fmt.Errorf("failed to read media cache directory %s: %w", cacheDir, err)
	}

	var files []string
	var totalSize int64
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		fullPath := filepath.Join(cacheDir, entry.Name())
		info, statErr := entry.Info()
		if statErr != nil {
			continue
		}
		files = append(files, fullPath)
		totalSize += info.Size()
	}

	if len(files) == 0 {
		cmd.Printf("Media cache is already empty (%s).\n", cacheDir)
		return nil
	}

	cmd.Printf("Media cache: %d file(s), %s in %s\n", len(files), formatBytes(totalSize), cacheDir)

	if purgeDryRun {
		cmd.Println("(dry-run, nothing deleted)")
		return nil
	}

	if !purgeForce && !purgeYes {
		reader := bufio.NewReader(cmd.InOrStdin())
		_, _ = fmt.Fprint(cmd.OutOrStdout(), "Delete these files? [y/N]: ")
		line, readErr := reader.ReadString('\n')
		if readErr != nil && len(line) == 0 {
			cmd.Println("Aborted.")
			return nil
		}
		switch strings.ToLower(strings.TrimSpace(line)) {
		case "y", "yes":
		default:
			cmd.Println("Aborted.")
			return nil
		}
	}

	var deleted int
	var failed int
	for _, f := range files {
		if rmErr := os.Remove(f); rmErr != nil {
			failed++
			continue
		}
		deleted++
	}

	if failed > 0 {
		return fmt.Errorf("purge incomplete: deleted %d file(s), failed to delete %d file(s) in %s", deleted, failed, cacheDir)
	}

	cmd.Printf("Purged %d file(s), freed %s.\n", deleted, formatBytes(totalSize))
	return nil
}
