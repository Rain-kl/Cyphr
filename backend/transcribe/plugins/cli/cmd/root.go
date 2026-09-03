// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package cmd provides Cobra CLI commands for the transcribe tool.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"Wavelet/transcribe/plugins/cli/client"
	"Wavelet/transcribe/plugins/cli/config"
)

var (
	cfgFile       string
	overrideURL   string
	overrideToken string

	appConfig *config.Config
	appClient *client.Client
)

const (
	// resultFilePerm keeps transcribed text readable only by the file owner.
	resultFilePerm = 0o600
)

// NewRootCmd creates and returns the root command for the cyphr CLI.
func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:           "cyphr",
		Aliases:       []string{"transcribe"},
		Short:         "Cyphr CLI - audio & video speech recognition client",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			var err error
			appConfig, err = config.Load(cfgFile)
			if err != nil {
				return fmt.Errorf("failed to load configuration: %w", err)
			}

			if overrideURL != "" {
				appConfig.ControllerURL = overrideURL
			}
			if overrideToken != "" {
				appConfig.AccessToken = overrideToken
			}

			appClient = client.New(appConfig.ControllerURL, appConfig.AccessToken)
			return nil
		},
	}

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.cyphr/config.yaml)")
	rootCmd.PersistentFlags().StringVar(&overrideURL, "url", "", "controller server URL")
	rootCmd.PersistentFlags().StringVar(&overrideToken, "token", "", "access token for authentication")

	rootCmd.AddCommand(NewLoginCmd())
	rootCmd.AddCommand(NewAsrCmd())
	rootCmd.AddCommand(NewJobsCmd())
	rootCmd.AddCommand(NewModelsCmd())

	return rootCmd
}

// Execute runs the root command and exits on error.
func Execute() error {
	rootCmd := NewRootCmd()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return err
	}
	return nil
}
