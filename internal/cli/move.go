package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/backlogit/backlogit/internal/core"
	"github.com/backlogit/backlogit/internal/db"
)

// newMoveCommand creates the `backlogit move` command.
func newMoveCommand(cwd *string) *cobra.Command {
	var status string

	cmd := &cobra.Command{
		Use:   "move <id>",
		Short: "Change artifact status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if status == "" {
				return fmt.Errorf("--status is required")
			}

			id := args[0]
			ctx := context.Background()
			ws, err := core.NewWorkspace(ctx, *cwd)
			if err != nil {
				return fmt.Errorf("open workspace: %w", err)
			}
			defer ws.Close()

			artifact, err := core.UpdateArtifact(ctx, ws, id, map[string]any{"status": status})
			if err != nil {
				return err
			}

			// Relocate the file to the directory mapped by the new status.
			newPath, err := core.RelocateArtifactFile(ctx, ws, artifact.ArtifactType, id, status)
			if err != nil {
				return fmt.Errorf("relocate artifact: %w", err)
			}

			if err := core.WriteArtifactFile(artifact, newPath); err != nil {
				return err
			}
			if err := db.UpsertItem(ctx, ws.DB, artifact); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Moved %s → %s\n", id, status)
			return nil
		},
	}

	cmd.Flags().StringVar(&status, "status", "", "new status (required)")
	return cmd
}
