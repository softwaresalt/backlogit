package cli

import (
	"context"
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/backlogit/backlogit/internal/core"
	"github.com/backlogit/backlogit/internal/db"
)

// newSearchCommand creates the `backlogit search` command.
func newSearchCommand(cwd *string) *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Full-text search across artifacts",
		Long: `Search the full-text index for matching artifacts.

Use this when you want quick keyword lookup without writing SQL.`,
		Example: `  backlogit search authentication
  backlogit search "token rotation" --limit 10`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := args[0]
			ctx := context.Background()
			ws, err := core.NewWorkspace(ctx, *cwd)
			if err != nil {
				return fmt.Errorf("open workspace: %w", err)
			}
			defer ws.Close()

			results, err := db.SearchItems(ctx, ws.DB, query, limit)
			if err != nil {
				return fmt.Errorf("search: %w", err)
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			for _, a := range results {
				fmt.Fprintf(w, "%s\t%s\t%s\n", a.ID, a.Title, a.Status)
			}
			return w.Flush()
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 20, "maximum number of results")
	return cmd
}
