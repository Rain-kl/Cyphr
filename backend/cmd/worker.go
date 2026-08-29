// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"Wavelet/core"

	"github.com/spf13/cobra"
)

var workerCmd = &cobra.Command{
	Use:   "worker",
	Short: "wavelet Worker",
	Run: func(_ *cobra.Command, _ []string) {
		runProfileApp(core.ProfileWorker, "worker", false)
	},
}
