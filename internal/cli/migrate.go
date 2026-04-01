package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/backlogit/backlogit/internal/core"
	"github.com/backlogit/backlogit/internal/db"
)

func newMigrateCommand(cwd *string) *cobra.Command {
	var dryRun, rollback bool

	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Migrate flat layout to hierarchical queue structure",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := context.Background()
			ws, err := core.NewWorkspace(ctx, *cwd)
			if err != nil {
				return fmt.Errorf("open workspace: %w", err)
			}
			defer ws.Close()

			if rollback {
				if err := core.RollbackMigration(ws); err != nil {
					return fmt.Errorf("rollback: %w", err)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "Migration rolled back")
				return nil
			}

			report, err := core.MigrateFlatToHierarchical(ws, dryRun)
			if err != nil {
				return fmt.Errorf("migrate: %w", err)
			}

			if dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "Dry run: %d files would move, %d skipped\n",
					report.FilesMoved, report.FilesSkipped)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Migrated %d files, %d skipped\n",
					report.FilesMoved, report.FilesSkipped)
				count, rehydErr := db.Rehydrate(ctx, ws.RootPath, ws.DB)
				if rehydErr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "Warning: rehydration failed: %v\n", rehydErr)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "Rehydrated %d artifacts\n", count)
				}
			}

			for _, e := range report.Errors {
				fmt.Fprintf(cmd.ErrOrStderr(), "Error: %s\n", e)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "preview changes without moving files")
	cmd.Flags().BoolVar(&rollback, "rollback", false, "reverse a previous migration")
	return cmd
}
