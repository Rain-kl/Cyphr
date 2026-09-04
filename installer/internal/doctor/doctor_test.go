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

func TestProbeResultParsingAndSynthesis(t *testing.T) {
	cudaVer := "13.0"
	rep := &Report{
		ProbeRun: true,
		ProbeResult: PythonProbeResult{
			PythonVersion: "3.12.3",
			Is64Bit:       true,
			Platform:      "linux",
			Torch: struct {
				Installed     bool    `json:"installed"`
				Version       string  `json:"version"`
				CUDAVersion   *string `json:"cuda_version"`
				CUDAAvailable bool    `json:"cuda_available"`
				DeviceCount   int     `json:"device_count"`
				DeviceName    string  `json:"device_name"`
				Error         string  `json:"error"`
			}{
				Installed:     true,
				Version:       "2.14.0+cu130",
				CUDAVersion:   &cudaVer,
				CUDAAvailable: true,
				DeviceCount:   2,
				DeviceName:    "NVIDIA A10",
			},
			QwenASR: true,
		},
	}

	rep.synthesize()

	if !rep.HasGPUAcceleration {
		t.Errorf("expected HasGPUAcceleration to be true, got false")
	}
	for _, issue := range rep.Issues {
		if issue == "Agent Python 环境未安装 PyTorch 深度学习框架。" {
			t.Errorf("unexpected issue reported: %s", issue)
		}
	}
}
