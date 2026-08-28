// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"log"

	"Wavelet/core"
	"github.com/spf13/cobra"
)

var apiCmd = &cobra.Command{
	Use:   "api",
	Short: "wavelet API",
	Run: func(_ *cobra.Command, _ []string) {
		app := newWaveletApp(core.ProfileAPI)
		if err := app.Run(); err != nil {
			log.Fatalf("[API] run failed: %v\n", err)
		}
	},
}
