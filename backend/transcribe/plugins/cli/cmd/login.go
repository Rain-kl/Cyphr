// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"Wavelet/transcribe/plugins/cli/client"
	"Wavelet/transcribe/plugins/cli/config"
)

var (
	loginURL   string
	loginToken string
)

func promptInput(r io.Reader, w io.Writer, prompt, defaultVal string) string {
	_, _ = fmt.Fprint(w, prompt)
	scanner := bufio.NewScanner(r)
	if scanner.Scan() {
		val := strings.TrimSpace(scanner.Text())
		if val != "" {
			return val
		}
	}
	return defaultVal
}

func newLoginCmd() *cobra.Command {
	loginCmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate and configure connection to the transcribe controller",
		RunE: func(cmd *cobra.Command, _ []string) error {
			targetURL := strings.TrimSpace(loginURL)
			if targetURL == "" && appConfig != nil {
				targetURL = appConfig.ControllerURL
			}
			if targetURL == "" {
				targetURL = promptInput(cmd.InOrStdin(), cmd.OutOrStdout(), fmt.Sprintf("Controller URL [%s]: ", config.DefaultControllerURL), config.DefaultControllerURL)
			}

			targetToken := strings.TrimSpace(loginToken)
			if targetToken == "" && appConfig != nil {
				targetToken = appConfig.AccessToken
			}
			if targetToken == "" {
				targetToken = promptInput(cmd.InOrStdin(), cmd.OutOrStdout(), "Access Token: ", "")
			}

			targetURL = strings.TrimRight(strings.TrimSpace(targetURL), "/")
			if targetURL == "" {
				targetURL = config.DefaultControllerURL
			}

			// Validate credentials by pinging controller models endpoint
			testClient := client.New(targetURL, targetToken)
			if err := testClient.TestConnection(cmd.Context()); err != nil {
				return fmt.Errorf("connection test failed against %s: %w", targetURL, err)
			}

			if appConfig == nil {
				appConfig = &config.Config{}
			}
			appConfig.ControllerURL = targetURL
			appConfig.AccessToken = targetToken
			if appConfig.DefaultModel == "" {
				appConfig.DefaultModel = config.DefaultModel
			}

			targetPath, err := config.ResolvePath(cfgFile)
			if err != nil {
				return fmt.Errorf("failed to resolve config path: %w", err)
			}

			if err := config.Save(appConfig, targetPath); err != nil {
				return fmt.Errorf("failed to save configuration: %w", err)
			}

			cmd.Printf("Successfully logged in to %s\nConfiguration saved to %s\n", targetURL, targetPath)
			return nil
		},
	}

	loginCmd.Flags().StringVar(&loginURL, "url", "", "transcribe controller URL")
	loginCmd.Flags().StringVar(&loginToken, "token", "", "user or agent access token")

	return loginCmd
}
