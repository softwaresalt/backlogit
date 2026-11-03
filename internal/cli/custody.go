package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
	corerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// custodyDBUnavailable is the degraded marker on a live custody result served
// without the SQLite cache. It matches the backlogit_custody_* MCP responses.
const custodyDBUnavailable = "unavailable"

// custodyCLIError is a structured custody refusal. Code and Message match the
// error and message fields of the corresponding MCP error response, so both
// surfaces report the same refusal.
type custodyCLIError struct {
	Code    string
	Message string
}

func (e *custodyCLIError) Error() string {
	return e.Code + ": " + e.Message
}

// custodyMutationOutput is the live or dry-run offer and accept result. DB is set
// only when the SQLite cache was bypassed.
type custodyMutationOutput struct {
	core.CustodyMutationResult
	DB string `json:"db,omitempty"`
}

// custodyRefusalCodes maps core custody refusals to the MCP error codes.
var custodyRefusalCodes = []struct {
	err  error
	code string
}{
	{core.ErrCustodyDefaultBranchForbidden, "custody_default_branch_forbidden"},
	{core.ErrNonceProviderUnavailable, "nonce_provider_unavailable"},
	{core.ErrNonceReplayed, "nonce_replayed"},
	{core.ErrAuthorizationRequired, "authorization_required"},
	{core.ErrApprovalChannelUnavailable, "approval_channel_unavailable"},
	{core.ErrAuthorizationRejected, "authorization_rejected"},
	{core.ErrApprovalWrongShipment, "approval_wrong_shipment"},
	{core.ErrApprovalWrongRequestIdentity, "approval_wrong_request_identity"},
	{core.ErrTrustAnchorRevoked, "trust_anchor_revoked"},
	{core.ErrOperatorApprovalRequired, "operator_approval_required"},
	{core.ErrCustodyStaleDigest, "custody_stale_digest"},
	{core.ErrCustodyWrongBranch, "custody_wrong_branch"},
	{core.ErrMixedIndexWorktreeDivergence, "custody_index_worktree_divergence"},
}

// NewCustodyCmd returns the custody command group. It is the CLI surface of the
// backlogit_custody_* MCP tools and shares their refusal codes and degraded marker.
func NewCustodyCmd(cwd *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "custody",
		Short: "Preview, offer, accept, and verify workflow custody handoffs",
		Long: `Manage workflow custody handoffs for a shipment.

Custody records which candidate changes one role hands to another. preview and
verify are read-only. offer and accept write the handoff; --dry-run validates
without writing. A live offer or accept runs without the SQLite cache only when
the cache cannot be opened, and then reports db: "unavailable".`,
	}
	cmd.AddCommand(
		newCustodyPreviewCmd(cwd),
		newCustodyOfferCmd(cwd),
		newCustodyAcceptCmd(cwd),
		newCustodyVerifyCmd(cwd),
	)
	return cmd
}

func newCustodyPreviewCmd(cwd *string) *cobra.Command {
	var candidates, branch string
	cmd := &cobra.Command{
		Use:   "preview <shipment-id>",
		Short: "Classify custody candidates without writing state",
		Long: `Classify candidate paths for a shipment against the base commit, the Git index,
and the worktree. The result reports the candidate-set digest and the authority
profile that the candidate set requires. This command is read-only and works
without the SQLite cache.

This mirrors the backlogit_custody_preview MCP tool.`,
		Example: `  backlogit custody preview 001-S --candidates '[{"path":"docs/plan.md","change_kind":"modified","staged_state":"unstaged_only","contributor":"stage","source_kind":"stage_artifact"}]'`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) (retErr error) {
			if err := custodyRequired("shipment_id", args[0]); err != nil {
				return err
			}
			var specs []core.CustodyCandidateSpec
			if err := custodyDecodeJSON("candidates", candidates, &specs); err != nil {
				return err
			}
			ctx := cmd.Context()
			ws, err := core.NewReadOnlyWorkspace(ctx, *cwd)
			if err != nil {
				return fmt.Errorf("open workspace: %w", err)
			}
			defer closeCustodyWorkspace(ws, &retErr)
			slog.Info("custody command invoked", "command", "preview", "shipment_id", args[0])

			preview, err := core.PreviewCustodyCandidates(ctx, ws, core.CustodyPreviewOptions{
				ShipmentID: args[0],
				Branch:     branch,
				Candidates: specs,
			})
			if err != nil {
				return custodyOpError("custody preview", err)
			}
			preview.Candidates = custodyNonNil(preview.Candidates)
			preview.Warnings = custodyNonNil(preview.Warnings)
			return writeCustodyJSON(cmd, preview)
		},
	}
	cmd.Flags().StringVar(&candidates, "candidates", "", "JSON array of candidate specs: path, change_kind, staged_state, contributor, source_kind")
	cmd.Flags().StringVar(&branch, "branch", "", "work branch to evaluate against (defaults to the checked-out branch)")
	_ = cmd.MarkFlagRequired("candidates")
	cmd.Flags().Bool("json", false, "emit JSON output (default)")
	return cmd
}

