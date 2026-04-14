package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/softwaresalt/backlogit/internal/core"
)

// splitCSV splits s on commas, trims whitespace from each entry, and drops empty entries.
func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// newAddCommand creates the `backlogit add` command.
func newAddCommand(cwd *string) *cobra.Command {
	var (
		artifactType string
		title        string
		description  string
		status       string
		parentID     string
		sections     []string
		priority     string
		sprint       string
		assignedTo   string
		owner        string
		labels       string
		dependencies string
		references   string
		commit       string
	)

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Create a new artifact",
		Long: `Create a new backlogit artifact in the current workspace.

Artifacts are written as Markdown files under .backlogit\queue or the target
directory selected by registry routing. Typed hierarchical IDs are assigned
automatically when the configured queue layout supports the requested type.`,
		Example: `  backlogit add --type feature --title "Authentication hardening"
  backlogit add --type task --title "Add token rotation" --parent 001-F
  backlogit add --type subtask --title "Write expiry tests" --parent 001.001-T --section description="Cover refresh and expiry flows"`,
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
			if parentID != "" {
				opts = append(opts, core.WithParent(parentID))
			}
			if priority != "" {
				opts = append(opts, core.WithPriority(priority))
			}
			if sprint != "" {
				opts = append(opts, core.WithSprint(sprint))
			}
			if assignedTo != "" {
				opts = append(opts, core.WithAssignedTo(assignedTo))
			}
			if owner != "" {
				opts = append(opts, core.WithOwner(owner))
			}
			if labels != "" {
				opts = append(opts, core.WithLabels(splitCSV(labels)))
			}
			if dependencies != "" {
				opts = append(opts, core.WithDependencies(splitCSV(dependencies)))
			}
			if references != "" {
				opts = append(opts, core.WithReferences(splitCSV(references)))
			}
			if commit != "" {
				opts = append(opts, core.WithCommit(commit))
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

	cmd.Flags().StringVar(&artifactType, "type", "", "artifact type (feature, task, subtask, …)")
	cmd.Flags().StringVar(&title, "title", "", "artifact title")
	cmd.Flags().StringVar(&description, "description", "", "artifact description")
	cmd.Flags().StringVar(&status, "status", "", "initial status (queued, active, …)")
	cmd.Flags().StringVar(&parentID, "parent", "", "parent artifact ID (required for level-2+ types such as task, review)")
	cmd.Flags().StringArrayVar(&sections, "section", nil, "section content as name=value (repeatable)")
	cmd.Flags().StringVar(&priority, "priority", "", "priority (low, medium, high, critical)")
	cmd.Flags().StringVar(&sprint, "sprint", "", "sprint ID")
	cmd.Flags().StringVar(&assignedTo, "assigned-to", "", "assignee")
	cmd.Flags().StringVar(&owner, "owner", "", "owner")
	cmd.Flags().StringVar(&labels, "labels", "", "comma-separated labels")
	cmd.Flags().StringVar(&dependencies, "dependencies", "", "comma-separated dependency IDs")
	cmd.Flags().StringVar(&references, "references", "", "comma-separated reference paths")
	cmd.Flags().StringVar(&commit, "commit", "", "commit SHA")
	return cmd
}
