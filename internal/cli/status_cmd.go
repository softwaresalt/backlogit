package cli

import (
	"context"
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/backlogit/backlogit/internal/core"
	"github.com/backlogit/backlogit/internal/db"
)

// newStatusCommand creates the `backlogit status` command.
func newStatusCommand(cwd *string) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show workspace artifact summary",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := context.Background()
			ws, err := core.NewWorkspace(ctx, *cwd)
			if err != nil {
				return fmt.Errorf("open workspace: %w", err)
			}
			defer ws.Close()

			results, err := db.ExecuteGatedQuery(ws.DB,
				`SELECT artifact_type, status, COUNT(*) as count FROM items GROUP BY artifact_type, status ORDER BY artifact_type, status`)
			if err != nil {
				return fmt.Errorf("query status: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Workspace: %s\n\n", ws.RootPath)
			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No artifacts indexed.")
				return nil
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "TYPE\tSTATUS\tCOUNT")
			for _, row := range results {
				fmt.Fprintf(w, "%v\t%v\t%v\n", row["artifact_type"], row["status"], row["count"])
			}
			return w.Flush()
		},
	}
}
