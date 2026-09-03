// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"Wavelet/transcribe/plugins/cli/client"
	"Wavelet/transcribe/plugins/cli/config"
)

func promptInput(r *bufio.Reader, w io.Writer, prompt, defaultVal string) string {
	_, _ = fmt.Fprint(w, prompt)
	line, err := r.ReadString('\n')
	if err == nil || len(line) > 0 {
		val := strings.TrimSpace(line)
		if val != "" {
			return val
		}
	}
	return defaultVal
}

func resolveLoginURL(cmd *cobra.Command, r *bufio.Reader) string {
	urlFlag := cmd.Flags().Lookup("url")
	urlEnv := strings.TrimSpace(os.Getenv(config.EnvCyphrURL))
	if urlEnv == "" {
		urlEnv = strings.TrimSpace(os.Getenv(config.EnvTranscribeURL))
	}
	urlExplicit := (urlFlag != nil && urlFlag.Changed) || urlEnv != ""
	if !urlExplicit {
		promptDefault := config.DefaultControllerURL
		if appConfig != nil && appConfig.ControllerURL != "" {
			promptDefault = appConfig.ControllerURL
		}
		return promptInput(r, cmd.OutOrStdout(), fmt.Sprintf("Controller URL [%s]: ", promptDefault), promptDefault)
	}

	targetURL := overrideURL
	if targetURL == "" && appConfig != nil {
		targetURL = appConfig.ControllerURL
	}
	if targetURL == "" {
		targetURL = config.DefaultControllerURL
	}
	return targetURL
}

func resolveLoginToken(cmd *cobra.Command, r *bufio.Reader) string {
	tokenFlag := cmd.Flags().Lookup("token")
	tokenEnv := strings.TrimSpace(os.Getenv(config.EnvCyphrToken))
	if tokenEnv == "" {
		tokenEnv = strings.TrimSpace(os.Getenv(config.EnvTranscribeToken))
	}
	tokenExplicit := (tokenFlag != nil && tokenFlag.Changed) || tokenEnv != ""
	if !tokenExplicit {
		defaultToken := ""
		if appConfig != nil {
			defaultToken = appConfig.AccessToken
		}
		return promptInput(r, cmd.OutOrStdout(), "Access Token: ", defaultToken)
	}

	targetToken := overrideToken
	if targetToken == "" && appConfig != nil {
		targetToken = appConfig.AccessToken
	}
	return targetToken
}

func newLoginCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "login",
		Short: "Authenticate and configure connection to the transcribe controller",
		RunE: func(cmd *cobra.Command, _ []string) error {
			inReader := bufio.NewReader(cmd.InOrStdin())
			targetURL := strings.TrimRight(strings.TrimSpace(resolveLoginURL(cmd, inReader)), "/")
			if targetURL == "" {
				targetURL = config.DefaultControllerURL
			}
			targetToken := strings.TrimSpace(resolveLoginToken(cmd, inReader))

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
}
