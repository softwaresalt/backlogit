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
		filterPriority   string
		filterAssignedTo string
		filterOwner      string
		filterSprint     string
		groupBy          string
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
				Priority:   filterPriority,
				AssignedTo: filterAssignedTo,
				Owner:      filterOwner,
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

			if groupBy != "" {
				items := make([]ListItem, len(artifacts))
				for i, a := range artifacts {
					items[i] = ListItem{
						ID:       a.ID,
						Title:    a.Title,
						Status:   string(a.Status),
						Type:     a.ArtifactType,
						ParentID: a.ParentID,
						Priority: a.Priority,
					}
				}
				fmt.Fprint(cmd.OutOrStdout(), FormatGroupedView(items, groupBy))
				return nil
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
	cmd.Flags().StringVar(&filterPriority, "priority", "", "filter by priority")
	cmd.Flags().StringVar(&filterAssignedTo, "assigned-to", "", "filter by assignee")
	cmd.Flags().StringVar(&filterOwner, "owner", "", "filter by owner")
	cmd.Flags().StringVar(&filterSprint, "sprint", "", "filter by sprint ID")
	cmd.Flags().StringVar(&groupBy, "group-by", "", "group output by field (status, type, priority)")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output as JSON array")
	return cmd
}
