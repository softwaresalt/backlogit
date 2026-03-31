package cli

import (
	"github.com/spf13/cobra"
)

// NewQueueCmd creates the `backlogit queue` command group for queue operations.
func NewQueueCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "queue",
		Short: "Manage the work queue",
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
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
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
		Use:   "move <item-id>",
		Short: "Reorder an item in the queue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}
	cmd.Flags().IntVar(&position, "position", 0, "target position in the queue")
	return cmd
}

// NewQueueBulkStatusCmd creates `backlogit queue bulk-status --ids T001,T002 --status active`.
func NewQueueBulkStatusCmd() *cobra.Command {
	var ids, status string

	cmd := &cobra.Command{
		Use:   "bulk-status",
		Short: "Update status for multiple items",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}
	cmd.Flags().StringVar(&ids, "ids", "", "comma-separated list of item IDs")
	cmd.Flags().StringVar(&status, "status", "", "target status")
	return cmd
}
