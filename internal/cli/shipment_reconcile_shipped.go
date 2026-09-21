package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/softwaresalt/backlogit/internal/core"
	corerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// newShipmentReconcileShippedCmd returns the `backlogit shipment
// reconcile-shipped <id>` subcommand (167.004-T, #423, 167-F, "U4"). It is the
// CLI surface for the governed archived-shipment-reconciliation-to-shipped
// transaction (core.ReconcileShipmentToShipped, 167.008-T): a legacy archived
// shipment whose released scope predates the governed ShipShipment envelope
// (144-F) can be repaired into the durable, event-backed archived_status:
// "shipped" via an idempotent, evidence-verified request.
//
// A live (non-dry-run) mutation requires an explicit confirmation guard
// (shipmentReconcileConfirm): the shipment-specific phrase "reconcile-shipped
// <shipment-id>" via --confirm, or the same phrase typed at an interactive TTY
// prompt. This is a confirmation-only guard against silent/accidental
// invocation plus a durable audit trail (Actor/Reason on the request) — it is
// NOT machine-authenticated operator authorization (see corerrors.
// ErrConfirmationRequired's doc comment); genuine authenticated authorization
// is tracked separately. --dry-run needs no confirmation at all and performs
// zero reconciliation writes: core.ReconcileShipmentToShipped's own DryRun
// branches evaluate every precondition and report the planned outcome without
// ever reaching a write.
func newShipmentReconcileShippedCmd() *cobra.Command {
	var reason, actor, idempotencyKey, mergeSHA, closureEvidence, secondApprover, confirm string
	var evidenceRefs []string
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "reconcile-shipped <shipment-id>",
		Short: "Governed repair: reconcile a legacy archived shipment to archived_status=shipped",
		Long: `Reconcile an archived shipment whose released work predates the governed
ShipShipment envelope (144-F) into the durable, event-backed archived_status:
"shipped" (#423, 167-F).

This is a governed two-phase transaction (core.ReconcileShipmentToShipped): it
validates the shipment is archived with archived_status=active and every
manifest member reached a supported terminal status (done/accepted), verifies
the supplied merge commit and closure evidence against the configured trusted
refs, and writes the archive frontmatter to archived_status="shipped" before
durably appending the reconciliation event. Requests are idempotent: replaying the same
--idempotency-key is a no-op; a different request under the same key is
refused as a conflict.

A live (non-dry-run) mutation requires the shipment-specific confirmation
phrase "reconcile-shipped <shipment-id>" via --confirm, or typing that same
phrase at an interactive TTY prompt. --dry-run needs no confirmation and makes
no reconciliation writes (the archive file, index row, and event log are all
left unchanged).`,
		Example: `  backlogit shipment reconcile-shipped 048-S --reason "legacy repair" --actor operator \
    --idempotency-key idem-048-s-1 --merge-sha 0123456789abcdef0123456789abcdef01234567 \
    --closure-evidence docs/closure/048-S.md --confirm "reconcile-shipped 048-S"
  backlogit shipment reconcile-shipped 048-S --dry-run --reason "preview" --actor operator \
    --idempotency-key idem-048-s-1 --merge-sha 0123456789abcdef0123456789abcdef01234567 \
    --closure-evidence docs/closure/048-S.md`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			shipmentID := args[0]

			if err := requireShipmentReconcileTrimmedFields(map[string]string{
				"reason":           reason,
				"actor":            actor,
				"idempotency-key":  idempotencyKey,
				"merge-sha":        mergeSHA,
				"closure-evidence": closureEvidence,
			}); err != nil {
				return err
			}

			if !dryRun {
				if err := shipmentReconcileConfirm(cmd, shipmentID, confirm); err != nil {
					cmd.SilenceErrors = true
					fmt.Fprintln(cmd.ErrOrStderr(), err.Error())
					return &ExitError{Code: ExitConfirmationRequired, Msg: err.Error(), Err: err}
				}
			}

			ctx := context.Background()
			slog.Info(
				"shipment command invoked",
				"operation", "shipment-reconcile-shipped",
				"shipment_id", shipmentID,
				"dry_run", dryRun,
			)

			ws, err := core.NewWorkspace(ctx, shipmentCWD(cmd))
			if err != nil {
				return fmt.Errorf("open workspace: %w", err)
			}
			defer ws.Close()

			req := core.ShipmentShippedReconcileRequest{
				ShipmentID:      shipmentID,
				Reason:          reason,
				Actor:           actor,
				SecondApprover:  secondApprover,
				IdempotencyKey:  idempotencyKey,
				MergeSHA:        mergeSHA,
				ClosureEvidence: closureEvidence,
				EvidenceRefs:    evidenceRefs,
				DryRun:          dryRun,
			}

			result, err := core.ReconcileShipmentToShipped(ctx, ws, req)
			if ee := shipmentReconcileExitError(result, err); ee != nil {
				cmd.SilenceErrors = true
				fmt.Fprintln(cmd.ErrOrStderr(), ee.Msg)
				return ee
			}
			if err != nil {
				return fmt.Errorf("reconcile shipment to shipped: %w", err)
			}

			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(map[string]any{
				"outcome":     string(result.Outcome),
				"shipment_id": result.ShipmentID,
				"dry_run":     result.DryRun,
				"message":     result.Message,
			})
		},
	}

	cmd.Flags().StringVar(&reason, "reason", "", "operator justification for the reconciliation (required)")
	cmd.Flags().StringVar(&actor, "actor", "", "actor performing the reconciliation, recorded for audit (required)")
	cmd.Flags().StringVar(&idempotencyKey, "idempotency-key", "", "idempotency key for this reconciliation request (required)")
	cmd.Flags().StringVar(&mergeSHA, "merge-sha", "", "merge commit SHA that delivered the released scope (required)")
	cmd.Flags().StringVar(&closureEvidence, "closure-evidence", "", "workspace-relative path to closure evidence documenting the release (required)")
	cmd.Flags().StringVar(&secondApprover, "second-approver", "", "optional second approver recorded for audit (must differ from --actor)")
	cmd.Flags().StringArrayVar(&evidenceRefs, "evidence-ref", nil, "additional evidence reference; repeatable")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "evaluate every precondition and print the planned outcome without writing (needs no --confirm)")
	cmd.Flags().StringVar(&confirm, "confirm", "", `confirmation phrase "reconcile-shipped <shipment-id>", required for a live (non-dry-run) mutation unless stdin is an interactive TTY`)
	_ = cmd.MarkFlagRequired("reason")
	_ = cmd.MarkFlagRequired("actor")
	_ = cmd.MarkFlagRequired("idempotency-key")
	_ = cmd.MarkFlagRequired("merge-sha")
	_ = cmd.MarkFlagRequired("closure-evidence")
	return cmd
}

