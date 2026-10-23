package cli

import (
	"context"
	"errors"
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
		Example: `  backlogit delete 001.001-T --force`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) (retErr error) {
			id := args[0]
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

			if !force {
				if _, werr := fmt.Fprintf(cmd.ErrOrStderr(), "Delete %s? Use --force to confirm.\n", id); werr != nil {
					return fmt.Errorf("write output: %w", werr)
				}
				return fmt.Errorf("use --force to delete %s", id)
			}

			if err := core.DeleteArtifact(ctx, ws, id); err != nil {
				return err
			}

			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Deleted %s\n", id); err != nil {
				return fmt.Errorf("write output: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "skip confirmation and delete immediately")
	return cmd
}
