// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package logstore

import (
	"os/exec"
	"strings"
	"testing"
)

// apps 禁止直连 analytics 做日志读写；查询过滤器请用 logstore.AccessLogFilter。
var forbiddenImports = []string{
	"github.com/Rain-kl/Wavelet/internal/repository/analytics",
}

// logstore 的 CH 实现按设计委托 analyticsrepo。
var allowedAnalyticsDelegation = map[string]bool{
	"github.com/Rain-kl/Wavelet/internal/repository/logstore": true,
}

func TestAppsMustNotImportLogBackendDirectly(t *testing.T) {
	t.Chdir("../../..")
	out, err := exec.Command("go", "list", "-test", "-f", `{{.ImportPath}} {{join .Imports " "}}`, "./internal/apps/...").Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		pkg := fields[0]
		if !strings.HasPrefix(pkg, "github.com/Rain-kl/Wavelet/internal/apps") {
			continue
		}
		for _, imp := range fields[1:] {
			for _, forbidden := range forbiddenImports {
				if imp == forbidden && !allowedAnalyticsDelegation[pkg] {
					t.Errorf("%s must not import forbidden log backend %s", pkg, forbidden)
				}
			}
		}
	}
}
