package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	mcplib "github.com/mark3labs/mcp-go/mcp"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
	corerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// custodyDBUnavailable is the degraded marker on a live custody response served
// without the SQLite cache. It is omitted when the cache served the operation.
const custodyDBUnavailable = "unavailable"

// custodyMutationResponse is the live offer and accept response: the core result
// plus the degraded database marker.
type custodyMutationResponse struct {
	core.CustodyMutationResult
	DB string `json:"db,omitempty"`
}

func noopRelease() {}

// custodyReadOnlyWorkspace returns the workspace for preview, verify, and dry-run.
// It reuses an initialized workspace when one is cached and otherwise opens a
// transient read-only workspace that never runs recovery or writes SQLite.
func (s *Server) custodyReadOnlyWorkspace(ctx context.Context) (*core.Workspace, func(), *mcplib.CallToolResult) {
	return s.manifestWorkspace(ctx, true)
}

// custodyWorkspaceFor returns the workspace for an offer or accept. A dry run is
// read-only. A live mutation requires the initialized workspace and falls back to
// the DB-optional workspace only when the SQLite cache is unusable; the fallback
// is reported through the degraded flag.
func (s *Server) custodyWorkspaceFor(ctx context.Context, dryRun bool) (*core.Workspace, func(), bool, *mcplib.CallToolResult) {
	if dryRun {
		ws, release, result := s.custodyReadOnlyWorkspace(ctx)
		return ws, release, false, result
	}
	return s.custodyLiveWorkspace(ctx)
}

// custodyLiveWorkspace returns the initialized workspace for a live custody
// mutation. When the SQLite cache cannot be opened, it returns a DB-optional
// canonical workspace and reports degraded=true; the fallback decision is logged.
// Every other workspace failure is returned as a structured error.
func (s *Server) custodyLiveWorkspace(ctx context.Context) (*core.Workspace, func(), bool, *mcplib.CallToolResult) {
	ws, err := s.ensureWorkspace(ctx)
	if err == nil {
		return ws, noopRelease, false, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return nil, noopRelease, false, WorkspaceNotInitialized()
	}
	var ambiguous *corerrors.AmbiguousWorkspaceRootError
	if errors.As(err, &ambiguous) {
		return nil, noopRelease, false, WorkspaceRootAmbiguous(ambiguous.Roots)
	}
	if !custodyIsSQLiteCacheError(err) {
		return nil, noopRelease, false, InternalError(fmt.Sprintf("open workspace: %v", err))
	}
	fallback, release, fbErr := s.custodyDBOptionalWorkspace(ctx)
	if fbErr != nil {
		return nil, noopRelease, false, InternalError(fmt.Sprintf("open custody workspace without sqlite cache: %v", errors.Join(err, fbErr)))
	}
	logger.Warn("custody live operation running without the SQLite cache", "error", err)
	return fallback, release, true, nil
}

// custodyIsSQLiteCacheError reports whether a workspace open failed at the SQLite
// cache: opening the database file or preparing its schema.
func custodyIsSQLiteCacheError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "open database:") || strings.Contains(msg, "ensure schema:")
}

// custodyDBOptionalWorkspace opens a canonical workspace with no SQLite handle
// (DB == nil). Custody mutations locate shipments on disk and skip index writes
// when DB is nil. Live lock acquisition still runs the pending shipment-operation
// recovery barrier. Recovery of non-custody journals needs the SQLite handle, so
// the fallback is refused while one is pending.
func (s *Server) custodyDBOptionalWorkspace(ctx context.Context) (*core.Workspace, func(), error) {
	storageRoot := s.backlogitDir()
	if err := custodyRequireCustodyOnlyJournals(storageRoot); err != nil {
		return nil, noopRelease, err
	}
	ws, err := core.NewReadOnlyWorkspace(ctx, s.RootPath)
	if err != nil {
		return nil, noopRelease, fmt.Errorf("open canonical workspace: %w", err)
	}
	headerDef, err := config.LoadHeaderDef(ws.StorageRoot)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, noopRelease, errors.Join(fmt.Errorf("load header-def: %w", err), ws.Close())
		}
		headerDef = nil
	}
	ws.HeaderDef = headerDef
	return ws, func() {
		if cerr := ws.Close(); cerr != nil {
			logger.Warn("close custody fallback workspace", "error", cerr)
		}
	}, nil
}

