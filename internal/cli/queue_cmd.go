package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/softwaresalt/backlogit/internal/cli/format"
	"github.com/softwaresalt/backlogit/internal/core"
)

const defaultQueueSort = "priority"

var defaultQueueStatuses = []string{"queued", "active", "blocked", "review"}

// NewQueueCmd creates the `backlogit queue` command group for queue operations.
func NewQueueCmd() *cobra.Command {
	cwd := "."
	return newQueueCmd(&cwd)
}

func newQueueCmd(cwd *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "queue",
		Short: "Manage the work queue",
		Long: `View and manipulate the indexed work queue.

Use queue view for grouped queue output, queue move to reorder items, and
queue bulk-status to update multiple items in one command.`,
	}
	cmd.AddCommand(newQueueViewCmd(cwd))
	cmd.AddCommand(newQueueMoveCmd(cwd))
	cmd.AddCommand(newQueueBulkStatusCmd(cwd))
	return cmd
}

// NewQueueViewCmd creates `backlogit queue view` with filter/group/sort flags.
func NewQueueViewCmd() *cobra.Command {
	cwd := "."
	return newQueueViewCmd(&cwd)
}

func newQueueViewCmd(cwd *string) *cobra.Command {
	var artifactType, status, groupBy, sortBy, formatOutput string

	cmd := &cobra.Command{
		Use:   "view",
		Short: "View queue items",
		Long: `View active work queue items.

By default, queue view shows queued, active, blocked, and review items using
priority as the secondary sort after any manually assigned queue positions.`,
		Example: `  backlogit queue view
  backlogit queue view --status active --group-by type
  backlogit queue view --sort priority`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			ws, err := core.NewWorkspace(ctx, *cwd)
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
			} else {
				filter.Statuses = append([]string(nil), defaultQueueStatuses...)
			}
			view, err := core.QueryQueue(ctx, ws.DB, filter)
			if err != nil {
				return fmt.Errorf("query queue: %w", err)
			}

			effectiveFormat := format.Format(formatOutput)
			if err := validateFormat(effectiveFormat, format.FormatTable, format.FormatJSON, format.FormatTile); err != nil {
				return err
			}
			switch effectiveFormat {
			case format.FormatTable, format.FormatTile:
				return newRenderer(effectiveFormat, cmd.OutOrStdout()).Render(cmd.OutOrStdout(), artifactColumns, artifactsToRows(ctx, ws, view.Items))
			default: // json
				// Route through the shared core shaper so `queue view --json`
				// projects the computed-on-read size_composition rollup onto each
				// aggregate at parity with MCP get_queue, and degrades to an
				// unprojected payload on rollup error rather than aborting
				// (114-F / 387DE4BF; 117-F / A6A1B47E).
				payload, err := core.QueueViewWithSizeComposition(ctx, ws, view)
				if err != nil {
					return err
				}
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(payload)
			}
		},
	}
	cmd.Flags().StringVar(&artifactType, "type", "", "filter by artifact type")
	cmd.Flags().StringVar(&status, "status", "", "filter by status")
	cmd.Flags().StringVar(&groupBy, "group-by", "", "group output by field")
	cmd.Flags().StringVar(&sortBy, "sort", "priority", "sort by field")
	cmd.Flags().StringVar(&formatOutput, "format", "table", "output format: table, json, tile")
	return cmd
}

// NewQueueMoveCmd creates `backlogit queue move <item-id> --position <N>`.
func NewQueueMoveCmd() *cobra.Command {
	cwd := "."
	return newQueueMoveCmd(&cwd)
}

func newQueueMoveCmd(cwd *string) *cobra.Command {
	var position int

	cmd := &cobra.Command{
		Use:   "move <item-id>",
		Short: "Reorder an item in the queue",
		Long: `Reorder an item within the default active queue view.

Positions are 1-based and use the same default scope as queue view: queued,
active, blocked, and review items.`,
		Example: `  backlogit queue move 001.001-T --position 1`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			ws, err := core.NewWorkspace(ctx, *cwd)
			if err != nil {
				return fmt.Errorf("open workspace: %w", err)
			}
			defer ws.Close()

			filter := &core.QueueFilter{
				Statuses: append([]string(nil), defaultQueueStatuses...),
				SortBy:   defaultQueueSort,
			}
			if err := core.MoveInQueue(ctx, ws, args[0], position, filter); err != nil {
				return fmt.Errorf("move in queue: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Moved %s to position %d\n", args[0], position)
			return nil
		},
	}
	cmd.Flags().IntVar(&position, "position", 0, "target position in the queue")
	return cmd
}

// NewQueueBulkStatusCmd creates `backlogit queue bulk-status --ids 001.001-T,001.002-T --status active`.
func NewQueueBulkStatusCmd() *cobra.Command {
	cwd := "."
	return newQueueBulkStatusCmd(&cwd)
}

func newQueueBulkStatusCmd(cwd *string) *cobra.Command {
	var ids, status string

	cmd := &cobra.Command{
		Use:     "bulk-status",
		Short:   "Update status for multiple items",
		Example: `  backlogit queue bulk-status --ids 001.001-T,001.002-T,001.003-T --status active`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if ids == "" {
				return fmt.Errorf("--ids is required")
			}
			if status == "" {
				return fmt.Errorf("--status is required")
			}
			ctx := context.Background()
			ws, err := core.NewWorkspace(ctx, *cwd)
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
