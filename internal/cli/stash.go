package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/softwaresalt/backlogit/internal/cli/format"
	"github.com/softwaresalt/backlogit/internal/core"
)

// NewStashCmd creates the stash command group.
func NewStashCmd(cwd *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stash",
		Short: "Manage the deferred work stash",
		Long: `Manage deferred work in .backlogit\stash.jsonl.

Use the stash to capture ideas, issues, risks, and follow-up work that should
be planned later and harvested into formal work items when ready.`,
	}
	cmd.AddCommand(newStashAddCommand(cwd))
	cmd.AddCommand(newStashListCommand(cwd))
	cmd.AddCommand(newStashGetCommand(cwd))
	cmd.AddCommand(newStashEditCommand(cwd))
	cmd.AddCommand(newStashRemoveCommand(cwd))
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
	cmd.Flags().StringVar(&kind, "kind", "task", "stash item kind (feature, task, bug, epic, unknown)")
	cmd.Flags().StringVar(&priority, "priority", "medium", "stash priority (low, medium, high, critical)")
	return cmd
}

func newStashListCommand(cwd *string) *cobra.Command {
	var priority, kind, formatOutput string
	var groupByPriority bool
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"fetch-stash"},
		Short:   "List the current active stash entries",
		Example: `  backlogit stash list
  backlogit stash list --priority high
  backlogit stash list --kind feature
  backlogit stash list --group-by-priority`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := context.Background()
			ws, err := core.NewWorkspace(ctx, *cwd)
			if err != nil {
				return fmt.Errorf("open workspace: %w", err)
			}
			defer ws.Close()
			entries, err := core.FetchStash(ctx, ws, core.FetchStashOptions{
				Priority:        priority,
				Kind:            kind,
				GroupByPriority: groupByPriority,
			})
			if err != nil {
				return err
			}

			// groupByPriority produces a structured map that only makes sense as JSON.
			if !groupByPriority {
				if err := validateFormat(formatOutput, format.FormatTable, format.FormatJSON, format.FormatTile); err != nil {
					return err
				}
			}
			if groupByPriority || format.Format(formatOutput) == format.FormatJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(entries)
			}

			stashCols := []format.Column{
				{Key: "id", Header: "ID"},
				{Key: "priority", Header: "PRIORITY"},
				{Key: "kind", Header: "KIND"},
				{Key: "text", Header: "TEXT"},
			}
			rows := make([]map[string]any, len(entries.Entries))
			for i, e := range entries.Entries {
				rows[i] = map[string]any{
					"id":       e.ID,
					"priority": e.Priority,
					"kind":     e.Kind,
					"text":     e.Text,
				}
			}
			return newRenderer(formatOutput).Render(cmd.OutOrStdout(), stashCols, rows)
		},
	}
	cmd.Flags().StringVar(&priority, "priority", "", "filter stash entries by priority")
	cmd.Flags().StringVar(&kind, "kind", "", "filter stash entries by kind (feature, task, bug, epic, unknown)")
	cmd.Flags().BoolVar(&groupByPriority, "group-by-priority", false, "group stash entries by priority")
	cmd.Flags().StringVar(&formatOutput, "format", "json", "output format: table, json, tile")
	return cmd
}

func newStashGetCommand(cwd *string) *cobra.Command {
	return &cobra.Command{
		Use:     "get <stash-id>",
		Short:   "Get a stash entry by ID",
		Example: `  backlogit stash get ABCD1234`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			ws, err := core.NewWorkspace(ctx, *cwd)
			if err != nil {
				return fmt.Errorf("open workspace: %w", err)
			}
			defer ws.Close()
			entry, err := core.GetStashEntry(ctx, ws, args[0])
			if err != nil {
				return err
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(entry)
		},
	}
}

func newStashEditCommand(cwd *string) *cobra.Command {
	var text, kind, priority string
	cmd := &cobra.Command{
		Use:   "edit <stash-id>",
		Short: "Edit a stash entry's text, kind, or priority",
		Example: `  backlogit stash edit ABCD1234 --kind feature
  backlogit stash edit ABCD1234 --priority high
  backlogit stash edit ABCD1234 --text "Updated description"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if text == "" && kind == "" && priority == "" {
				return fmt.Errorf("at least one of --text, --kind, or --priority is required")
			}
			ctx := context.Background()
			ws, err := core.NewWorkspace(ctx, *cwd)
			if err != nil {
				return fmt.Errorf("open workspace: %w", err)
			}
			defer ws.Close()
			entry, err := core.EditStashEntry(ctx, ws, args[0], core.EditStashOptions{
				Text:     text,
				Kind:     kind,
				Priority: priority,
			})
			if err != nil {
				return err
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(entry)
		},
	}
	cmd.Flags().StringVar(&text, "text", "", "new stash item text")
	cmd.Flags().StringVar(&kind, "kind", "", "new stash item kind (feature, task, bug, epic, unknown)")
	cmd.Flags().StringVar(&priority, "priority", "", "new stash priority (low, medium, high, critical)")
	return cmd
}

func newStashRemoveCommand(cwd *string) *cobra.Command {
	return &cobra.Command{
		Use:     "remove <stash-id>",
		Short:   "Remove an active stash entry",
		Example: `  backlogit stash remove ABCD1234`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			ws, err := core.NewWorkspace(ctx, *cwd)
			if err != nil {
				return fmt.Errorf("open workspace: %w", err)
			}
			defer ws.Close()
			entry, err := core.RemoveStashEntry(ctx, ws, args[0])
			if err != nil {
				return err
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(map[string]any{
				"id":     entry.ID,
				"status": "removed",
			})
		},
	}
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
