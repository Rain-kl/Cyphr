package doctor

import (
	"testing"

	"cyphr/installer/internal/config"
)

func TestDoctorRun(t *testing.T) {
	tempDir := t.TempDir()
	paths := config.NewAppPathsFromDir(tempDir)

	report := Run(paths)
	if report == nil {
		t.Fatal("expected non-nil doctor report")
	}

	if report.OS == "" || report.Arch == "" {
		t.Errorf("expected OS and Arch to be non-empty, got OS=%s, Arch=%s", report.OS, report.Arch)
	}

	formatted := report.Format()
	if formatted == "" {
		t.Error("expected non-empty formatted report")
	}
}
