// Package proc provides cross-platform process management, daemonization, and status inspection.
package proc

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	dirPerm  = 0750
	filePerm = 0600
)

// ProcessStats holds runtime metrics for a running process.
type ProcessStats struct {
	PID     int
	Running bool
	Uptime  string
	RSSMB   int
}

// IsRunning tests if a process with the given PID exists and is actively running.
func IsRunning(pid int) bool {
	return isProcessAlive(pid)
}

// ReadPid reads and parses an integer PID from a file.
func ReadPid(pidFile string) (int, error) {
	data, err := os.ReadFile(filepath.Clean(pidFile)) //nolint:gosec // Local PID file read
	if err != nil {
		return 0, err
	}
	pidStr := strings.TrimSpace(string(data))
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return 0, fmt.Errorf("invalid pid in %s: %w", pidFile, err)
	}
	return pid, nil
}

// WritePid writes an integer PID to a file.
func WritePid(pidFile string, pid int) error {
	dir := filepath.Dir(pidFile)
	if err := os.MkdirAll(dir, dirPerm); err != nil { //nolint:gosec // Directory permissions
		return err
	}
	return os.WriteFile(filepath.Clean(pidFile), []byte(strconv.Itoa(pid)+"\n"), filePerm) //nolint:gosec // Local PID file write
}

// RemovePid removes a PID file safely.
func RemovePid(pidFile string) {
	_ = os.Remove(pidFile)
}

// Daemonize launches a command in a detached session with devnull stdin and output redirected.
func Daemonize(command []string, env []string, cwd string, logPath string) (int, error) {
	return daemonizeProcess(command, env, cwd, logPath)
}

// GracefulStop attempts to terminate a process gracefully before force killing.
func GracefulStop(pid int, timeout time.Duration) error {
	return gracefulStopProcess(pid, timeout)
}

// GracefulStopDownload sends interrupt signal before force killing.
func GracefulStopDownload(pid int, timeout time.Duration) error {
	return gracefulStopDownloadProcess(pid, timeout)
}

// GetStats inspects a process and returns its stats.
func GetStats(pid int) ProcessStats {
	stats := ProcessStats{
		PID:     pid,
		Running: IsRunning(pid),
	}
	if !stats.Running {
		return stats
	}

	uptime, rssMB := getProcessMetrics(pid)
	stats.Uptime = uptime
	stats.RSSMB = rssMB
	return stats
}

// TailLines returns the last n lines of a file.
func TailLines(filePath string, n int) []string {
	data, err := os.ReadFile(filepath.Clean(filePath)) //nolint:gosec // Local log file read
	if err != nil {
		return nil
	}
	lines := strings.Split(string(data), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) <= n {
		return lines
	}
	return lines[len(lines)-n:]
}
