// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package main is the entry point for the standalone transcribe CLI executable.
package main

import (
	"os"

	"Wavelet/transcribe/plugins/cli/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
