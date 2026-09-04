package proc

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// ProcessStats holds runtime metrics for a running process.
type ProcessStats struct {
	PID     int
	Running bool
	Uptime  string
	RSSMB   int
}

// IsRunning tests if a process with the given PID exists, responds to signal 0, and is not a zombie (Z).
func IsRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	// Direct syscall test
	err := syscall.Kill(pid, 0)
	if err != nil {
		return false
	}

	// Re-verify that it is not in zombie (Z) state using ps
	out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "stat=").Output()
	if err != nil {
		return false
	}
	stat := strings.TrimSpace(string(out))
	if stat == "" || strings.HasPrefix(stat, "Z") {
		return false
	}

	return true
}

// ReadPid reads and parses an integer PID from a file.
func ReadPid(pidFile string) (int, error) {
	data, err := os.ReadFile(pidFile)
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
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(pidFile, []byte(strconv.Itoa(pid)+"\n"), 0644)
}

// RemovePid removes a PID file safely.
func RemovePid(pidFile string) {
	_ = os.Remove(pidFile)
}

// Daemonize launches a command in a detached session with setsid, devnull stdin, and output redirected.
func Daemonize(command []string, env []string, cwd string, logPath string) (int, error) {
	if len(command) == 0 {
		return 0, fmt.Errorf("empty command")
	}

	// Ensure log file directory exists
	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		return 0, fmt.Errorf("create log dir failed: %w", err)
	}

	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return 0, fmt.Errorf("open log file failed: %w", err)
	}

	devNull, err := os.OpenFile(os.DevNull, os.O_RDONLY, 0)
	if err != nil {
		_ = logFile.Close()
		return 0, fmt.Errorf("open devnull failed: %w", err)
	}

	cmd := exec.Command(command[0], command[1:]...)
	if cwd != "" {
		cmd.Dir = cwd
	}
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}

	cmd.Stdin = devNull
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	// Create new session & process group (setsid)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}

	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		_ = devNull.Close()
		return 0, fmt.Errorf("start daemon process failed: %w", err)
	}

	pid := cmd.Process.Pid
	// Release process handle
	_ = cmd.Process.Release()
	_ = logFile.Close()
	_ = devNull.Close()

	return pid, nil
}

// GracefulStop attempts to terminate a process by sending SIGTERM, waiting up to timeout, then SIGKILL.
func GracefulStop(pid int, timeout time.Duration) error {
	if !IsRunning(pid) {
		return nil
	}

	_ = syscall.Kill(pid, syscall.SIGTERM)

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !IsRunning(pid) {
			return nil
		}
		time.Sleep(150 * time.Millisecond)
	}

	if IsRunning(pid) {
		_ = syscall.Kill(pid, syscall.SIGKILL)
		time.Sleep(150 * time.Millisecond)
	}

	return nil
}

// GracefulStopDownload sends SIGINT first, then SIGKILL.
func GracefulStopDownload(pid int, timeout time.Duration) error {
	if !IsRunning(pid) {
		return nil
	}

	_ = syscall.Kill(pid, syscall.SIGINT)

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !IsRunning(pid) {
			return nil
		}
		time.Sleep(150 * time.Millisecond)
	}

	if IsRunning(pid) {
		_ = syscall.Kill(pid, syscall.SIGKILL)
		time.Sleep(150 * time.Millisecond)
	}

	return nil
}

// GetStats inspects a process using ps.
func GetStats(pid int) ProcessStats {
	stats := ProcessStats{
		PID:     pid,
		Running: IsRunning(pid),
	}
	if !stats.Running {
		return stats
	}

	out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "etime=,rss=").Output()
	if err == nil {
		fields := strings.Fields(string(bytes.TrimSpace(out)))
		if len(fields) >= 1 {
			stats.Uptime = fields[0]
		}
		if len(fields) >= 2 {
			if rssKB, err := strconv.Atoi(fields[1]); err == nil {
				stats.RSSMB = rssKB / 1024
			}
		}
	}
	return stats
}

// TailLines returns the last n lines of a file.
func TailLines(filePath string, n int) []string {
	data, err := os.ReadFile(filePath)
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
