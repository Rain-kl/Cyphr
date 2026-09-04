//go:build !windows

package doctor

import (
	"context"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

const (
	queryTimeout = 3 * time.Second
	splitParts   = 2
)

// querySystemDisplayAdapters queries display adapters on non-Windows systems (Linux/macOS).
//
//nolint:unused
func querySystemDisplayAdapters() []string {
	ctx, cancel := context.WithTimeout(context.Background(), queryTimeout)
	defer cancel()

	if runtime.GOOS == "darwin" {
		return queryDarwinAdapters(ctx)
	}

	return queryLinuxAdapters(ctx)
}

func queryDarwinAdapters(ctx context.Context) []string {
	var adapters []string
	cmd := exec.CommandContext(ctx, "system_profiler", "SPDisplaysDataType") //nolint:gosec // Diagnostic tool command
	out, err := cmd.Output()
	if err != nil {
		return adapters
	}
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Chipset Model:") {
			model := strings.TrimSpace(strings.TrimPrefix(line, "Chipset Model:"))
			adapters = append(adapters, model)
		}
	}
	return adapters
}

func queryLinuxAdapters(ctx context.Context) []string {
	var adapters []string
	if _, err := exec.LookPath("lspci"); err != nil {
		return adapters
	}
	cmd := exec.CommandContext(ctx, "lspci") //nolint:gosec // Diagnostic tool command
	out, err := cmd.Output()
	if err != nil {
		return adapters
	}
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		lower := strings.ToLower(line)
		if !strings.Contains(lower, "vga") && !strings.Contains(lower, "3d controller") && !strings.Contains(lower, "display") {
			continue
		}
		parts := strings.SplitN(line, ": ", splitParts)
		if len(parts) == splitParts {
			adapters = append(adapters, strings.TrimSpace(parts[1]))
		} else {
			adapters = append(adapters, strings.TrimSpace(line))
		}
	}
	return adapters
}
