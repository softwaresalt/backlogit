package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/backlogit/backlogit/internal/core"
)

func newArchiveCommand(cwd *string) *cobra.Command {
	var allDone bool

	cmd := &cobra.Command{
		Use:   "archive <id>",
		Short: "Archive a completed artifact",
		Long: `Archive one completed artifact or bulk-archive all terminal artifacts.

Archived items are moved into .backlogit\archive and tracked in the index.`,
		Example: `  backlogit archive T001
  backlogit archive --all-done`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			ws, err := core.NewWorkspace(ctx, *cwd)
			if err != nil {
				return fmt.Errorf("open workspace: %w", err)
			}
			defer ws.Close()

			if allDone {
				policy := &core.ArchivePolicy{
					TerminalStatuses: []string{"done", "accepted", "rejected"},
					RetentionDays:    0,
				}
				count, err := core.AutoArchive(ctx, ws.DB, ws, policy)
				if err != nil {
					return fmt.Errorf("auto-archive: %w", err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Archived %d items\n", count)
				return nil
			}

			if len(args) == 0 {
				return fmt.Errorf("item ID required (or use --all-done)")
			}
			record, err := core.ArchiveItem(ctx, ws.DB, ws, args[0])
			if err != nil {
				return fmt.Errorf("archive: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Archived %s → %s\n", record.ID, record.ArchivePath)
			return nil
		},
	}
	cmd.Flags().BoolVar(&allDone, "all-done", false, "archive all items with terminal status")
	return cmd
}
