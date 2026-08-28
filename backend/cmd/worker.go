// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"log"

	"github.com/Rain-kl/Wavelet/backend/core"
	"github.com/spf13/cobra"
)

var workerCmd = &cobra.Command{
	Use:   "worker",
	Short: "wavelet Worker",
	Run: func(_ *cobra.Command, _ []string) {
		app := newWaveletApp(core.ProfileWorker)
		if err := app.Run(); err != nil {
			log.Fatalf("[Worker] run failed: %v\n", err)
		}
	},
}
