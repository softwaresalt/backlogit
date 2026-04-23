package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/softwaresalt/backlogit/internal/core"
)

// newDeleteCommand creates the `backlogit delete` command.
func newDeleteCommand(cwd *string) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete an artifact",
		Long: `Delete an artifact from the workspace and remove it from the index.

This is a destructive operation and requires --force.`,
		Example: `  backlogit delete 001-T --force`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			ctx := context.Background()
			ws, err := core.NewWorkspace(ctx, *cwd)
			if err != nil {
				return fmt.Errorf("open workspace: %w", err)
			}
			defer ws.Close()

			if !force {
				fmt.Fprintf(cmd.ErrOrStderr(), "Delete %s? Use --force to confirm.\n", id)
				return fmt.Errorf("use --force to delete %s", id)
			}

			if err := core.DeleteArtifact(ctx, ws, id); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Deleted %s\n", id)
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "skip confirmation and delete immediately")
	return cmd
}