func newCustodyOfferCmd(cwd *string) *cobra.Command {
	var spec string
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "offer <shipment-id>",
		Short: "Offer a custody handoff for a shipment",
		Long: `Record a custody handoff offer for a shipment. --spec is a JSON custody receipt
with its entries, candidate_set_digest, authority_profile, and accepting role.
--dry-run validates the offer without writing any state.

This mirrors the backlogit_custody_offer MCP tool.`,
		Example: `  backlogit custody offer 001-S --spec '<receipt-json>' --dry-run`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) (retErr error) {
			if err := custodyRequired("shipment_id", args[0]); err != nil {
				return err
			}
			var receipt core.CustodyReceipt
			if err := custodyDecodeJSON("spec", spec, &receipt); err != nil {
				return err
			}
			ctx := cmd.Context()
			ws, degraded, err := openCustodyMutation(ctx, *cwd, dryRun)
			if err != nil {
				return err
			}
			defer closeCustodyWorkspace(ws, &retErr)
			slog.Info("custody command invoked", "command", "offer", "shipment_id", args[0],
				"dry_run", dryRun, "db_unavailable", degraded)

			offered, err := core.OfferCustodyHandoff(ctx, ws, core.CustodyOfferOptions{
				ShipmentID: args[0],
				Spec:       receipt,
				DryRun:     dryRun,
			})
			if err != nil {
				return custodyOpError("custody offer", err)
			}
			return writeCustodyMutation(cmd, offered, degraded)
		},
	}
	cmd.Flags().StringVar(&spec, "spec", "", "JSON custody receipt with entries, candidate_set_digest, authority_profile, and accepting_role")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "validate the offer without writing")
	_ = cmd.MarkFlagRequired("spec")
	cmd.Flags().Bool("json", false, "emit JSON output (default)")
	return cmd
}

func newCustodyAcceptCmd(cwd *string) *cobra.Command {
	var receiptID, by, idempotencyKey, authToken string
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "accept <shipment-id>",
		Short: "Accept an offered custody handoff",
		Long: `Accept a previously offered custody receipt. operator_verified receipts require
--auth-token, a signed operator approval, and a configured nonce provider; the
operator approval must be supplied in the signed token rather than inferred from
a confirmation. --dry-run verifies the acceptance without consuming a nonce or
writing state.

This mirrors the backlogit_custody_accept MCP tool.`,
		Example: `  backlogit custody accept 001-S --receipt-id rcpt-stage-ship-1 --by ship --dry-run`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) (retErr error) {
			if err := custodyRequired("shipment_id", args[0]); err != nil {
				return err
			}
			if err := custodyRequired("receipt_id", receiptID); err != nil {
				return err
			}
			if err := custodyRequired("by", by); err != nil {
				return err
			}
			ctx := cmd.Context()
			ws, degraded, err := openCustodyMutation(ctx, *cwd, dryRun)
			if err != nil {
				return err
			}
			defer closeCustodyWorkspace(ws, &retErr)
			slog.Info("custody command invoked", "command", "accept", "shipment_id", args[0],
				"receipt_id", receiptID, "dry_run", dryRun, "db_unavailable", degraded)

			accepted, err := core.AcceptCustodyHandoff(ctx, ws, core.CustodyAcceptOptions{
				ShipmentID:     args[0],
				ReceiptID:      receiptID,
				By:             by,
				IdempotencyKey: idempotencyKey,
				AuthToken:      authToken,
				DryRun:         dryRun,
			})
			if err != nil {
				return custodyOpError("custody accept", err)
			}
			return writeCustodyMutation(cmd, accepted, degraded)
		},
	}
	cmd.Flags().StringVar(&receiptID, "receipt-id", "", "offered receipt ID to accept")
	cmd.Flags().StringVar(&by, "by", "", "accepting role or principal")
	cmd.Flags().StringVar(&idempotencyKey, "idempotency-key", "", "key that deduplicates retried accepts")
	cmd.Flags().StringVar(&authToken, "auth-token", "", "encoded operator approval token (required for operator_verified receipts)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "verify the acceptance without consuming a nonce or writing")
	_ = cmd.MarkFlagRequired("receipt-id")
	_ = cmd.MarkFlagRequired("by")
	cmd.Flags().Bool("json", false, "emit JSON output (default)")
	return cmd
}

