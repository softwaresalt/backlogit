package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"

	"github.com/softwaresalt/backlogit/internal/core"
)

// newAdoptCommand returns the `backlogit adopt` command.
func newAdoptCommand(cwd *string) *cobra.Command {
	var parentID string
	var keepID bool

	cmd := &cobra.Command{
		Use:   "adopt <item-id>",
		Short: "Adopt an orphaned item under a new parent feature",
		Example: `  backlogit adopt 015.009-T --parent 016-F
  backlogit adopt 015.009-T --parent 016-F --keep-id`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) (retErr error) {
			ctx := context.Background()
			slog.Info("adopt command invoked", "item_id", args[0], "new_parent_id", parentID)

			ws, err := core.NewWorkspace(ctx, *cwd)
			if err != nil {
				return fmt.Errorf("open workspace: %w", err)
			}
			defer func() {
				if cerr := ws.Close(); cerr != nil {
					retErr = errors.Join(retErr, fmt.Errorf("close workspace: %w", cerr))
				}
			}()

			result, err := core.AdoptItemWithOptions(ctx, ws, args[0], parentID, core.AdoptItemOptions{KeepID: keepID})
			if err != nil {
				return fmt.Errorf("adopt item: %w", err)
			}

			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(result)
		},
	}
	cmd.Flags().StringVar(&parentID, "parent", "", "New parent feature ID (required)")
	cmd.Flags().BoolVar(&keepID, "keep-id", false, "Re-parent without renumbering the artifact ID or renaming files")
	_ = cmd.MarkFlagRequired("parent")
	return cmd
}
