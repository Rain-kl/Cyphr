package proc

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReadWritePid(t *testing.T) {
	tempDir := t.TempDir()
	pidFile := filepath.Join(tempDir, "test.pid")

	if err := WritePid(pidFile, 12345); err != nil {
		t.Fatalf("WritePid failed: %v", err)
	}

	pid, err := ReadPid(pidFile)
	if err != nil {
		t.Fatalf("ReadPid failed: %v", err)
	}
	if pid != 12345 {
		t.Fatalf("expected 12345, got %d", pid)
	}

	RemovePid(pidFile)
	if _, err := os.Stat(pidFile); !os.IsNotExist(err) {
		t.Fatalf("expected pid file to be removed")
	}
}

func TestDaemonizeAndStop(t *testing.T) {
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "test.log")

	// Start a sleep process
	pid, err := Daemonize([]string{"sleep", "5"}, nil, tempDir, logFile)
	if err != nil {
		t.Fatalf("Daemonize failed: %v", err)
	}

	if !IsRunning(pid) {
		t.Fatalf("expected process %d to be running", pid)
	}

	stats := GetStats(pid)
	if !stats.Running {
		t.Fatalf("expected stats.Running to be true")
	}

	// Stop it
	if err := GracefulStop(pid, 2*time.Second); err != nil {
		t.Fatalf("GracefulStop failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)
	if IsRunning(pid) {
		t.Fatalf("expected process %d to be stopped", pid)
	}
}