// custodyRequireCustodyOnlyJournals refuses the SQLite-free fallback when a pending
// shipment-operation journal is not a custody journal. Such recovery needs SQLite.
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
// mirror the core shipment-operation constants.
func custodyOperation(op string) bool {
	return op == "custody_offer" || op == "custody_accept"
}

// custodyJSONArg decodes a JSON-encoded string argument into dst.
func custodyJSONArg(args map[string]any, key string, dst any) error {
	raw, _ := args[key].(string)
	if strings.TrimSpace(raw) == "" {
		return fmt.Errorf("%s is required", key)
	}
	if err := json.Unmarshal([]byte(raw), dst); err != nil {
		return fmt.Errorf("%s must be valid JSON: %w", key, err)
	}
	return nil
}

func custodyNonNil[T any](items []T) []T {
	if items == nil {
		return []T{}
	}
	return items
}

func (s *Server) handleCustodyPreview(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	args := request.Params.Arguments
	shipmentID, _ := args["shipment_id"].(string)
	if shipmentID == "" {
		return ValidationFailed("shipment_id is required"), nil
	}
	var candidates []core.CustodyCandidateSpec
	if err := custodyJSONArg(args, "candidates", &candidates); err != nil {
		return ValidationFailed(err.Error()), nil
	}
	branch, _ := args["branch"].(string)
	ws, release, result := s.custodyReadOnlyWorkspace(ctx)
	if result != nil {
		return result, nil
	}
	defer release()
	logger.Info("custody tool invoked", "tool", "backlogit_custody_preview", "shipment_id", shipmentID)

	preview, err := core.PreviewCustodyCandidates(ctx, ws, core.CustodyPreviewOptions{
		ShipmentID: shipmentID,
		Branch:     branch,
		Candidates: candidates,
	})
	if err != nil {
		return custodyError("custody preview", err), nil
	}
	preview.Candidates = custodyNonNil(preview.Candidates)
	preview.Warnings = custodyNonNil(preview.Warnings)
	return toolResultJSON(preview)
}

func (s *Server) handleCustodyVerify(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	args := request.Params.Arguments
	shipmentID, _ := args["shipment_id"].(string)
	if shipmentID == "" {
		return ValidationFailed("shipment_id is required"), nil
	}
	branch, _ := args["branch"].(string)
	ws, release, result := s.custodyReadOnlyWorkspace(ctx)
	if result != nil {
		return result, nil
	}
	defer release()
	logger.Info("custody tool invoked", "tool", "backlogit_custody_verify", "shipment_id", shipmentID)

	verified, err := core.VerifyCustodyState(ctx, ws, core.CustodyVerifyOptions{
		ShipmentID: shipmentID,
		Branch:     branch,
	})
	if err != nil {
		return custodyError("custody verify", err), nil
	}
	verified.Verdicts = custodyNonNil(verified.Verdicts)
	verified.Warnings = custodyNonNil(verified.Warnings)
	return toolResultJSON(verified)
}

func (s *Server) handleCustodyOffer(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	args := request.Params.Arguments
	shipmentID, _ := args["shipment_id"].(string)
	if shipmentID == "" {
		return ValidationFailed("shipment_id is required"), nil
	}
	var spec core.CustodyReceipt
	if err := custodyJSONArg(args, "spec", &spec); err != nil {
		return ValidationFailed(err.Error()), nil
	}
	dryRun, _ := args["dry_run"].(bool)
	ws, release, degraded, result := s.custodyWorkspaceFor(ctx, dryRun)
	if result != nil {
		return result, nil
	}
	defer release()
	logger.Info("custody tool invoked", "tool", "backlogit_custody_offer",
		"shipment_id", shipmentID, "dry_run", dryRun, "db_unavailable", degraded)

	offered, err := core.OfferCustodyHandoff(ctx, ws, core.CustodyOfferOptions{
		ShipmentID: shipmentID,
		Spec:       spec,
		DryRun:     dryRun,
	})
	if err != nil {
		return custodyError("custody offer", err), nil
	}
	return custodyMutationResultJSON(offered, degraded)
}

