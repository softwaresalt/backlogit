package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/backlogit/backlogit/internal/core"
	"github.com/backlogit/backlogit/internal/db"
)

// NewDepCmd creates the `backlogit dep` command group for managing dependencies.
func NewDepCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dep",
		Short: "Manage artifact dependencies",
		Long: `Manage explicit dependency edges between work items.

Dependencies are stored in the backlogit index and can be queried from both the
CLI and MCP tools.`,
	}
	cmd.AddCommand(NewDepAddCmd())
	cmd.AddCommand(NewDepRemoveCmd())
	cmd.AddCommand(NewDepListCmd())
	return cmd
}

// NewDepAddCmd creates `backlogit dep add <item-id> <depends-on> [--type blocks]`.
func NewDepAddCmd() *cobra.Command {
	var depType string

	cmd := &cobra.Command{
		Use:   "add <item-id> <depends-on>",
		Short: "Add a dependency edge",
		Example: `  backlogit dep add T002 T001
  backlogit dep add T010 F002 --type blocks`,
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			itemID := args[0]
			dependsOn := args[1]

			ctx := context.Background()
			cwd := "."
			ws, err := core.NewWorkspace(ctx, cwd)
			if err != nil {
				return fmt.Errorf("open workspace: %w", err)
			}
			defer ws.Close()

			if err := db.AddDependencyChecked(ctx, ws.DB, itemID, dependsOn, depType); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Added dependency %s → %s\n", itemID, dependsOn)
			return nil
		},
	}
	cmd.Flags().StringVar(&depType, "type", "blocks", "dependency relationship type")
	return cmd
}

// NewDepRemoveCmd creates `backlogit dep remove <item-id> <depends-on>`.
func NewDepRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <item-id> <depends-on>",
		Short: "Remove a dependency edge",
		Example: `  backlogit dep remove T002 T001`,
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			itemID := args[0]
			dependsOn := args[1]

			ctx := context.Background()
			cwd := "."
			ws, err := core.NewWorkspace(ctx, cwd)
			if err != nil {
				return fmt.Errorf("open workspace: %w", err)
			}
			defer ws.Close()

			if err := db.DeleteDependency(ctx, ws.DB, itemID, dependsOn); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed dependency %s → %s\n", itemID, dependsOn)
			return nil
		},
	}
}

// NewDepListCmd creates `backlogit dep list <item-id> [--reverse]`.
func NewDepListCmd() *cobra.Command {
	var reverse bool

	cmd := &cobra.Command{
		Use:   "list <item-id>",
		Short: "List dependencies for an artifact",
		Example: `  backlogit dep list T002
  backlogit dep list T001 --reverse`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			itemID := args[0]

			ctx := context.Background()
			cwd := "."
			ws, err := core.NewWorkspace(ctx, cwd)
			if err != nil {
				return fmt.Errorf("open workspace: %w", err)
			}
			defer ws.Close()

			var edges []db.DependencyEdge
			if reverse {
				edges, err = db.GetDependents(ctx, ws.DB, itemID)
			} else {
				edges, err = db.GetDependencies(ctx, ws.DB, itemID)
			}
			if err != nil {
				return err
			}
			for _, e := range edges {
				fmt.Fprintf(cmd.OutOrStdout(), "%s → %s (%s)\n", e.ItemID, e.DependsOn, e.DepType)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&reverse, "reverse", false, "show items that depend on this item")
	return cmd
}
