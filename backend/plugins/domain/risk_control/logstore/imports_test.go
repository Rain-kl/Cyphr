// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package logstore

import (
	"os/exec"
	"strings"
	"testing"
)

// plugins/domain 不得直接 import 已废弃的 internal。
var forbiddenImports = []string{
	"github.com/Rain-kl/Wavelet/internal",
	"github.com/Rain-kl/Wavelet/internal",
}

func TestAppsMustNotImportPersistenceDirectly(t *testing.T) {
	t.Chdir("../../../..")
	out, err := exec.Command("go", "list", "-test", "-f", `{{.ImportPath}} {{join .Imports " "}}`, "./plugins/domain/...").Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		pkg := fields[0]
		if !strings.HasPrefix(pkg, "github.com/Rain-kl/Wavelet/plugins/domain") {
			continue
		}
		for _, imp := range fields[1:] {
			for _, forbidden := range forbiddenImports {
				if imp == forbidden {
					t.Errorf("%s must not import forbidden package %s", pkg, forbidden)
				}
			}
		}
	}
}
