//go:build windows

package proc

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Windows process creation flags
const (
	createNoWindow        = 0x08000000
	detachedProcess       = 0x00000008
	createBreakawayFromJob = 0x01000000
)

// isProcessAlive checks if a process with given pid exists using tasklist or process open.
func isProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}

	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	defer p.Release()

	// On Windows, FindProcess always succeeds. Check with tasklist to verify actual existence.
	cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/FO", "CSV", "/NH")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	line := strings.TrimSpace(string(out))
	if line == "" || strings.Contains(line, "No tasks") || strings.Contains(line, "INFO:") {
		return false
	}

	return strings.Contains(line, strconv.Itoa(pid))
}

// daemonizeProcess creates a detached background process on Windows.
func daemonizeProcess(command []string, env []string, cwd string, logPath string) (int, error) {
	if len(command) == 0 {
		return 0, fmt.Errorf("empty command")
	}

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

	// Detached and no-window process creation on Windows
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: createNoWindow | detachedProcess | createBreakawayFromJob,
	}

	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		_ = devNull.Close()
		return 0, fmt.Errorf("start daemon process failed: %w", err)
	}

	pid := cmd.Process.Pid
	_ = cmd.Process.Release()
	_ = logFile.Close()
	_ = devNull.Close()

	return pid, nil
}

// gracefulStopProcess attempts graceful termination on Windows via taskkill /PID.
func gracefulStopProcess(pid int, timeout time.Duration) error {
	if !isProcessAlive(pid) {
		return nil
	}

	// Try graceful stop without /F first
	_ = exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/T").Run()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !isProcessAlive(pid) {
			return nil
		}
		time.Sleep(150 * time.Millisecond)
	}

	// Force kill with /F if still running
	if isProcessAlive(pid) {
		_ = exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/T", "/F").Run()
		time.Sleep(150 * time.Millisecond)
	}

	return nil
}

// gracefulStopDownloadProcess stops download process on Windows.
func gracefulStopDownloadProcess(pid int, timeout time.Duration) error {
	return gracefulStopProcess(pid, timeout)
}

// getProcessMetrics retrieves memory usage using tasklist on Windows.
func getProcessMetrics(pid int) (uptime string, rssMB int) {
	cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/FO", "CSV", "/NH")
	out, err := cmd.Output()
	if err != nil {
		return "", 0
	}

	line := strings.TrimSpace(string(out))
	if line == "" {
		return "", 0
	}

	// CSV format: "Image Name","PID","Session Name","Session#","Mem Usage"
	parts := strings.Split(line, ",")
	if len(parts) >= 5 {
		memStr := strings.Trim(parts[4], "\" ")
		memStr = strings.ReplaceAll(memStr, " K", "")
		memStr = strings.ReplaceAll(memStr, ",", "")
		memStr = strings.ReplaceAll(memStr, " ", "")
		if kb, err := strconv.Atoi(memStr); err == nil {
			rssMB = kb / 1024
		}
	}
	return "", rssMB
}
