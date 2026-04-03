package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/backlogit/backlogit/internal/core"
	"github.com/backlogit/backlogit/internal/db"
)

// newQueryCommand creates the `backlogit query` command.
func newQueryCommand(cwd *string) *cobra.Command {
	return &cobra.Command{
		Use:   "query \"<sql>\"",
		Short: "Execute a read-only SQL query against the index",
		Long: `Execute a gated read-only SQL query against the backlogit SQLite cache.

Only SELECT statements are allowed. Use this for token-efficient inspection of
items, dependencies, logs, and indexed stash data.`,
		Example: `  backlogit query "SELECT id, title, status FROM items ORDER BY updated_at DESC LIMIT 20"
  backlogit query "SELECT stash_id, kind, state FROM stash_entries ORDER BY updated_at DESC"`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sqlStr := args[0]
			ctx := context.Background()
			ws, err := core.NewWorkspace(ctx, *cwd)
			if err != nil {
				return fmt.Errorf("open workspace: %w", err)
			}
			defer ws.Close()

			results, err := db.ExecuteGatedQuery(ws.DB, sqlStr)
			if err != nil {
				return err
			}

			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(results)
		},
	}
}
