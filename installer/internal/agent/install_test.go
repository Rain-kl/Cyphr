package agent

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"

	"cyphr/installer/internal/config"
)

func TestInstallAgentFromLocalZip(t *testing.T) {
	tempDir := t.TempDir()
	zipPath := filepath.Join(tempDir, "mock-agent.zip")

	// Create mock zip
	zf, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("create zip failed: %v", err)
	}
	zw := zip.NewWriter(zf)

	// Write mock main.py
	w, _ := zw.Create("agent/main.py")
	_, _ = w.Write([]byte("print('hello agent')"))

	// Write mock config.example.yaml
	w2, _ := zw.Create("agent/config.example.yaml")
	_, _ = w2.Write([]byte("controller_url: http://localhost:8080"))

	_ = zw.Close()
	_ = zf.Close()

	destDir := filepath.Join(tempDir, "installed_agent")
	paths := &config.AppPaths{
		AgentDir: destDir,
	}

	svc := NewService(paths)
	opts := InstallOptions{
		TargetDir:  destDir,
		ZipURL:     zipPath,
		SkipVenv:   true,
		AutoConfig: true,
	}

	stages := []string{}
	err = svc.InstallAgent(opts, func(stage string, progress float64, message string) {
		stages = append(stages, stage)
	})
	if err != nil {
		t.Fatalf("InstallAgent failed: %v", err)
	}

	// Verify main.py and config.yaml exist
	mainPy := filepath.Join(destDir, "main.py")
	if _, err := os.Stat(mainPy); err != nil {
		t.Fatalf("expected main.py to exist in %s", destDir)
	}

	cfgFile := filepath.Join(destDir, "config.yaml")
	if _, err := os.Stat(cfgFile); err != nil {
		t.Fatalf("expected config.yaml to exist in %s", destDir)
	}

	if !svc.IsInstalled() {
		t.Fatalf("expected svc.IsInstalled() to be true")
	}
}
