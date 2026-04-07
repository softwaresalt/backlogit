package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/backlogit/backlogit/internal/core"
)

// NewStashCmd creates the stash command group.
func NewStashCmd(cwd *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stash",
		Short: "Manage the deferred work stash",
		Long: `Manage deferred work in .backlogit\queue\.stash.md.

Use the stash to capture ideas, issues, risks, and follow-up work that should
be planned later and harvested into formal work items when ready.`,
	}
	cmd.AddCommand(newStashAddCommand(cwd))
	cmd.AddCommand(newStashFetchCommand(cwd))
	cmd.AddCommand(newStashHarvestCommand(cwd))
	return cmd
}

func newStashAddCommand(cwd *string) *cobra.Command {
	var kind, priority string
	cmd := &cobra.Command{
		Use:   "add <text>",
		Short: "Add an item to the stash",
		Example: `  backlogit stash add "Investigate tenant-specific rate limits" --kind feature --priority high
  backlogit stash add "Document migration edge cases" --kind task`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			ws, err := core.NewWorkspace(ctx, *cwd)
			if err != nil {
				return fmt.Errorf("open workspace: %w", err)
			}
			defer ws.Close()

			entry, err := core.AddStashEntry(ctx, ws, kind, priority, args[0])
			if err != nil {
				return err
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(entry)
		},
	}
	cmd.Flags().StringVar(&kind, "kind", "task", "stash item kind (feature, task, bug, epic)")
	cmd.Flags().StringVar(&priority, "priority", "medium", "stash priority (low, medium, high, critical)")
	return cmd
}

func newStashFetchCommand(cwd *string) *cobra.Command {
	var priority string
	var groupByPriority bool
	cmd := &cobra.Command{
		Use:   "fetch-stash",
		Short: "Fetch the current active stash entries",
		Example: `  backlogit stash fetch-stash
  backlogit stash fetch-stash --priority high
  backlogit stash fetch-stash --group-by-priority`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := context.Background()
			ws, err := core.NewWorkspace(ctx, *cwd)
			if err != nil {
				return fmt.Errorf("open workspace: %w", err)
			}
			defer ws.Close()

			entries, err := core.FetchStash(ctx, ws, core.FetchStashOptions{
				Priority:        priority,
				GroupByPriority: groupByPriority,
			})
			if err != nil {
				return err
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(entries)
		},
	}
	cmd.Flags().StringVar(&priority, "priority", "", "filter stash entries by priority")
	cmd.Flags().BoolVar(&groupByPriority, "group-by-priority", false, "group stash entries by priority")
	return cmd
}

func newStashHarvestCommand(cwd *string) *cobra.Command {
	var artifactType, title, description, status, parentID, priority string
	cmd := &cobra.Command{
		Use:   "harvest [stash-id]",
		Short: "Harvest a stash item into a planned work item",
		Example: `  backlogit stash harvest ABCD1234 --type feature
  backlogit stash harvest ABCD1234 --type task --parent-id 001-F --status active
  backlogit stash harvest --priority critical --type task`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			ws, err := core.NewWorkspace(ctx, *cwd)
			if err != nil {
				return fmt.Errorf("open workspace: %w", err)
			}
			defer ws.Close()

			if len(args) == 0 && priority == "" {
				return fmt.Errorf("either stash-id or --priority is required")
			}
			if len(args) > 0 && priority != "" {
				return fmt.Errorf("use either stash-id or --priority, not both")
			}

			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			if priority != "" {
				result, err := core.HarvestStashByPriority(ctx, ws, core.HarvestStashOptions{
					Priority:     priority,
					ArtifactType: artifactType,
					Title:        title,
					Description:  description,
					Status:       status,
					ParentID:     parentID,
				})
				if err != nil {
					return err
				}
				return enc.Encode(result)
			}

			result, err := core.HarvestStashEntry(ctx, ws, core.HarvestStashOptions{
				StashID:      args[0],
				ArtifactType: artifactType,
				Title:        title,
				Description:  description,
				Status:       status,
				ParentID:     parentID,
			})
			if err != nil {
				return err
			}
			return enc.Encode(result)
		},
	}
	cmd.Flags().StringVar(&artifactType, "type", "task", "target artifact type (feature, task, subtask)")
	cmd.Flags().StringVar(&title, "title", "", "override title for the harvested work item")
	cmd.Flags().StringVar(&description, "description", "", "description for the harvested work item")
	cmd.Flags().StringVar(&status, "status", "queued", "status for the harvested work item")
	cmd.Flags().StringVar(&parentID, "parent-id", "", "optional parent work item ID")
	cmd.Flags().StringVar(&priority, "priority", "", "harvest all stash entries at the given priority")
	return cmd
}
