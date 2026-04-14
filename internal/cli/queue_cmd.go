package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/softwaresalt/backlogit/internal/core"
)

// NewQueueCmd creates the `backlogit queue` command group for queue operations.
func NewQueueCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "queue",
		Short: "Manage the work queue",
		Long: `View and manipulate the indexed work queue.

Use queue view for grouped queue output, queue move to reorder items, and
queue bulk-status to update multiple items in one command.`,
	}
	cmd.AddCommand(NewQueueViewCmd())
	cmd.AddCommand(NewQueueMoveCmd())
	cmd.AddCommand(NewQueueBulkStatusCmd())
	return cmd
}

// NewQueueViewCmd creates `backlogit queue view` with filter/group/sort flags.
func NewQueueViewCmd() *cobra.Command {
	var artifactType, status, groupBy, sortBy string

	cmd := &cobra.Command{
		Use:   "view",
		Short: "View queue items",
		Example: `  backlogit queue view
  backlogit queue view --status active --group-by type
  backlogit queue view --sort priority`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			ws, err := core.NewWorkspace(ctx, ".")
			if err != nil {
				return fmt.Errorf("open workspace: %w", err)
			}
			defer ws.Close()

			filter := &core.QueueFilter{
				GroupBy: groupBy,
				SortBy:  sortBy,
			}
			if artifactType != "" {
				filter.Types = []string{artifactType}
			}
			if status != "" {
				filter.Statuses = []string{status}
			}
			view, err := core.QueryQueue(ctx, ws.DB, filter)
			if err != nil {
				return fmt.Errorf("query queue: %w", err)
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(view)
		},
	}
	cmd.Flags().StringVar(&artifactType, "type", "", "filter by artifact type")
	cmd.Flags().StringVar(&status, "status", "", "filter by status")
	cmd.Flags().StringVar(&groupBy, "group-by", "", "group output by field")
	cmd.Flags().StringVar(&sortBy, "sort", "priority", "sort by field")
	return cmd
}

// NewQueueMoveCmd creates `backlogit queue move <item-id> --position <N>`.
func NewQueueMoveCmd() *cobra.Command {
	var position int

	cmd := &cobra.Command{
		Use:     "move <item-id>",
		Short:   "Reorder an item in the queue",
		Example: `  backlogit queue move 001-T --position 1`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			ws, err := core.NewWorkspace(ctx, ".")
			if err != nil {
				return fmt.Errorf("open workspace: %w", err)
			}
			defer ws.Close()

			if err := core.MoveInQueue(ctx, ws.DB, args[0], position); err != nil {
				return fmt.Errorf("move in queue: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Moved %s to position %d\n", args[0], position)
			return nil
		},
	}
	cmd.Flags().IntVar(&position, "position", 0, "target position in the queue")
	return cmd
}

// NewQueueBulkStatusCmd creates `backlogit queue bulk-status --ids 001-T,002-T --status active`.
func NewQueueBulkStatusCmd() *cobra.Command {
	var ids, status string

	cmd := &cobra.Command{
		Use:     "bulk-status",
		Short:   "Update status for multiple items",
		Example: `  backlogit queue bulk-status --ids 001-T,002-T,003-T --status active`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if ids == "" {
				return fmt.Errorf("--ids is required")
			}
			if status == "" {
				return fmt.Errorf("--status is required")
			}
			ctx := context.Background()
			ws, err := core.NewWorkspace(ctx, ".")
			if err != nil {
				return fmt.Errorf("open workspace: %w", err)
			}
			defer ws.Close()

			itemIDs := strings.Split(ids, ",")
			result, err := core.BulkUpdateStatus(ctx, ws.DB, ws, itemIDs, status)
			if err != nil {
				return fmt.Errorf("bulk update status: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Updated %d items to status %q\n", result.Succeeded, status)
			if len(result.Failed) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "Failed to update %d items: %s\n", len(result.Failed), strings.Join(result.Failed, ", "))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&ids, "ids", "", "comma-separated list of item IDs")
	cmd.Flags().StringVar(&status, "status", "", "target status")
	return cmd
}