func (s *Server) handleCustodyAccept(ctx context.Context, request mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	args := request.Params.Arguments
	shipmentID, _ := args["shipment_id"].(string)
	if shipmentID == "" {
		return ValidationFailed("shipment_id is required"), nil
	}
	receiptID, _ := args["receipt_id"].(string)
	if receiptID == "" {
		return ValidationFailed("receipt_id is required"), nil
	}
	by, _ := args["by"].(string)
	if by == "" {
		return ValidationFailed("by is required"), nil
	}
	idempotencyKey, _ := args["idempotency_key"].(string)
	authToken, _ := args["auth_token"].(string)
	dryRun, _ := args["dry_run"].(bool)
	ws, release, degraded, result := s.custodyWorkspaceFor(ctx, dryRun)
	if result != nil {
		return result, nil
	}
	defer release()
	logger.Info("custody tool invoked", "tool", "backlogit_custody_accept",
		"shipment_id", shipmentID, "receipt_id", receiptID, "dry_run", dryRun, "db_unavailable", degraded)

	accepted, err := core.AcceptCustodyHandoff(ctx, ws, core.CustodyAcceptOptions{
		ShipmentID:     shipmentID,
		ReceiptID:      receiptID,
		By:             by,
		IdempotencyKey: idempotencyKey,
		AuthToken:      authToken,
		DryRun:         dryRun,
	})
	if err != nil {
		return custodyError("custody accept", err), nil
	}
	return custodyMutationResultJSON(accepted, degraded)
}

// custodyMutationResultJSON encodes a live or dry-run custody mutation result with
// non-nil collections and the degraded database marker when the cache was bypassed.
func custodyMutationResultJSON(result core.CustodyMutationResult, degraded bool) (*mcplib.CallToolResult, error) {
	result.Verdicts = custodyNonNil(result.Verdicts)
	result.Warnings = custodyNonNil(result.Warnings)
	resp := custodyMutationResponse{CustodyMutationResult: result}
	if degraded {
		resp.DB = custodyDBUnavailable
	}
	return toolResultJSON(resp)
}

// custodyError maps core custody refusals to structured MCP error codes. Each
// refusal stays fail-closed; unknown failures use the shared domain mapping.
func custodyError(op string, err error) *mcplib.CallToolResult {
	code := ""
	switch {
	case errors.Is(err, core.ErrCustodyDefaultBranchForbidden):
		code = "custody_default_branch_forbidden"
	case errors.Is(err, core.ErrNonceProviderUnavailable):
		code = "nonce_provider_unavailable"
	case errors.Is(err, core.ErrNonceReplayed):
		code = "nonce_replayed"
	case errors.Is(err, core.ErrAuthorizationRequired):
		code = "authorization_required"
	case errors.Is(err, core.ErrApprovalChannelUnavailable):
		code = "approval_channel_unavailable"
	case errors.Is(err, core.ErrAuthorizationRejected):
		code = "authorization_rejected"
	case errors.Is(err, core.ErrApprovalWrongShipment):
		code = "approval_wrong_shipment"
	case errors.Is(err, core.ErrApprovalWrongRequestIdentity):
		code = "approval_wrong_request_identity"
	case errors.Is(err, core.ErrTrustAnchorRevoked):
		code = "trust_anchor_revoked"
	case errors.Is(err, core.ErrOperatorApprovalRequired):
		code = "operator_approval_required"
	case errors.Is(err, core.ErrCustodyStaleDigest):
		code = "custody_stale_digest"
	case errors.Is(err, core.ErrCustodyWrongBranch):
		code = "custody_wrong_branch"
	case errors.Is(err, core.ErrMixedIndexWorktreeDivergence):
		code = "custody_index_worktree_divergence"
	default:
		return domainError(op, err)
	}
	return makeErrorResult(code, fmt.Sprintf("%s: %v", op, err))
}
