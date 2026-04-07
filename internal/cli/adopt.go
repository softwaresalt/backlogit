package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"

	"github.com/backlogit/backlogit/internal/core"
)

// newAdoptCommand returns the `backlogit adopt` command.
func newAdoptCommand(cwd *string) *cobra.Command {
	var parentID string

	cmd := &cobra.Command{
		Use:     "adopt <item-id>",
		Short:   "Adopt an orphaned item under a new parent feature",
		Example: `  backlogit adopt 015.009-T --parent 016-F`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			slog.Info("adopt command invoked", "item_id", args[0], "new_parent_id", parentID)

			ws, err := core.NewWorkspace(ctx, *cwd)
			if err != nil {
				return fmt.Errorf("open workspace: %w", err)
			}
			defer ws.Close()

			result, err := core.AdoptItem(ctx, ws, args[0], parentID)
			if err != nil {
				return fmt.Errorf("adopt item: %w", err)
			}

			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(result)
		},
	}
	cmd.Flags().StringVar(&parentID, "parent", "", "New parent feature ID (required)")
	_ = cmd.MarkFlagRequired("parent")
	return cmd
}
