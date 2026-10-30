package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"

	"github.com/softwaresalt/backlogit/internal/core"
)

// shipmentManifestFlags holds the flags shared by the manifest re-plan commands.
type shipmentManifestFlags struct {
	reason string
	force  bool
	dryRun bool
}

func (f *shipmentManifestFlags) bind(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.reason, "reason", "", "operator justification (required with --force)")
	cmd.Flags().BoolVar(&f.force, "force", false, "override the queued-only guard (requires --reason)")
	cmd.Flags().BoolVar(&f.dryRun, "dry-run", false, "preview the change without writing")
}

func (f *shipmentManifestFlags) options() core.ShipmentManifestOptions {
	return core.ShipmentManifestOptions{Force: f.force, Reason: f.reason, DryRun: f.dryRun}
}

// runShipmentManifestOp opens the workspace, runs op, and encodes the result as
// indented JSON, isomorphic to the corresponding MCP tool result.
func runShipmentManifestOp(
	cmd *cobra.Command,
	operation string,
	op func(ctx context.Context, ws *core.Workspace) (any, error),
) (retErr error) {
	ctx := context.Background()
	slog.Info("shipment command invoked", "operation", operation)

	ws, err := core.NewWorkspace(ctx, shipmentCWD(cmd))
	if err != nil {
		return fmt.Errorf("open workspace: %w", err)
	}
	defer func() {
		if cerr := ws.Close(); cerr != nil {
			retErr = errors.Join(retErr, fmt.Errorf("close workspace: %w", cerr))
		}
	}()

	result, err := op(ctx, ws)
	if err != nil {
		return fmt.Errorf("%s: %w", operation, err)
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

// newShipmentRemoveCmd returns the `backlogit shipment remove` subcommand, the
// CLI fallback for the backlogit_remove_from_shipment MCP tool.
func newShipmentRemoveCmd() *cobra.Command {
	var flags shipmentManifestFlags
	cmd := &cobra.Command{
		Use:   "remove <shipment-id> <item-id>",
		Short: "Remove an item from a shipment manifest",
		Long: `Remove a backlog item from a queued shipment manifest.

This mirrors the backlogit_remove_from_shipment MCP tool. Only queued shipments
may be changed unless --force is supplied together with --reason; a blocked
shipment is never modifiable. Use --dry-run to preview the resulting manifest.`,
		Example: `  backlogit shipment remove 001-S 001.001-T --reason "descope"
  backlogit shipment remove 001-S 001.001-T --dry-run`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runShipmentManifestOp(cmd, "shipment-remove", func(ctx context.Context, ws *core.Workspace) (any, error) {
				return core.RemoveFromShipment(ctx, ws, args[0], args[1], flags.options())
			})
		},
	}
	flags.bind(cmd)
	return cmd
}

// newShipmentMoveCmd returns the `backlogit shipment move` subcommand, the CLI
// fallback for the backlogit_move_shipment_item MCP tool.
func newShipmentMoveCmd() *cobra.Command {
	var flags shipmentManifestFlags
	var from, to string
	cmd := &cobra.Command{
		Use:   "move <item-id> --from <shipment-id> --to <shipment-id>",
		Short: "Move an item between shipment manifests",
		Long: `Atomically move a backlog item from one shipment manifest to another.

This mirrors the backlogit_move_shipment_item MCP tool. Both shipments must be
queued unless --force is supplied together with --reason; a blocked shipment is
never modifiable. Use --dry-run to preview both resulting manifests.`,
		Example: `  backlogit shipment move 001.001-T --from 001-S --to 002-S --reason "split scope"`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runShipmentManifestOp(cmd, "shipment-move", func(ctx context.Context, ws *core.Workspace) (any, error) {
				return core.MoveShipmentItem(ctx, ws, args[0], from, to, flags.options())
			})
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "source shipment ID")
	cmd.Flags().StringVar(&to, "to", "", "destination shipment ID")
	_ = cmd.MarkFlagRequired("from")
	_ = cmd.MarkFlagRequired("to")
	flags.bind(cmd)
	return cmd
}

// newShipmentReorderCmd returns the `backlogit shipment reorder` subcommand, the
// CLI fallback for the backlogit_reorder_shipment MCP tool.
func newShipmentReorderCmd() *cobra.Command {
	var flags shipmentManifestFlags
	var items string
	var auto bool
	cmd := &cobra.Command{
		Use:   "reorder <shipment-id>",
		Short: "Reorder a shipment manifest",
		Long: `Reorder a queued shipment manifest.

This mirrors the backlogit_reorder_shipment MCP tool. Supply exactly one of
--items (an explicit comma-separated order that must be a permutation of the
current members and respect dependencies) or --auto (dependency-respecting
order). Use --dry-run to preview the resulting order.`,
		Example: `  backlogit shipment reorder 001-S --auto --dry-run
  backlogit shipment reorder 001-S --items 001-F,001.001-T,001.002-T`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runShipmentManifestOp(cmd, "shipment-reorder", func(ctx context.Context, ws *core.Workspace) (any, error) {
				opts := flags.options()
				opts.Items = splitShipmentItems(items)
				opts.Auto = auto
				return core.ReorderShipment(ctx, ws, args[0], opts)
			})
		},
	}
	cmd.Flags().StringVar(&items, "items", "", "explicit comma-separated manifest order")
	cmd.Flags().BoolVar(&auto, "auto", false, "compute a dependency-respecting order")
	flags.bind(cmd)
	return cmd
}
