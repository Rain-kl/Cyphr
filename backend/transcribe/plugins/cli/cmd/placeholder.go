// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	jobsDirPerm  = 0o700
	jobsFilePerm = 0o600
)

// JobPlaceholder stores metadata for an ongoing transcription job in ~/.cyphr/jobs.
type JobPlaceholder struct {
	JobID            uint64    `json:"job_id"`
	OriginalFileName string    `json:"original_file_name"`
	BaseName         string    `json:"base_name"`
	OutputDir        string    `json:"output_dir"`
	Model            string    `json:"model"`
	Format           string    `json:"format"`
	CreatedAt        time.Time `json:"created_at"`
}

// GetJobsDir resolves the directory used for job placeholders (~/.cyphr/jobs).
func GetJobsDir() (string, error) {
	if env := strings.TrimSpace(os.Getenv("CYPHR_JOBS_DIR")); env != "" {
		return filepath.Clean(env), nil
	}
	if cfgFile != "" {
		return filepath.Join(filepath.Dir(filepath.Clean(cfgFile)), "jobs"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	return filepath.Join(home, ".cyphr", "jobs"), nil
}

// SaveJobPlaceholder writes a placeholder file into ~/.cyphr/jobs/<job_id>.json.
func SaveJobPlaceholder(ph *JobPlaceholder) error {
	if ph == nil || ph.JobID == 0 {
		return fmt.Errorf("invalid placeholder: nil or missing job ID")
	}

	dir, err := GetJobsDir()
	if err != nil {
		return err
	}

	if merr := os.MkdirAll(dir, jobsDirPerm); merr != nil {
		return fmt.Errorf("failed to create jobs placeholder directory %s: %w", dir, merr)
	}

	data, err := json.MarshalIndent(ph, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal placeholder: %w", err)
	}

	filePath := filepath.Join(dir, fmt.Sprintf("%d.json", ph.JobID))
	if werr := os.WriteFile(filePath, data, jobsFilePerm); werr != nil {
		return fmt.Errorf("failed to write job placeholder %s: %w", filePath, werr)
	}

	return nil
}

// RemoveJobPlaceholder deletes the placeholder file for a given job ID.
func RemoveJobPlaceholder(jobID uint64) error {
	if jobID == 0 {
		return nil
	}

	dir, err := GetJobsDir()
	if err != nil {
		return err
	}

	filePath := filepath.Join(dir, fmt.Sprintf("%d.json", jobID))
	if rerr := os.Remove(filePath); rerr != nil && !os.IsNotExist(rerr) {
		return rerr
	}

	// Clean up legacy non-json extension filename if it exists
	altPath := filepath.Join(dir, fmt.Sprintf("%d", jobID))
	_ = os.Remove(altPath)

	return nil
}

// GetJobPlaceholder loads a placeholder by job ID if present.
func GetJobPlaceholder(jobID uint64) (*JobPlaceholder, error) {
	if jobID == 0 {
		return nil, fmt.Errorf("invalid job ID: 0")
	}

	dir, err := GetJobsDir()
	if err != nil {
		return nil, err
	}

	filePath := filepath.Join(dir, fmt.Sprintf("%d.json", jobID))
	data, err := os.ReadFile(filepath.Clean(filePath)) //nolint:gosec // G304: Local job placeholder file
	if err != nil {
		if os.IsNotExist(err) {
			// Try fallback without extension
			altPath := filepath.Join(dir, fmt.Sprintf("%d", jobID))
			altData, altErr := os.ReadFile(filepath.Clean(altPath)) //nolint:gosec // G304: Local job placeholder file
			if altErr != nil {
				return nil, err
			}
			data = altData
		} else {
			return nil, err
		}
	}

	var ph JobPlaceholder
	if uerr := json.Unmarshal(data, &ph); uerr != nil {
		return nil, fmt.Errorf("corrupted placeholder file %s: %w", filePath, uerr)
	}
	if ph.JobID == 0 {
		ph.JobID = jobID
	}

	return &ph, nil
}

// ListJobPlaceholders lists all active placeholders sorted by CreatedAt ascending.
func ListJobPlaceholders() ([]*JobPlaceholder, error) {
	dir, err := GetJobsDir()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*JobPlaceholder{}, nil
		}
		return nil, fmt.Errorf("failed to read jobs directory: %w", err)
	}

	var list []*JobPlaceholder
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".json") {
			// Check if filename is purely numeric
			if _, nerr := strconv.ParseUint(name, 10, 64); nerr != nil {
				continue
			}
		}

		filePath := filepath.Join(dir, name)
		data, rerr := os.ReadFile(filepath.Clean(filePath)) //nolint:gosec // G304: Local job placeholder file
		if rerr != nil {
			continue
		}

		var ph JobPlaceholder
		if uerr := json.Unmarshal(data, &ph); uerr != nil {
			continue
		}
		if ph.JobID == 0 {
			numStr := strings.TrimSuffix(name, ".json")
			if id, perr := strconv.ParseUint(numStr, 10, 64); perr == nil {
				ph.JobID = id
			}
		}
		if ph.JobID > 0 {
			list = append(list, &ph)
		}
	}

	sort.Slice(list, func(i, j int) bool {
		if list[i].CreatedAt.Equal(list[j].CreatedAt) {
			return list[i].JobID < list[j].JobID
		}
		return list[i].CreatedAt.Before(list[j].CreatedAt)
	})

	return list, nil
}