func newCustodyVerifyCmd(cwd *string) *cobra.Command {
	var branch string
	cmd := &cobra.Command{
		Use:   "verify <shipment-id>",
		Short: "Verify recorded custody receipts against Git state",
		Long: `Verify the recorded custody receipts of a shipment against the current Git state.
This command is read-only.

This mirrors the backlogit_custody_verify MCP tool.`,
		Example: `  backlogit custody verify 001-S --branch feat/my-branch`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) (retErr error) {
			if err := custodyRequired("shipment_id", args[0]); err != nil {
				return err
			}
			ctx := cmd.Context()
			ws, err := core.NewReadOnlyWorkspace(ctx, *cwd)
			if err != nil {
				return fmt.Errorf("open workspace: %w", err)
			}
			defer closeCustodyWorkspace(ws, &retErr)
			slog.Info("custody command invoked", "command", "verify", "shipment_id", args[0])

			verified, err := core.VerifyCustodyState(ctx, ws, core.CustodyVerifyOptions{
				ShipmentID: args[0],
				Branch:     branch,
			})
			if err != nil {
				return custodyOpError("custody verify", err)
			}
			verified.Verdicts = custodyNonNil(verified.Verdicts)
			verified.Warnings = custodyNonNil(verified.Warnings)
			return writeCustodyJSON(cmd, verified)
		},
	}
	cmd.Flags().StringVar(&branch, "branch", "", "work branch to evaluate against (defaults to the checked-out branch)")
	cmd.Flags().Bool("json", false, "emit JSON output (default)")
	return cmd
}

// openCustodyMutation opens the workspace for a custody offer or accept. A dry run
// is read-only. A live mutation uses the initialized workspace and falls back to the
// DB-optional canonical workspace only when the SQLite cache cannot be opened. Every
// other failure is returned, and the returned flag reports the fallback.
func openCustodyMutation(ctx context.Context, root string, dryRun bool) (*core.Workspace, bool, error) {
	if dryRun {
		ws, err := core.NewReadOnlyWorkspace(ctx, root)
		if err != nil {
			return nil, false, fmt.Errorf("open workspace: %w", err)
		}
		return ws, false, nil
	}
	ws, err := core.NewWorkspace(ctx, root)
	if err == nil {
		return ws, false, nil
	}
	if !custodyIsSQLiteCacheError(err) {
		return nil, false, fmt.Errorf("open workspace: %w", err)
	}
	fallback, fbErr := custodyDBOptionalWorkspace(ctx, root)
	if fbErr != nil {
		return nil, false, fmt.Errorf("open custody workspace without sqlite cache: %w", errors.Join(err, fbErr))
	}
	slog.Warn("custody live operation running without the SQLite cache", "error", err)
	return fallback, true, nil
}

// custodyIsSQLiteCacheError reports whether a workspace open failed at the SQLite
// cache: opening the database file or preparing its schema.
func custodyIsSQLiteCacheError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "open database:") || strings.Contains(msg, "ensure schema:")
}

