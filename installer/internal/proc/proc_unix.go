//go:build !windows

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

// isProcessAlive checks if a process responds to signal 0 and is not a zombie.
func isProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	if err != nil {
		return false
	}

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

// daemonizeProcess creates a detached process on Unix systems.
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
	_ = cmd.Process.Release()
	_ = logFile.Close()
	_ = devNull.Close()

	return pid, nil
}

// killProcess sends a termination signal to the process.
func killProcess(pid int, sig syscall.Signal) error {
	return syscall.Kill(pid, sig)
}

// gracefulStopProcess handles Unix graceful process termination.
func gracefulStopProcess(pid int, timeout time.Duration) error {
	if !isProcessAlive(pid) {
		return nil
	}

	_ = killProcess(pid, syscall.SIGTERM)

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !isProcessAlive(pid) {
			return nil
		}
		time.Sleep(150 * time.Millisecond)
	}

	if isProcessAlive(pid) {
		_ = killProcess(pid, syscall.SIGKILL)
		time.Sleep(150 * time.Millisecond)
	}

	return nil
}

// gracefulStopDownloadProcess stops a download process gracefully on Unix.
func gracefulStopDownloadProcess(pid int, timeout time.Duration) error {
	if !isProcessAlive(pid) {
		return nil
	}

	_ = killProcess(pid, syscall.SIGINT)

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !isProcessAlive(pid) {
			return nil
		}
		time.Sleep(150 * time.Millisecond)
	}

	if isProcessAlive(pid) {
		_ = killProcess(pid, syscall.SIGKILL)
		time.Sleep(150 * time.Millisecond)
	}

	return nil
}

// getProcessMetrics retrieves uptime and RSS using ps.
func getProcessMetrics(pid int) (uptime string, rssMB int) {
	out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "etime=,rss=").Output()
	if err == nil {
		fields := strings.Fields(string(bytes.TrimSpace(out)))
		if len(fields) >= 1 {
			uptime = fields[0]
		}
		if len(fields) >= 2 {
			if rssKB, err := strconv.Atoi(fields[1]); err == nil {
				rssMB = rssKB / 1024
			}
		}
	}
	return uptime, rssMB
}
