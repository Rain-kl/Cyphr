// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"log"

	"github.com/spf13/cobra"

	"Wavelet/core"
)

var apiCmd = &cobra.Command{
	Use:   "api",
	Short: "wavelet API",
	Run: func(_ *cobra.Command, _ []string) {
		printStartupBanner(startupState{
			mode:           "api",
			listensForHTTP: true,
		})
		app := newWaveletApp(core.ProfileAPI)
		if err := app.Run(); err != nil {
			log.Fatalf("[API] run failed: %v\n", err)
		}
	},
}
