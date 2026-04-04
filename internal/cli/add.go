package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/backlogit/backlogit/internal/core"
	"github.com/backlogit/backlogit/internal/db"
)

// newAddCommand creates the `backlogit add` command.
func newAddCommand(cwd *string) *cobra.Command {
	var (
		artifactType string
		title        string
		description  string
		status       string
		sections     []string
	)

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Create a new artifact",
		Long: `Create a new backlogit artifact in the current workspace.

Artifacts are written as Markdown files under .backlogit\queue or the target
directory selected by registry routing. Typed hierarchical IDs are assigned
automatically when the configured queue layout supports the requested type.`,
		Example: `  backlogit add --type feature --title "Authentication hardening"
  backlogit add --type task --title "Add token rotation" --status active
  backlogit add --type subtask --title "Write expiry tests" --section description="Cover refresh and expiry flows"`,
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
			if status != "" {
				opts = append(opts, core.WithStatus(status))
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
			if err := db.UpsertItem(ctx, ws.DB, artifact); err != nil {
				return fmt.Errorf("index artifact: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created %s: %s\n", artifact.ArtifactType, artifact.ID)
			return nil
		},
	}

	cmd.Flags().StringVar(&artifactType, "type", "", "artifact type (feature, task, subtask, …)")
	cmd.Flags().StringVar(&title, "title", "", "artifact title")
	cmd.Flags().StringVar(&description, "description", "", "artifact description")
	cmd.Flags().StringVar(&status, "status", "", "initial status (queued, active, …)")
	cmd.Flags().StringArrayVar(&sections, "section", nil, "section content as name=value (repeatable)")
	return cmd
}
