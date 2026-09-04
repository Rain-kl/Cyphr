package updater

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractBinaryFromArchive(t *testing.T) {
	tempDir := t.TempDir()
	tarGzPath := filepath.Join(tempDir, "installer_test.tar.gz")

	// Create a mock tar.gz
	f, err := os.Create(tarGzPath)
	if err != nil {
		t.Fatalf("create file failed: %v", err)
	}
	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)

	content := []byte("#!/bin/sh\necho update\n")
	hdr := &tar.Header{
		Name: "cyphr-installer/cyphr-installer",
		Mode: 0755,
		Size: int64(len(content)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("write header failed: %v", err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatalf("write content failed: %v", err)
	}

	_ = tw.Close()
	_ = gw.Close()
	_ = f.Close()

	extractedExe, err := extractBinaryFromArchive(tarGzPath, "cyphr-installer_v1.0.0_darwin_arm64.tar.gz", "cyphr-installer")
	if err != nil {
		t.Fatalf("extract failed: %v", err)
	}
	defer os.Remove(extractedExe)

	data, err := os.ReadFile(extractedExe)
	if err != nil {
		t.Fatalf("read extracted file failed: %v", err)
	}
	if string(data) != string(content) {
		t.Fatalf("content mismatch: got %q, expected %q", string(data), string(content))
	}
}

func TestReplaceExecutable(t *testing.T) {
	tempDir := t.TempDir()
	currentExe := filepath.Join(tempDir, "current_exe")
	newExe := filepath.Join(tempDir, "new_exe")

	_ = os.WriteFile(currentExe, []byte("v1"), 0755)
	_ = os.WriteFile(newExe, []byte("v2"), 0755)

	if err := replaceExecutable(currentExe, newExe); err != nil {
		t.Fatalf("replaceExecutable failed: %v", err)
	}

	data, err := os.ReadFile(currentExe)
	if err != nil {
		t.Fatalf("read replaced file failed: %v", err)
	}
	if string(data) != "v2" {
		t.Fatalf("expected v2, got %s", string(data))
	}
}
