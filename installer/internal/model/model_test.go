package model

import (
	"os"
	"path/filepath"
	"testing"

	"cyphr/installer/internal/config"
)

func TestListLocalModels(t *testing.T) {
	tempDir := t.TempDir()
	modelsDir := filepath.Join(tempDir, "models")
	paths := &config.AppPaths{
		ModelsDir: modelsDir,
	}

	// Model 1: complete
	m1 := filepath.Join(modelsDir, "qwen3-asr-0.6b")
	_ = os.MkdirAll(m1, 0755)
	_ = os.WriteFile(filepath.Join(m1, "config.json"), []byte("{}"), 0644)
	_ = os.WriteFile(filepath.Join(m1, "model.bin"), []byte("data"), 0644)

	// Model 2: partial
	m2 := filepath.Join(modelsDir, "whisper-base")
	_ = os.MkdirAll(m2, 0755)
	_ = os.WriteFile(filepath.Join(m2, "config.json"), []byte("{}"), 0644)
	_ = os.WriteFile(filepath.Join(m2, "model.safetensors.part"), []byte("part"), 0644)

	svc := NewService(paths)
	models, err := svc.ListLocalModels()
	if err != nil {
		t.Fatalf("ListLocalModels failed: %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(models))
	}

	foundReady := false
	foundPart := false
	for _, m := range models {
		if m.DirName == "qwen3-asr-0.6b" && m.IsReady {
			foundReady = true
		}
		if m.DirName == "whisper-base" && !m.IsReady {
			foundPart = true
		}
	}
	if !foundReady || !foundPart {
		t.Fatalf("expected foundReady=%v, foundPart=%v", foundReady, foundPart)
	}
}
