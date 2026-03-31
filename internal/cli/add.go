package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/backlogit/backlogit/internal/core"
)

// newAddCommand creates the `backlogit add` command.
func newAddCommand(cwd *string) *cobra.Command {
	var (
		artifactType string
		title        string
		description  string
		sections     []string
	)

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Create a new artifact",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if artifactType == "" {
				return fmt.Errorf("--type is required")
			}
			if title == "" {
				return fmt.Errorf("--title is required")
			}

			ctx := context.Background()
			ws, err := core.NewWorkspace(ctx, *cwd)
			if err != nil {
				return fmt.Errorf("open workspace: %w", err)
			}
			defer ws.Close()

			opts := []core.Option{}
			if description != "" {
				opts = append(opts, core.WithDescription(description))
			}

			// Apply --section name=value pairs as description overrides when applicable.
			for _, sec := range sections {
				name, value, found := strings.Cut(sec, "=")
				if !found {
					return fmt.Errorf("invalid --section format %q: expected name=value", sec)
				}
				if strings.ToLower(name) == "description" {
					opts = append(opts, core.WithDescription(value))
				}
			}

			artifact, err := core.CreateArtifact(ctx, ws, title, artifactType, opts...)
			if err != nil {
				return fmt.Errorf("create artifact: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created %s: %s\n", artifact.ArtifactType, artifact.ID)
			return nil
		},
	}

	cmd.Flags().StringVar(&artifactType, "type", "", "artifact type (task, bug, epic, …)")
	cmd.Flags().StringVar(&title, "title", "", "artifact title")
	cmd.Flags().StringVar(&description, "description", "", "artifact description")
	cmd.Flags().StringArrayVar(&sections, "section", nil, "section content as name=value (repeatable)")
	return cmd
}
