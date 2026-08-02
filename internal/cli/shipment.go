package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/spf13/cobra"

	"github.com/softwaresalt/backlogit/internal/cli/format"
	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/models"
)

// NewShipmentCmd returns the top-level `backlogit shipment` command group.
func NewShipmentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "shipment",
		Short: "Manage shipment work groups",
		Long: `Manage shipment artifacts that group related backlog items for delivery.

Use shipment commands to create a shipment, inspect its current state, list
shipments in the workspace, claim queued shipments, and return blocked items.`,
	}
	cmd.AddCommand(newShipmentCreateCmd())
	cmd.AddCommand(newShipmentGetCmd())
	cmd.AddCommand(newShipmentListCmd())
	cmd.AddCommand(newShipmentAddCmd())
	cmd.AddCommand(newShipmentClaimCmd())
	cmd.AddCommand(newShipmentShipCmd())
	cmd.AddCommand(newShipmentReturnBlockedCmd())
	return cmd
}

// newShipmentAddCmd returns the `backlogit shipment add <shipment-id> <item-id>`
// subcommand. It is the CLI fallback for the backlogit_add_to_shipment MCP tool:
// positional args matching the sibling `shipment get <id>` convention, the shared
// core.AddItemToShipment mutation (idempotent for an item already in this
// shipment, refused for one assigned to another), and a success JSON isomorphic
// to the MCP handler result ({shipment_id, item_id, status:"added"}).
func newShipmentAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <shipment-id> <item-id>",
		Short: "Add an item to a shipment",
		Long: `Add a backlog item to a shipment manifest.

This mirrors the backlogit_add_to_shipment MCP tool: it takes positional
<shipment-id> and <item-id> arguments and associates the item via the shared
core mutation. It is idempotent when the item already belongs to this shipment;
adding an item already assigned to another shipment is refused.`,
		Example: `  backlogit shipment add 001-S 001.001-T`,
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			shipmentID := args[0]
			itemID := args[1]
			slog.Info(
				"shipment command invoked",
				"operation", "shipment-add",
				"shipment_id", shipmentID,
				"item_id", itemID,
			)

			ws, err := core.NewWorkspace(ctx, shipmentCWD(cmd))
			if err != nil {
				return fmt.Errorf("open workspace: %w", err)
			}
			defer ws.Close()

			if err := core.AddItemToShipment(ctx, ws, shipmentID, itemID); err != nil {
				return fmt.Errorf("add item to shipment: %w", err)
			}

			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(map[string]string{
				"shipment_id": shipmentID,
				"item_id":     itemID,
				"status":      "added",
			})
		},
	}
}

// newShipmentCreateCmd returns the `backlogit shipment create` subcommand.
func newShipmentCreateCmd() *cobra.Command {
	var title string
	var items string
	var priority string

	cmd := &cobra.Command{
		Use:     "create",
		Short:   "Create a shipment",
		Example: `  backlogit shipment create --title "Sprint 1" --items 001-F,001.001-T`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := context.Background()
			slog.Info("shipment command invoked", "operation", "shipment-create")

			ws, err := core.NewWorkspace(ctx, shipmentCWD(cmd))
			if err != nil {
				return fmt.Errorf("open workspace: %w", err)
			}
			defer ws.Close()

			var opts []core.Option
			if priority != "" {
				opts = append(opts, core.WithPriority(priority))
			}
			shipment, err := core.CreateShipment(ctx, ws, title, splitShipmentItems(items), opts...)
			if err != nil {
				return fmt.Errorf("create shipment: %w", err)
			}

			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(shipment)
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "shipment title")
	cmd.Flags().StringVar(&items, "items", "", "comma-separated item IDs")
	cmd.Flags().StringVar(&priority, "priority", "", "shipment priority (critical, high, medium, low)")
	_ = cmd.MarkFlagRequired("title")
	return cmd
}

// newShipmentGetCmd returns the `backlogit shipment get <id>` subcommand.
func newShipmentGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Get a shipment by ID",
		Long: `Get a shipment by ID and print it as JSON.

The response includes a top-level "covering_feature" object ({id, title}) when
the shipment manifest contains a root covering feature. This field is a
read-only, render-time derivation from the manifest — it is never stored on the
shipment and is omitted entirely when the shipment has no covering feature.

The response also carries a computed-on-read "size_composition" rollup, at
parity with the MCP get_shipment tool.`,
		Example: `  backlogit shipment get 001-S`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			shipmentID := args[0]
			slog.Info("shipment command invoked", "operation", "shipment-get", "shipment_id", shipmentID)

			ws, err := core.NewWorkspace(ctx, shipmentCWD(cmd))
			if err != nil {
				return fmt.Errorf("open workspace: %w", err)
			}
			defer ws.Close()

			shipment, err := core.GetShipment(ctx, ws, shipmentID)
			if err != nil {
				return fmt.Errorf("get shipment: %w", err)
			}

			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(core.ShipmentViewWithComposition(ctx, ws, shipment))
		},
	}
}

