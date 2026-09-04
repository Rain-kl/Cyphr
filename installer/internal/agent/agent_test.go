package agent

import (
	"os"
	"path/filepath"
	"testing"

	"cyphr/installer/internal/config"
)

func TestAgentStatusStopped(t *testing.T) {
	tempDir := t.TempDir()
	paths := &config.AppPaths{
		AgentDir: tempDir,
		PidFile:  filepath.Join(tempDir, "agent.pid"),
		LogFile:  filepath.Join(tempDir, "agent.log"),
	}

	svc := NewService(paths)
	st := svc.Status()
	if st.Running {
		t.Fatalf("expected agent to be stopped initially")
	}

	// Write mock log
	_ = os.WriteFile(paths.LogFile, []byte("line1\nline2\nline3\n"), 0644)
	st = svc.Status()
	if len(st.RecentLogs) != 3 {
		t.Fatalf("expected 3 log lines, got %d", len(st.RecentLogs))
	}
}
