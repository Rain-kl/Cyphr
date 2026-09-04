// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"encoding/json"
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var profileJSON bool

// NewProfileCmd creates and returns the profile command.
func NewProfileCmd() *cobra.Command {
	profileCmd := &cobra.Command{
		Use:     "profile",
		Aliases: []string{"whoami", "me"},
		Short:   "Display current user profile and account details",
		RunE: func(cmd *cobra.Command, _ []string) error {
			profile, err := appClient.GetProfile(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to get user profile: %w", err)
			}

			if profileJSON {
				out, jerr := json.MarshalIndent(profile, "", "  ")
				if jerr != nil {
					return fmt.Errorf("failed to format profile JSON: %w", jerr)
				}
				cmd.Println(string(out))
				return nil
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, defaultTabPadding, ' ', 0)
			_, _ = fmt.Fprintf(w, "User ID:\t%d\n", profile.ID)
			_, _ = fmt.Fprintf(w, "Username:\t%s\n", profile.Username)
			if profile.Nickname != "" {
				_, _ = fmt.Fprintf(w, "Nickname:\t%s\n", profile.Nickname)
			}
			if profile.Email != "" {
				_, _ = fmt.Fprintf(w, "Email:\t%s\n", profile.Email)
			}
			role := "Standard User"
			if profile.IsAdmin {
				role = "Administrator"
			}
			_, _ = fmt.Fprintf(w, "Role:\t%s\n", role)
			if profile.Phone != "" {
				_, _ = fmt.Fprintf(w, "Phone:\t%s\n", profile.Phone)
			}
			if profile.Location != "" {
				_, _ = fmt.Fprintf(w, "Location:\t%s\n", profile.Location)
			}
			if profile.Website != "" {
				_, _ = fmt.Fprintf(w, "Website:\t%s\n", profile.Website)
			}
			if profile.Bio != "" {
				_, _ = fmt.Fprintf(w, "Bio:\t%s\n", profile.Bio)
			}
			if appConfig != nil && appConfig.ControllerURL != "" {
				_, _ = fmt.Fprintf(w, "Server URL:\t%s\n", appConfig.ControllerURL)
			}
			_ = w.Flush()

			return nil
		},
	}

	profileCmd.Flags().BoolVar(&profileJSON, "json", false, "output profile in JSON format")

	return profileCmd
}
