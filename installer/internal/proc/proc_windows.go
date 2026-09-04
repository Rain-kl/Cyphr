//go:build windows

package proc

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
)

// Windows process creation flags
const (
	createNoWindow        = 0x08000000 // CREATE_NO_WINDOW
	createNewProcessGroup = 0x00000200 // CREATE_NEW_PROCESS_GROUP
)

// isProcessAlive checks if a process with given pid exists using native Windows API.
func isProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}

	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		// If access is denied, the process exists and is running under another security context.
		return err == windows.ERROR_ACCESS_DENIED
	}
	defer windows.CloseHandle(h)

	var exitCode uint32
	if err := windows.GetExitCodeProcess(h, &exitCode); err != nil {
		return false
	}

	const stillActive = 259
	return exitCode == stillActive
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

	cmd := exec.CommandContext(context.Background(), command[0], command[1:]...)
	if cwd != "" {
		cmd.Dir = cwd
	}
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}

	cmd.Stdin = devNull
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	// Hidden console and independent process group (CREATE_NO_WINDOW | CREATE_NEW_PROCESS_GROUP)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: createNoWindow | createNewProcessGroup,
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
	_ = exec.CommandContext(context.Background(), "taskkill", "/PID", strconv.Itoa(pid), "/T").Run()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !isProcessAlive(pid) {
			return nil
		}
		time.Sleep(150 * time.Millisecond)
	}

	// Force kill with /F if still running
	if isProcessAlive(pid) {
		_ = exec.CommandContext(context.Background(), "taskkill", "/PID", strconv.Itoa(pid), "/T", "/F").Run()
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
	cmd := exec.CommandContext(context.Background(), "tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/FO", "CSV", "/NH")
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
