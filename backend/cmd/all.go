// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package cmd 提供 CLI 命令入口
package cmd

import (
	"Wavelet/core"

	"github.com/spf13/cobra"
)

var allCmd = &cobra.Command{
	Use:   "all",
	Short: "以融合模式同时启动 API、Worker 和 Scheduler",
	Run: func(_ *cobra.Command, _ []string) {
		runProfileApp(core.ProfileAll, "all (API + Worker + Scheduler)", true)
	},
}
