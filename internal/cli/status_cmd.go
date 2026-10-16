package cli

import (
	"context"
	"errors"
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/db"
)

// newStatusCommand creates the `backlogit status` command.
func newStatusCommand(cwd *string) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show workspace artifact summary",
		Long: `Show a workspace summary grouped by artifact type and status.

This is a quick health check for the current indexed backlog state.`,
		Example: `  backlogit status
  backlogit --cwd D:\Source\MyProject status`,
		RunE: func(cmd *cobra.Command, _ []string) (retErr error) {
			ctx := context.Background()
			ws, err := core.NewWorkspace(ctx, *cwd)
			if err != nil {
				return fmt.Errorf("open workspace: %w", err)
			}
			defer func() {
				if cerr := ws.Close(); cerr != nil {
					retErr = errors.Join(retErr, fmt.Errorf("close workspace: %w", cerr))
				}
			}()

			results, err := db.ExecuteGatedQuery(ws.DB,
				`SELECT artifact_type, status, COUNT(*) as count FROM items GROUP BY artifact_type, status ORDER BY artifact_type, status`)
			if err != nil {
				return fmt.Errorf("query status: %w", err)
			}

			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Workspace: %s\n\n", ws.RootPath); err != nil {
				return fmt.Errorf("write output: %w", err)
			}
			if len(results) == 0 {
				if _, err := fmt.Fprintln(cmd.OutOrStdout(), "No artifacts indexed."); err != nil {
					return fmt.Errorf("write output: %w", err)
				}
				return nil
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			if _, err := fmt.Fprintln(w, "TYPE\tSTATUS\tCOUNT"); err != nil {
				return fmt.Errorf("write output: %w", err)
			}
			for _, row := range results {
				if _, err := fmt.Fprintf(w, "%v\t%v\t%v\n", row["artifact_type"], row["status"], row["count"]); err != nil {
					return fmt.Errorf("write output: %w", err)
				}
			}
			return w.Flush()
		},
	}
}