// custodyDBOptionalWorkspace opens a canonical workspace with no SQLite handle.
// Custody mutations locate shipments on disk and skip index writes when DB is nil.
// Recovery of a pending non-custody journal needs SQLite, so the fallback is refused.
func custodyDBOptionalWorkspace(ctx context.Context, root string) (*core.Workspace, error) {
	ws, err := core.NewReadOnlyWorkspace(ctx, root)
	if err != nil {
		return nil, fmt.Errorf("open canonical workspace: %w", err)
	}
	if err := custodyRequireCustodyOnlyJournals(ws.StorageRoot); err != nil {
		return nil, errors.Join(err, ws.Close())
	}
	headerDef, err := config.LoadHeaderDef(ws.StorageRoot)
	switch {
	case err == nil:
		ws.HeaderDef = headerDef
	case !errors.Is(err, os.ErrNotExist):
		return nil, errors.Join(fmt.Errorf("load header-def: %w", err), ws.Close())
	}
	return ws, nil
}

// custodyRequireCustodyOnlyJournals refuses the SQLite-free fallback when a pending
// shipment-operation journal is not a custody journal.
func custodyRequireCustodyOnlyJournals(storageRoot string) error {
	opsDir := filepath.Join(storageRoot, "ops")
	entries, err := os.ReadDir(opsDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read pending shipment operations: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		raw, readErr := os.ReadFile(filepath.Join(opsDir, entry.Name()))
		if readErr != nil {
			return fmt.Errorf("read pending shipment operation %s: %w", entry.Name(), readErr)
		}
		var journal struct {
			Operation string `json:"operation"`
		}
		if jsonErr := json.Unmarshal(raw, &journal); jsonErr != nil || !custodyOperation(journal.Operation) {
			return fmt.Errorf("pending shipment operation %s requires the sqlite cache for recovery", entry.Name())
		}
	}
	return nil
}

// custodyOperation reports whether op is a custody journal operation. The values
// mirror the unexported core shipment-operation constants.
func custodyOperation(op string) bool {
	return op == "custody_offer" || op == "custody_accept"
}

// custodyOpError maps a custody failure to the structured code the MCP tool returns.
func custodyOpError(op string, err error) error {
	for _, refusal := range custodyRefusalCodes {
		if errors.Is(err, refusal.err) {
			return &custodyCLIError{Code: refusal.code, Message: fmt.Sprintf("%s: %v", op, err)}
		}
	}
	if errors.Is(err, corerrors.ErrValidation) {
		return &custodyCLIError{Code: "validation_failed", Message: err.Error()}
	}
	return &custodyCLIError{Code: "internal", Message: fmt.Sprintf("%s: %v", op, err)}
}

// custodyRequired refuses an empty required value with the MCP validation code.
func custodyRequired(name, value string) error {
	if value == "" {
		return &custodyCLIError{Code: "validation_failed", Message: name + " is required"}
	}
	return nil
}

// custodyDecodeJSON decodes a required JSON flag value, refusing empty input.
func custodyDecodeJSON(name, raw string, dst any) error {
	if strings.TrimSpace(raw) == "" {
		return &custodyCLIError{Code: "validation_failed", Message: name + " is required"}
	}
	if err := json.Unmarshal([]byte(raw), dst); err != nil {
		return &custodyCLIError{Code: "validation_failed", Message: fmt.Sprintf("%s must be valid JSON: %v", name, err)}
	}
	return nil
}

// custodyNonNil replaces a nil slice with an empty one so JSON encodes []. Never null.
func custodyNonNil[T any](items []T) []T {
	if items == nil {
		return []T{}
	}
	return items
}

// closeCustodyWorkspace closes ws and joins any close failure into retErr.
func closeCustodyWorkspace(ws *core.Workspace, retErr *error) {
	if cerr := ws.Close(); cerr != nil {
		*retErr = errors.Join(*retErr, fmt.Errorf("close workspace: %w", cerr))
	}
}

// writeCustodyMutation encodes a live or dry-run offer or accept result.
func writeCustodyMutation(cmd *cobra.Command, result core.CustodyMutationResult, degraded bool) error {
	result.Verdicts = custodyNonNil(result.Verdicts)
	result.Warnings = custodyNonNil(result.Warnings)
	out := custodyMutationOutput{CustodyMutationResult: result}
	if degraded {
		out.DB = custodyDBUnavailable
	}
	return writeCustodyJSON(cmd, out)
}

func writeCustodyJSON(cmd *cobra.Command, v any) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return fmt.Errorf("write custody result: %w", err)
	}
	return nil
}