// requireShipmentReconcileTrimmedFields validates that every named flag value
// is non-empty AFTER trimming whitespace. Cobra's required-flag check only
// verifies a flag was SUPPLIED, so `--reason ”` or `--reason '   '` passes
// that check while remaining semantically empty (PR #424 review finding).
// This is a fast-fail CLI-boundary UX layer: the equivalent trimmed-non-empty
// validation already exists at the core layer (167.001-T/167.014-T
// preconditions), so this check does not replace it, only fails faster and
// with a clearer message, without ever reaching a core call or a write.
// fieldFlags iterates in caller-declared order via a slice of pairs so the
// FIRST empty field (in flag-declaration order) is reported deterministically.
func requireShipmentReconcileTrimmedFields(fields map[string]string) error {
	order := []string{"reason", "actor", "idempotency-key", "merge-sha", "closure-evidence"}
	for _, name := range order {
		value, ok := fields[name]
		if !ok {
			continue
		}
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("shipment reconcile-shipped: --%s must be non-empty (whitespace-only is rejected): %w", name, corerrors.ErrValidation)
		}
	}
	return nil
}

// shipmentReconcileConfirmPhrase returns the deterministic, shipment-scoped
// confirmation phrase a live (non-dry-run) `shipment reconcile-shipped`
// mutation must match exactly — a fixed literal per shipment ID so a stray or
// copy-pasted token confirming a DIFFERENT shipment never confirms this one.
func shipmentReconcileConfirmPhrase(shipmentID string) string {
	return "reconcile-shipped " + shipmentID
}

