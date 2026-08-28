// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"log"

	"github.com/spf13/cobra"

	"Wavelet/core"
)

var workerCmd = &cobra.Command{
	Use:   "worker",
	Short: "wavelet Worker",
	Run: func(_ *cobra.Command, _ []string) {
		printStartupBanner(startupState{
			mode:           "worker",
			listensForHTTP: false,
		})
		app := newWaveletApp(core.ProfileWorker)
		if err := app.Run(); err != nil {
			log.Fatalf("[Worker] run failed: %v\n", err)
		}
	},
}
