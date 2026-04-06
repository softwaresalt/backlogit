package cli

import (
	"github.com/spf13/cobra"
)

// NewShipmentCmd returns the top-level `backlogit shipment` command group.
//
// Worker: Register subcommands: create, get, list, claim, return-blocked.
// Each subcommand should have .Short, .Long, and .Example fields populated.
// Emit slog.Info per invocation with operation name and shipment ID.
func NewShipmentCmd() *cobra.Command {
	panic("not implemented: Worker: Register shipment subcommands (create, get, list, claim, return-blocked) with Cobra")
}

// newShipmentCreateCmd returns the `backlogit shipment create` subcommand.
//
// Worker: Accept --title and --items flags, call core.CreateShipment, print the
// new shipment ID. Emit slog.Info with operation=shipment-create.
func newShipmentCreateCmd() *cobra.Command {
	panic("not implemented: Worker: Implement shipment create with --title and --items flags calling core.CreateShipment")
}

// newShipmentGetCmd returns the `backlogit shipment get <id>` subcommand.
//
// Worker: Accept shipment ID as arg, call core.GetShipment, print formatted output.
func newShipmentGetCmd() *cobra.Command {
	panic("not implemented: Worker: Implement shipment get with ID arg calling core.GetShipment")
}

// newShipmentListCmd returns the `backlogit shipment list` subcommand.
//
// Worker: Accept optional --status filter, query shipments from index, print table.
func newShipmentListCmd() *cobra.Command {
	panic("not implemented: Worker: Implement shipment list with --status filter querying index")
}

// newShipmentClaimCmd returns the `backlogit shipment claim <id>` subcommand.
//
// Worker: Move shipment from queued to active. Emit slog.Info.
func newShipmentClaimCmd() *cobra.Command {
	panic("not implemented: Worker: Implement shipment claim moving status queued->active")
}

// newShipmentReturnBlockedCmd returns the `backlogit shipment return-blocked` subcommand.
//
// Worker: Accept --shipment, --item, --reason flags, call core.ReturnBlockedItem.
func newShipmentReturnBlockedCmd() *cobra.Command {
	panic("not implemented: Worker: Implement return-blocked with --shipment, --item, --reason flags")
}
