// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"Wavelet/pkg/buildinfo"
	"strings"
	"testing"
)

func TestFormatStartupBanner(t *testing.T) {
	previousVersion := buildinfo.Version
	previousBuildTime := buildinfo.BuildTime
	t.Cleanup(func() {
		buildinfo.Version = previousVersion
		buildinfo.BuildTime = previousBuildTime
	})

	buildinfo.Version = "v3.2.1"
	buildinfo.BuildTime = "2026-07-13T08:00:00Z"

	banner := formatStartupBanner(startupState{
		mode:           "API",
		listensForHTTP: true,
		env:            "production",
		addr:           ":3000",
	})

	for _, want := range []string{
		"Wavelet v3.2.1",
		"Environment: production",
		"Build time:  2026-07-13T08:00:00Z",
		"Listening:   http://:3000",
		"Mode:        API",
	} {
		if !strings.Contains(banner, want) {
			t.Errorf("banner missing %q:\n%s", want, banner)
		}
	}
}
