// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

// NewModelsCmd creates and returns the models command.
func NewModelsCmd() *cobra.Command {
	modelsCmd := &cobra.Command{
		Use:     "models",
		Aliases: []string{"model"},
		Short:   "List available transcription models",
		RunE: func(cmd *cobra.Command, _ []string) error {
			models, err := appClient.ListModels(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to list models: %w", err)
			}

			if len(models) == 0 {
				cmd.Println("No models available.")
				return nil
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, defaultTabPadding, ' ', 0)
			_, _ = fmt.Fprintln(w, "MODEL\tTASK TYPE\tSTATUS\tDESCRIPTION")
			for _, m := range models {
				statusStr := "inactive"
				if m.IsActive {
					statusStr = "active"
				}
				desc := m.Description
				if desc == "" {
					desc = "-"
				}
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
					m.Name, m.TaskType, statusStr, desc)
			}
			_ = w.Flush()

			cmd.Printf("\nTotal %d model(s) available\n", len(models))
			return nil
		},
	}

	return modelsCmd
}
