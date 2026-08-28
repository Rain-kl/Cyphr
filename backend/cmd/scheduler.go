// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"log"

	"Wavelet/core"
	"github.com/spf13/cobra"
)

var schedulerCmd = &cobra.Command{
	Use:   "scheduler",
	Short: "wavelet Scheduler",
	Run: func(_ *cobra.Command, _ []string) {
		app := newWaveletApp(core.ProfileSchedule)
		if err := app.Run(); err != nil {
			log.Fatalf("[Scheduler] run failed: %v\n", err)
		}
	},
}
