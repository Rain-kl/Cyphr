// Copyright 2025 linux.do
// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"log"
	"time"

	"github.com/Rain-kl/Wavelet/pkg/buildinfo"
	"github.com/Rain-kl/Wavelet/pkg/config"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	"github.com/Rain-kl/Wavelet/pkg/trace"
	"github.com/spf13/cobra"
)

const traceShutdownTimeout = 10 * time.Second

var rootCmd = &cobra.Command{
	Use: "wavelet",
	PersistentPreRun: func(_ *cobra.Command, _ []string) {
		logger.Init(logger.Config{
			Level:      config.Config.Log.Level,
			Format:     config.Config.Log.Format,
			Output:     config.Config.Log.Output,
			FilePath:   config.Config.Log.FilePath,
			MaxSize:    config.Config.Log.MaxSize,
			MaxAge:     config.Config.Log.MaxAge,
			MaxBackups: config.Config.Log.MaxBackups,
			Compress:   config.Config.Log.Compress,
		})
		trace.Init(trace.Config{
			AppName:      config.Config.App.AppName,
			SamplingRate: config.Config.Otel.SamplingRate,
			TracerName:   config.Config.Otel.TracerName,
		})
	},
	PersistentPostRun: func(_ *cobra.Command, _ []string) {
		shutdownTraceProvider()
	},
	Run: func(_ *cobra.Command, args []string) {
		// 无参数时默认以融合模式启动所有服务
		allCmd.Run(allCmd, args)
	},
}

func shutdownTraceProvider() {
	ctx, cancel := context.WithTimeout(context.Background(), traceShutdownTimeout)
	defer cancel()
	trace.Shutdown(ctx)
}

func init() {
	rootCmd.Version = buildinfo.Version
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	// 集中将子命令注册到根命令，以解决 Cobra 的 unknown command 校验限制
	rootCmd.AddCommand(allCmd, apiCmd, workerCmd, schedulerCmd)
}

// Execute 执行根命令
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatalf("[CMD] execute failed; %s\n", err)
	}
}
