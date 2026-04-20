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
	cmd.AddCommand(newShipmentClaimCmd())
	cmd.AddCommand(newShipmentShipCmd())
	cmd.AddCommand(newShipmentReturnBlockedCmd())
	return cmd
}

// newShipmentCreateCmd returns the `backlogit shipment create` subcommand.
func newShipmentCreateCmd() *cobra.Command {
	var title string
	var items string

	cmd := &cobra.Command{
		Use:     "create",
		Short:   "Create a shipment",
		Example: `  backlogit shipment create --title "Sprint 1" --items 001-T,002-T`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := context.Background()
			slog.Info("shipment command invoked", "operation", "shipment-create")

			ws, err := core.NewWorkspace(ctx, shipmentCWD(cmd))
			if err != nil {
				return fmt.Errorf("open workspace: %w", err)
			}
			defer ws.Close()

			shipment, err := core.CreateShipment(ctx, ws, title, splitShipmentItems(items))
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
	_ = cmd.MarkFlagRequired("title")
	return cmd
}

// newShipmentGetCmd returns the `backlogit shipment get <id>` subcommand.
func newShipmentGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "get <id>",
		Short:   "Get a shipment by ID",
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
			return enc.Encode(shipment)
		},
	}
}

// newShipmentListCmd returns the `backlogit shipment list` subcommand.
func newShipmentListCmd() *cobra.Command {
	var status, formatOutput string

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List shipments",
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

			switch format.Format(formatOutput) {
			case format.FormatTable, format.FormatTile:
				return newRenderer(formatOutput).Render(cmd.OutOrStdout(), artifactColumns, artifactsToRows(shipments))
			default: // json (default)
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(shipments)
			}
		},
	}
	cmd.Flags().StringVar(&status, "status", "", "filter shipments by status")
	cmd.Flags().StringVar(&formatOutput, "format", "json", "output format: table, json, tile")
	return cmd
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

// newShipmentReturnBlockedCmd returns the `backlogit shipment return-blocked` subcommand.
func newShipmentReturnBlockedCmd() *cobra.Command {
	var shipmentID string
	var itemID string
	var reason string

	cmd := &cobra.Command{
		Use:     "return-blocked",
		Short:   "Return a blocked item from a shipment",
		Example: `  backlogit shipment return-blocked --shipment 001-S --item 001-T --reason "blocked"`,
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
