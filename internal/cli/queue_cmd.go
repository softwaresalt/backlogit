package cli

import (
	"github.com/spf13/cobra"
)

// NewQueueCmd creates the `backlogit queue` command group for queue operations.
//
// Worker: Create cobra command with subcommands: view, move, bulk-status.
// Wire each subcommand to the corresponding core.Queue* function.
func NewQueueCmd() *cobra.Command {
	panic("not implemented: Worker: Create 'queue' cobra command group with view/move/bulk-status subcommands")
}

// NewQueueViewCmd creates `backlogit queue view [--type task] [--status queued] [--group-by type] [--sort priority]`.
//
// Worker: Parse filter flags into a QueueFilter struct. Call core.QueryQueue.
// Format output as a table with columns: ID, Title, Status, Type, Priority.
// When --group-by is set, separate groups with headers.
func NewQueueViewCmd() *cobra.Command {
	panic("not implemented: Worker: Create 'queue view' command with filter/group/sort flags and table output")
}

// NewQueueMoveCmd creates `backlogit queue move <item-id> --position <N>`.
//
// Worker: Parse item-id from args and position from flag. Call core.MoveInQueue.
func NewQueueMoveCmd() *cobra.Command {
	panic("not implemented: Worker: Create 'queue move' command to reorder items within their parent")
}

// NewQueueBulkStatusCmd creates `backlogit queue bulk-status --ids T001,T002 --status active`.
//
// Worker: Parse item IDs (comma-separated) and target status. Call core.BulkUpdateStatus.
// Display count of updated items.
func NewQueueBulkStatusCmd() *cobra.Command {
	panic("not implemented: Worker: Create 'queue bulk-status' command for batch status transitions")
}
