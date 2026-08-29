package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/softwaresalt/backlogit/internal/core"
)

// newReconcileCommand returns the `backlogit reconcile` command.
//
// Reconcile corrects archived items whose archived_status does not reflect the
// correct terminal status.  Each item is unarchived, updated to the target
// status, and re-archived with a durable lifecycle_reconciliation event.
//
// STUB: the handler calls core.ReconcileArchivedLifecycle and validates the
// workspace but outputs a placeholder JSON response.  Replace with real JSON
// serialisation of the ReconciliationResult in the green phase.
func newReconcileCommand(cwd *string) *cobra.Command {
	var reason string
	var actor string
	var targetStatus string
	var idempotencyKey string

	cmd := &cobra.Command{
		Use:   "reconcile <item-id...>",
		Short: "Reconcile archived items by correcting their lifecycle status",
		Long: `Reconcile archived items whose archived_status does not reflect the correct
terminal status.  Each item is unarchived, updated to the target status, and
re-archived with a durable lifecycle_reconciliation event that preserves the
original archive history.

The operation is idempotent: items already at the target status are returned
as no_op without modification.`,
		Example: `  backlogit reconcile 001-T 002-T --reason "P-001 lifecycle fix" --actor "ship-agent"
  backlogit reconcile 001-T --reason "closed without resolution" --actor "ops" --target-status rejected`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			ws, err := core.NewWorkspace(ctx, *cwd)
			if err != nil {
				return fmt.Errorf("open workspace: %w", err)
			}
			defer ws.Close()

			req := core.ReconciliationRequest{
				ItemIDs:        args,
				TargetStatus:   targetStatus,
				Reason:         reason,
				Actor:          actor,
				IdempotencyKey: idempotencyKey,
			}
			result, err := core.ReconcileArchivedLifecycle(ctx, ws.DB, ws, req)
			if err != nil {
				return fmt.Errorf("reconcile: %w", err)
			}

			// STUB: discard the real result and output a placeholder.
			// Replace with json.NewEncoder(cmd.OutOrStdout()).Encode(result)
			// once the green phase wires the full response shape.
			_ = result
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(map[string]string{"outcome": "not_implemented"})
		},
	}

	cmd.Flags().StringVar(&reason, "reason", "", "Reason for reconciliation (required)")
	cmd.Flags().StringVar(&actor, "actor", "", "Actor performing the reconciliation (required)")
	cmd.Flags().StringVar(&targetStatus, "target-status", "done", "Target terminal status (done, accepted, rejected, abandoned, shipped)")
	cmd.Flags().StringVar(&idempotencyKey, "idempotency-key", "", "Optional idempotency key for deduplication")
	_ = cmd.MarkFlagRequired("reason")
	_ = cmd.MarkFlagRequired("actor")

	return cmd
}
