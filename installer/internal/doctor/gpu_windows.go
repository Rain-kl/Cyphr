//go:build windows

package doctor

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

// querySystemDisplayAdapters queries Windows display adapters via PowerShell CIM or WMIC.
func querySystemDisplayAdapters() []string {
	var adapters []string

	// 1. Try PowerShell Get-CimInstance
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command",
		`Get-CimInstance Win32_VideoController | ForEach-Object { "$($_.Name)|$($_.DriverVersion)" }`)
	if out, err := cmd.Output(); err == nil {
		lines := strings.Split(string(out), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			parts := strings.Split(line, "|")
			name := strings.TrimSpace(parts[0])
			driver := ""
			if len(parts) > 1 {
				driver = strings.TrimSpace(parts[1])
			}
			if name != "" {
				if driver != "" {
					adapters = append(adapters, name+" (驱动版本: "+driver+")")
				} else {
					adapters = append(adapters, name)
				}
			}
		}
		if len(adapters) > 0 {
			return adapters
		}
	}

	// 2. Fallback to WMIC
	ctxWmic, cancelWmic := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancelWmic()

	wmicCmd := exec.CommandContext(ctxWmic, "wmic", "path", "win32_VideoController", "get", "name,driverversion", "/format:csv")
	if out, err := wmicCmd.Output(); err == nil {
		lines := strings.Split(string(out), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "Node,") {
				continue
			}
			parts := strings.Split(line, ",")
			if len(parts) >= 3 {
				driver := strings.TrimSpace(parts[1])
				name := strings.TrimSpace(parts[2])
				if name != "" {
					if driver != "" {
						adapters = append(adapters, name+" (驱动版本: "+driver+")")
					} else {
						adapters = append(adapters, name)
					}
				}
			}
		}
	}

	return adapters
}
