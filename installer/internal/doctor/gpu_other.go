//go:build !windows

package doctor

import (
	"context"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// querySystemDisplayAdapters queries display adapters on non-Windows systems (Linux/macOS).
//
//nolint:unused
func querySystemDisplayAdapters() []string {
	var adapters []string
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if runtime.GOOS == "darwin" {
		cmd := exec.CommandContext(ctx, "system_profiler", "SPDisplaysDataType")
		if out, err := cmd.Output(); err == nil {
			lines := strings.Split(string(out), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "Chipset Model:") {
					model := strings.TrimSpace(strings.TrimPrefix(line, "Chipset Model:"))
					adapters = append(adapters, model)
				}
			}
		}
		return adapters
	}

	// Linux: try lspci
	if _, err := exec.LookPath("lspci"); err == nil {
		cmd := exec.CommandContext(ctx, "lspci")
		if out, err := cmd.Output(); err == nil {
			lines := strings.Split(string(out), "\n")
			for _, line := range lines {
				lower := strings.ToLower(line)
				if strings.Contains(lower, "vga") || strings.Contains(lower, "3d controller") || strings.Contains(lower, "display") {
					parts := strings.SplitN(line, ": ", 2)
					if len(parts) == 2 {
						adapters = append(adapters, strings.TrimSpace(parts[1]))
					} else {
						adapters = append(adapters, strings.TrimSpace(line))
					}
				}
			}
		}
	}

	return adapters
}