// shipmentReconcileStdinIsInteractive is an injectable seam (overridable in
// tests) reporting whether the process's real stdin is an interactive
// terminal. It gates whether an unmatched/absent --confirm token may still be
// satisfied via an interactive prompt.
var shipmentReconcileStdinIsInteractive = func() bool {
	return isTerminal(os.Stdin)
}

// shipmentReconcileConfirm enforces the explicit-confirmation guard for a live
// shipment reconcile-shipped mutation (167.004-T): the supplied --confirm
// token must exactly equal shipmentReconcileConfirmPhrase(shipmentID), or —
// only when stdin is an interactive TTY — the operator must type that same
// exact phrase when prompted. It returns corerrors.ErrConfirmationRequired,
// wrapped, when neither condition is met. This check runs BEFORE any
// workspace is opened or core call made, so a denied confirmation performs
// zero reconciliation writes.
//
// TRUTHFUL FRAMING: this is NOT machine-authenticated authorization — an
// autonomous caller can supply the phrase or allocate a TTY just as easily as
// a human operator. It is an explicit-confirmation guard against silent or
// accidental invocation, plus a durable audit trail via the request's
// Actor/Reason fields. Genuine authenticated operator authorization is
// tracked separately (see the 167.004-T backlog task body) and is
// deliberately NOT implemented here.
func shipmentReconcileConfirm(cmd *cobra.Command, shipmentID, confirmFlag string) error {
	phrase := shipmentReconcileConfirmPhrase(shipmentID)
	if confirmFlag == phrase {
		return nil
	}
	if shipmentReconcileStdinIsInteractive() {
		fmt.Fprintf(cmd.OutOrStdout(), "Type %q to confirm this shipment reconciliation: ", phrase)
		reader := bufio.NewReader(cmd.InOrStdin())
		line, _ := reader.ReadString('\n')
		if strings.TrimRight(line, "\r\n") == phrase {
			return nil
		}
	}
	return fmt.Errorf("shipment %s reconciliation requires the confirmation phrase %q via --confirm, or typing it at an interactive prompt: %w", shipmentID, phrase, corerrors.ErrConfirmationRequired)
}

// shipmentReconcileExitError maps a core.ReconcileShipmentToShipped result/
// error pair to the versioned reconcile exit code (167.004-T), or nil when
// the outcome is an ordinary success (reconciled/no_op, whether dry-run or
// live) that should fall through to exit 0. A typed reject (conflict, or a
// precondition/evidence validation failure) maps to ExitReconcileConflict; an
// indeterminate write/read outcome maps to the distinct
// ExitReconcileIndeterminate. Conflict and no_op outcomes are returned by
// core.ReconcileShipmentToShipped WITH A NIL ERROR (they are not exceptional),
// so both the result.Outcome and err are inspected here — err alone is
// insufficient.
func shipmentReconcileExitError(result core.ShipmentShippedReconcileResult, err error) *ExitError {
	if err != nil {
		if corerrors.IsWriteIndeterminate(err) || result.Outcome == core.ShipmentReconcileOutcomeIndeterminate {
			return &ExitError{Code: ExitReconcileIndeterminate, Msg: err.Error(), Err: err}
		}
		if errors.Is(err, corerrors.ErrShipmentReconcileConflict) ||
			errors.Is(err, corerrors.ErrUnsupportedLegacyPreState) ||
			errors.Is(err, corerrors.ErrUnsupportedLegacyDescope) ||
			errors.Is(err, corerrors.ErrShipmentReconcileEvidence) ||
			errors.Is(err, corerrors.ErrValidation) {
			return &ExitError{Code: ExitReconcileConflict, Msg: err.Error(), Err: err}
		}
		return nil
	}
	switch result.Outcome {
	case core.ShipmentReconcileOutcomeConflict:
		return &ExitError{Code: ExitReconcileConflict, Msg: result.Message}
	case core.ShipmentReconcileOutcomeIndeterminate:
		return &ExitError{Code: ExitReconcileIndeterminate, Msg: result.Message}
	default:
		return nil
	}
}
