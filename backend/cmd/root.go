// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"Wavelet/core/extpoints"
	"Wavelet/pkg/buildinfo"
	"Wavelet/pkg/idgen"
	"Wavelet/pkg/logger"
	"Wavelet/pkg/trace"
	"Wavelet/plugins/infra/config"
	"context"
	"log"
	"time"

	"github.com/spf13/cobra"
)

const traceShutdownTimeout = 10 * time.Second

type hostConfig struct {
	App struct {
		AppName string `config:"app_name" env:"APP_NAME" default:"Wavelet"`
		Env     string `config:"env" env:"APP_ENV" default:"production"`
		NodeID  int64  `config:"node_id" env:"APP_NODE_ID" default:"1"`
		Addr    string `config:"addr" env:"APP_ADDR" default:"127.0.0.1:3000"`
	} `config:"app"`
	Log struct {
		Level      string `config:"level" env:"LOG_LEVEL" default:"info"`
		Format     string `config:"format" env:"LOG_FORMAT" default:"json"`
		Output     string `config:"output" env:"LOG_OUTPUT" default:"stdout"`
		FilePath   string `config:"file_path" env:"LOG_FILE_PATH" default:"./logs/app.log"`
		MaxSize    int    `config:"max_size" env:"LOG_MAX_SIZE" default:"100"`
		MaxAge     int    `config:"max_age" env:"LOG_MAX_AGE" default:"30"`
		MaxBackups int    `config:"max_backups" env:"LOG_MAX_BACKUPS" default:"10"`
		Compress   bool   `config:"compress" env:"LOG_COMPRESS" default:"true"`
	} `config:"log"`
	OTel struct {
		SamplingRate float64 `config:"sampling_rate" env:"OTEL_SAMPLING_RATE" default:"1.0"`
		TracerName   string  `config:"tracer_name" env:"OTEL_TRACER_NAME" default:"github.com/Rain-kl/Wavelet"`
	} `config:"otel"`
}

var rootCmd = &cobra.Command{
	Use: "wavelet",
	PersistentPreRun: func(_ *cobra.Command, _ []string) {
		src, err := config.NewSource()
		if err != nil {
			log.Fatalf("[CMD] load config source failed: %v", err)
		}
		var cfg hostConfig
		reg := extpoints.NewConfigRegistry(src)
		_ = reg.Declare("host", extpoints.ConfigBinding{Target: &cfg})
		if err := reg.Resolve(); err != nil {
			log.Fatalf("[CMD] resolve host config failed: %v", err)
		}
		_ = reg.Bind("", &cfg)

		// Initialize idgen snowflake generator
		if err := idgen.Init(cfg.App.NodeID); err != nil {
			log.Fatalf("[CMD] init idgen failed: %v", err)
		}

		logger.Init(logger.Config{
			Level:      cfg.Log.Level,
			Format:     cfg.Log.Format,
			Output:     cfg.Log.Output,
			FilePath:   cfg.Log.FilePath,
			MaxSize:    cfg.Log.MaxSize,
			MaxAge:     cfg.Log.MaxAge,
			MaxBackups: cfg.Log.MaxBackups,
			Compress:   cfg.Log.Compress,
		})
		trace.Init(trace.Config{
			AppName:      cfg.App.AppName,
			SamplingRate: cfg.OTel.SamplingRate,
			TracerName:   cfg.OTel.TracerName,
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