// newShipmentListCmd returns the `backlogit shipment list` subcommand.
func newShipmentListCmd() *cobra.Command {
	var status, formatOutput string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List shipments",
		Long: `List shipments in table, tile, or JSON format.

Table and tile output include a COVERING FEATURE column, and JSON output
includes a top-level "covering_feature" object ({id, title}) per shipment. The
covering feature is a read-only, render-time derivation from each shipment
manifest (never stored) and is blank/omitted when a shipment has no covering
feature. JSON output also carries a computed-on-read "size_composition" rollup
per shipment, at parity with the MCP list_shipments tool.`,
		Example: `  backlogit shipment list --status active`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := context.Background()
			slog.Info("shipment command invoked", "operation", "shipment-list", "status", status)

			ws, err := core.NewWorkspace(ctx, shipmentCWD(cmd))
			if err != nil {
				return fmt.Errorf("open workspace: %w", err)
			}
			defer ws.Close()

			shipments, err := db.QueryItems(ctx, ws.DB, db.QueryFilters{
				Type:   "shipment",
				Status: status,
			})
			if err != nil {
				return fmt.Errorf("list shipments: %w", err)
			}
			for _, shipment := range shipments {
				// Normalize on the read edge so the CLI list surface carries an
				// identical items array shape to the MCP list surface. SQLite JSON
				// round-trips treat empty/absent arrays as lossy (a stored shipment
				// whose items is null reaches here as null), so this guarantees the
				// never-null invariant. The nil-map guard exists solely to provide an
				// assignment target; the never-null VALUE guarantee comes entirely
				// from core.NormalizeShipmentItems (the single source of truth).
				if shipment.CustomFields == nil {
					shipment.CustomFields = map[string]any{}
				}
				shipment.CustomFields["items"] = core.NormalizeShipmentItems(shipment)
			}

			effectiveFormat := format.Format(formatOutput)
			if err := validateFormat(effectiveFormat, format.FormatTable, format.FormatJSON, format.FormatTile); err != nil {
				return err
			}
			switch effectiveFormat {
			case format.FormatTable, format.FormatTile:
				return newRenderer(effectiveFormat, cmd.OutOrStdout()).Render(cmd.OutOrStdout(), shipmentColumns(), shipmentRows(ctx, ws, shipments))
			default: // json
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(core.ShipmentViewsWithComposition(ctx, ws, shipments))
			}
		},
	}
	cmd.Flags().StringVar(&status, "status", "", "filter shipments by status")
	cmd.Flags().StringVar(&formatOutput, "format", "json", "output format: table, json, tile")
	return cmd
}

// shipmentColumns returns the table/tile column set for shipment views: a COPY
// of the shared artifactColumns with a trailing COVERING FEATURE column. It
// never mutates the shared artifactColumns slice, which also drives the `list`
// and `queue` views.
func shipmentColumns() []format.Column {
	cols := make([]format.Column, len(artifactColumns), len(artifactColumns)+1)
	copy(cols, artifactColumns)
	return append(cols, format.Column{Key: "covering_feature", Header: "COVERING FEATURE"})
}

// shipmentRows builds table/tile rows for shipments by composing over the shared
// artifactsToRows and appending the derived covering feature (rendered
// "<id> — <title>", blank when absent). Derivation is read-only.
func shipmentRows(ctx context.Context, ws *core.Workspace, shipments []*models.Artifact) []map[string]any {
	rows := artifactsToRows(ctx, ws, shipments)
	for i, shipment := range shipments {
		if cf, ok := core.DeriveCoveringFeature(ctx, ws, shipment); ok {
			rows[i]["covering_feature"] = fmt.Sprintf("%s — %s", cf.ID, cf.Title)
		} else {
			rows[i]["covering_feature"] = ""
		}
	}
	return rows
}
func newShipmentClaimCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "claim <id>",
		Short:   "Claim a queued shipment",
		Example: `  backlogit shipment claim 001-S`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			shipmentID := args[0]
			slog.Info("shipment command invoked", "operation", "shipment-claim", "shipment_id", shipmentID)

			ws, err := core.NewWorkspace(ctx, shipmentCWD(cmd))
			if err != nil {
				return fmt.Errorf("open workspace: %w", err)
			}
			defer ws.Close()

			shipment, err := core.ClaimShipment(ctx, ws, shipmentID)
			if err != nil {
				return fmt.Errorf("claim shipment: %w", err)
			}

			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(shipment)
		},
	}
}

