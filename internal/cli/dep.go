package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/db"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
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
		Example: `  backlogit dep add 001.002-T 001.001-T
  backlogit dep add 010-T 002-F --type blocks`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			itemID := args[0]
			dependsOn := args[1]

			ctx := context.Background()
			ws, err := core.NewWorkspace(ctx, depCWD(cmd))
			if err != nil {
				return fmt.Errorf("open workspace: %w", err)
			}
			defer ws.Close()

			// Route shipment→shipment blocks edges through AddShipmentBlock so the
			// CLI cannot bypass endpoint validation for that edge shape.
			// Non-blocks dep_types and non-shipment endpoints use the generic path.
			if depType == "" || depType == "blocks" {
				itemArt, e1 := db.GetItem(ctx, ws.DB, itemID)
				if e1 != nil && !errors.Is(e1, blerrors.ErrNotFound) {
					// Propagate real DB errors; ErrNotFound (cache miss) falls through
					// to AddDependency which uses the filesystem-backed findArtifact.
					return fmt.Errorf("routing check for %s: %w", itemID, e1)
				}
				if e1 == nil && itemArt.ArtifactType == "shipment" {
					// Source is a shipment with a blocks edge: route through
					// AddShipmentBlock which validates both endpoints are shipments.
					if err := core.AddShipmentBlock(ctx, ws, itemID, dependsOn); err != nil {
						return err
					}
					fmt.Fprintf(cmd.OutOrStdout(), "Added dependency %s → %s\n", itemID, dependsOn)
					return nil
				}
			}

			if err := core.AddDependency(ctx, ws, itemID, dependsOn, depType); err != nil {
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
		Use:     "remove <item-id> <depends-on>",
		Short:   "Remove a dependency edge",
		Example: `  backlogit dep remove 001.002-T 001.001-T`,
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			itemID := args[0]
			dependsOn := args[1]

			ctx := context.Background()
			ws, err := core.NewWorkspace(ctx, depCWD(cmd))
			if err != nil {
				return fmt.Errorf("open workspace: %w", err)
			}
			defer ws.Close()

			if err := core.RemoveDependency(ctx, ws, itemID, dependsOn); err != nil {
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
		Example: `  backlogit dep list 001.002-T
  backlogit dep list 001.001-T --reverse`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			itemID := args[0]

			ctx := context.Background()
			ws, err := core.NewWorkspace(ctx, depCWD(cmd))
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

// depCWD reads the workspace root from the root command's persistent --cwd flag.
// Falls back to "." when the flag is absent or unset.
func depCWD(cmd *cobra.Command) string {
	if cmd == nil || cmd.Root() == nil {
		return "."
	}
	cwd, err := cmd.Root().PersistentFlags().GetString("cwd")
	if err != nil || cwd == "" {
		return "."
	}
	return cwd
}
