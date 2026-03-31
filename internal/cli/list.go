package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/backlogit/backlogit/internal/core"
	"github.com/backlogit/backlogit/internal/db"
)

// newListCommand creates the `backlogit list` command.
func newListCommand(cwd *string) *cobra.Command {
	var (
		filterType       string
		filterStatus     string
		filterAssignedTo string
		filterSprint     string
		jsonOutput       bool
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List artifacts in the workspace",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := context.Background()
			ws, err := core.NewWorkspace(ctx, *cwd)
			if err != nil {
				return fmt.Errorf("open workspace: %w", err)
			}
			defer ws.Close()

			artifacts, err := db.QueryItems(ctx, ws.DB, db.QueryFilters{
				Type:       filterType,
				Status:     filterStatus,
				AssignedTo: filterAssignedTo,
				Sprint:     filterSprint,
			})
			if err != nil {
				return fmt.Errorf("query items: %w", err)
			}

			if jsonOutput {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(artifacts)
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tTITLE\tSTATUS\tTYPE\tPRIORITY")
			for _, a := range artifacts {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
					a.ID, a.Title, a.Status, a.ArtifactType, a.Priority)
			}
			return w.Flush()
		},
	}

	cmd.Flags().StringVar(&filterType, "type", "", "filter by artifact type")
	cmd.Flags().StringVar(&filterStatus, "status", "", "filter by status")
	cmd.Flags().StringVar(&filterAssignedTo, "assigned-to", "", "filter by assignee")
	cmd.Flags().StringVar(&filterSprint, "sprint", "", "filter by sprint ID")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output as JSON array")
	return cmd
}