// newShipmentShipCmd returns the `backlogit shipment ship <id>` subcommand.
func newShipmentShipCmd() *cobra.Command {
	var sha string
	var message string
	var author string

	cmd := &cobra.Command{
		Use:     "ship <id>",
		Short:   "Close a released shipment and archive the released scope",
		Example: `  backlogit shipment ship 001-S --sha deadbeef --message "merge: release" --author "dev@example.com"`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			shipmentID := args[0]
			slog.Info("shipment command invoked", "operation", "shipment-ship", "shipment_id", shipmentID)

			ws, err := core.NewWorkspace(ctx, shipmentCWD(cmd))
			if err != nil {
				return fmt.Errorf("open workspace: %w", err)
			}
			defer ws.Close()

			result, err := core.ShipShipment(ctx, ws, shipmentID, &core.CommitMetadata{
				SHA:     sha,
				Message: message,
				Author:  author,
			})
			if err != nil {
				// F5 (083.003-T): a shipment-completion gate refusal must preserve
				// its versioned exit code (6 blocked / 7 config-setup / 8 retryable)
				// rather than collapsing to the generic 1. Mirror moveGateError.
				if ee := shipmentShipGateError(cmd, err); ee != nil {
					return ee
				}
				return fmt.Errorf("ship shipment: %w", err)
			}

			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(result)
		},
	}
	cmd.Flags().StringVar(&sha, "sha", "", "merge commit SHA to record on released artifacts")
	cmd.Flags().StringVar(&message, "message", "", "merge commit message to record on released artifacts")
	cmd.Flags().StringVar(&author, "author", "", "merge commit author to record on released artifacts")
	return cmd
}

// shipmentShipGateError maps a shipment-completion gate refusal to an *ExitError
// carrying the versioned gate exit code (6 blocked / 7 config-setup / 8 retryable)
// so `backlogit shipment ship` preserves DecisionError exit-code fidelity (F5,
// 083.003-T), mirroring moveGateError. It returns nil for a non-gate error so the
// caller wraps it as a generic failure (exit 1). On a gate error it silences
// cobra's default error print and emits the one-line reason to stderr.
func shipmentShipGateError(cmd *cobra.Command, err error) *ExitError {
	ee := gateExitError(err)
	if ee == nil {
		return nil
	}
	cmd.SilenceErrors = true
	fmt.Fprintln(cmd.ErrOrStderr(), err.Error())
	return ee
}

// newShipmentReturnBlockedCmd returns the `backlogit shipment return-blocked` subcommand.
func newShipmentReturnBlockedCmd() *cobra.Command {
	var shipmentID string
	var itemID string
	var reason string

	cmd := &cobra.Command{
		Use:     "return-blocked",
		Short:   "Return a blocked item from a shipment",
		Example: `  backlogit shipment return-blocked --shipment 001-S --item 001.001-T --reason "blocked"`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := context.Background()
			slog.Info(
				"shipment command invoked",
				"operation", "shipment-return-blocked",
				"shipment_id", shipmentID,
				"item_id", itemID,
			)

			ws, err := core.NewWorkspace(ctx, shipmentCWD(cmd))
			if err != nil {
				return fmt.Errorf("open workspace: %w", err)
			}
			defer ws.Close()

			if err := core.ReturnBlockedItem(ctx, ws, shipmentID, itemID, reason); err != nil {
				return fmt.Errorf("return blocked item: %w", err)
			}

			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(map[string]string{
				"shipment_id": shipmentID,
				"item_id":     itemID,
				"item_status": "blocked",
				"reason":      reason,
			})
		},
	}
	cmd.Flags().StringVar(&shipmentID, "shipment", "", "shipment ID")
	cmd.Flags().StringVar(&itemID, "item", "", "item ID")
	cmd.Flags().StringVar(&reason, "reason", "", "blocked reason")
	_ = cmd.MarkFlagRequired("shipment")
	_ = cmd.MarkFlagRequired("item")
	_ = cmd.MarkFlagRequired("reason")
	return cmd
}

func shipmentCWD(cmd *cobra.Command) string {
	if cmd == nil || cmd.Root() == nil {
		return "."
	}
	cwd, err := cmd.Root().PersistentFlags().GetString("cwd")
	if err != nil || cwd == "" {
		return "."
	}
	return cwd
}

func splitShipmentItems(items string) []string {
	if items == "" {
		return nil
	}
	parts := strings.Split(items, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}
