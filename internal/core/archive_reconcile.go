package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/softwaresalt/backlogit/internal/db"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/models"
)

// validReconciliationTargetStatuses is the set of allowed target statuses for reconciliation.
var validReconciliationTargetStatuses = map[string]struct{}{
	"done":      {},
	"accepted":  {},
	"rejected":  {},
	"abandoned": {},
	"shipped":   {},
}

// ReconciliationOutcome describes the per-item or overall outcome of a lifecycle reconciliation.
type ReconciliationOutcome string

const (
	// ReconciliationCompleted indicates the item was successfully reconciled.
	ReconciliationCompleted ReconciliationOutcome = "completed"
	// ReconciliationNoOp indicates the item was already at the target status.
	ReconciliationNoOp ReconciliationOutcome = "no_op"
	// ReconciliationPartial indicates some items were reconciled and some were not.
	ReconciliationPartial ReconciliationOutcome = "partial"
	// ReconciliationIndeterminate indicates a durable write outcome is unknown.
	ReconciliationIndeterminate ReconciliationOutcome = "indeterminate"
)

// ReconciliationRequest is the input to ReconcileArchivedLifecycle.
type ReconciliationRequest struct {
	// ItemIDs is the list of archived item IDs to reconcile. At least one required.
	ItemIDs []string
	// TargetStatus is the terminal status to transition to before re-archiving.
	// Defaults to "done" when empty.
	TargetStatus string
	// Reason is a human-readable explanation for the reconciliation. Required.
	Reason string
	// Actor is the operator or agent performing the reconciliation. Required.
	Actor string
	// IdempotencyKey is an optional caller-supplied key. If a previously reconciled
	// item already carries this key in custom_fields, the item is returned as no_op.
	IdempotencyKey string
}

// ReconciliationItemResult is the per-item outcome.
type ReconciliationItemResult struct {
	// ID is the item ID.
	ID string
	// Outcome is the per-item outcome.
	Outcome ReconciliationOutcome
	// Error is a human-readable error message, empty when no error.
	Error string
}

// ReconciliationResult is the structured outcome of ReconcileArchivedLifecycle.
type ReconciliationResult struct {
	// Items contains per-item results in the same order as the request.
	Items []ReconciliationItemResult
	// Outcome is the overall outcome across all items.
	Outcome ReconciliationOutcome
}

// ReconcileArchivedLifecycle is a governed, fail-closed operation that corrects
// archived items whose archived_status does not reflect the correct terminal status.
// For each item it executes: unarchive → status update → re-archive, recording a
// durable lifecycle_reconciliation event and preserving the original archive history.
//
// The operation is idempotent: items already at the target status return no_op.
// ErrWriteIndeterminate at any step causes the item to be recorded as indeterminate
// and the operation continues to the next item without rollback.
//
// When every item in the batch fails, a function-level error is returned (with the
// item errors joined) so single-item callers receive a conventional Go error.
// Batches with at least one success return (result, nil) with ReconciliationPartial.
func ReconcileArchivedLifecycle(ctx context.Context, database *sql.DB, ws *Workspace, req ReconciliationRequest) (*ReconciliationResult, error) {
	if req.Reason == "" {
		return nil, fmt.Errorf("reason: required: %w", blerrors.ErrValidation)
	}
	if req.Actor == "" {
		return nil, fmt.Errorf("actor: required: %w", blerrors.ErrValidation)
	}
	if len(req.ItemIDs) == 0 {
		return nil, fmt.Errorf("item_ids: at least one required: %w", blerrors.ErrValidation)
	}

	targetStatus := req.TargetStatus
	if targetStatus == "" {
		targetStatus = "done"
	}
	if _, ok := validReconciliationTargetStatuses[targetStatus]; !ok {
		return nil, fmt.Errorf(
			"target_status %q is not a valid reconciliation target (must be one of: done, accepted, rejected, abandoned, shipped): %w",
			targetStatus, blerrors.ErrValidation,
		)
	}

	for _, id := range req.ItemIDs {
		if id == "" {
			return nil, fmt.Errorf("item_ids: empty ID not allowed: %w", blerrors.ErrValidation)
		}
		if strings.Contains(id, "..") || strings.ContainsAny(id, "/\\") {
			return nil, fmt.Errorf("item_ids: ID %q contains path traversal characters: %w", id, blerrors.ErrValidation)
		}
	}

	lockedCtx, unlock, lockErr := lockArtifactMutations(ctx, ws, req.ItemIDs)
	if lockErr != nil {
		return nil, fmt.Errorf("acquire reconciliation locks: %w", lockErr)
	}
	defer func() { _ = unlock() }()

	result := &ReconciliationResult{Items: make([]ReconciliationItemResult, 0, len(req.ItemIDs))}
	var itemErrors []error
	for _, itemID := range req.ItemIDs {
		itemResult, itemErr := reconcileArchivedItem(
			lockedCtx, database, ws,
			itemID, targetStatus,
			req.Reason, req.Actor, req.IdempotencyKey,
		)
		result.Items = append(result.Items, itemResult)
		if itemErr != nil {
			itemErrors = append(itemErrors, itemErr)
		}
	}

	result.Outcome = computeReconciliationOutcome(result.Items)

	// When every item failed, surface a function-level error so single-item
	// callers get a conventional Go error. errors.Join preserves the sentinel
	// chain so errors.Is(err, blerrors.ErrNotFound) works for not-found items.
	if len(itemErrors) == len(req.ItemIDs) && len(req.ItemIDs) > 0 {
		return nil, errors.Join(itemErrors...)
	}
	return result, nil
}

// reconcileArchivedItem performs the unarchive → status-update → re-archive
// sequence for a single item. It returns both the per-item result struct and the
// underlying error so ReconcileArchivedLifecycle can aggregate errors for the
// all-items-failed case.
func reconcileArchivedItem(
	ctx context.Context,
	database *sql.DB,
	ws *Workspace,
	itemID, targetStatus, reason, actor, idempotencyKey string,
) (ReconciliationItemResult, error) {
	// Step 1: Locate the artifact (searches queue and archive directories).
	artifactPath, err := FindArtifactPath(ctx, ws, itemID)
	if err != nil {
		// FindArtifactPath already wraps blerrors.ErrNotFound for missing items;
		// re-wrap with context so errors.Is traversal works from the call site.
		wrErr := fmt.Errorf("find artifact %s: %w", itemID, err)
		return ReconciliationItemResult{ID: itemID, Outcome: ReconciliationPartial, Error: wrErr.Error()}, wrErr
	}

	// Step 2: Read the artifact file.
	raw, err := os.ReadFile(artifactPath)
	if err != nil {
		wrErr := fmt.Errorf("read artifact %s: %w", itemID, err)
		return ReconciliationItemResult{ID: itemID, Outcome: ReconciliationPartial, Error: wrErr.Error()}, wrErr
	}

	// Step 3: Parse frontmatter.
	fm, _, err := models.ParseFrontmatter(string(raw))
	if err != nil {
		wrErr := fmt.Errorf("parse artifact %s: %w", itemID, err)
		return ReconciliationItemResult{ID: itemID, Outcome: ReconciliationPartial, Error: wrErr.Error()}, wrErr
	}

	// Step 4: Validate that the item is actually archived (has an archived_status).
	archivedStatus, _ := fm["archived_status"].(string)
	if archivedStatus == "" {
		wrErr := fmt.Errorf("item %s is not archived (no archived_status field)", itemID)
		return ReconciliationItemResult{ID: itemID, Outcome: ReconciliationPartial, Error: wrErr.Error()}, wrErr
	}

	// Step 5: NoOp — already at the target archived_status.
	if archivedStatus == targetStatus {
		return ReconciliationItemResult{ID: itemID, Outcome: ReconciliationNoOp}, nil
	}

	// Step 6: Idempotency key check — item was already reconciled with this key.
	if idempotencyKey != "" {
		if cf, ok := fm["custom_fields"].(map[string]any); ok {
			if existing, _ := cf["reconciliation_idempotency_key"].(string); existing == idempotencyKey {
				return ReconciliationItemResult{ID: itemID, Outcome: ReconciliationNoOp}, nil
			}
		}
	}

	// Step 8: Verify all children have reached a terminal status.
	if err := CheckChildrenTerminal(ctx, database, itemID); err != nil {
		wrErr := fmt.Errorf("check children terminal %s: %w", itemID, err)
		return ReconciliationItemResult{ID: itemID, Outcome: ReconciliationPartial, Error: wrErr.Error()}, wrErr
	}

	// Step 9: Unarchive — restores the item to the queue with status = archivedStatus.
	if err := UnarchiveItem(ctx, database, ws, itemID); err != nil {
		if blerrors.IsWriteIndeterminate(err) {
			wrErr := fmt.Errorf("unarchive %s: write outcome indeterminate: %w", itemID, err)
			return ReconciliationItemResult{ID: itemID, Outcome: ReconciliationIndeterminate, Error: wrErr.Error()}, wrErr
		}
		wrErr := fmt.Errorf("unarchive %s: %w", itemID, err)
		return ReconciliationItemResult{ID: itemID, Outcome: ReconciliationPartial, Error: wrErr.Error()}, wrErr
	}

	// Step 10: Build the reconciliation metadata.
	cfUpdates := map[string]any{
		"reconciled_at":            time.Now().UTC().Format(time.RFC3339),
		"reconciliation_reason":    reason,
		"reconciliation_actor":     actor,
		"original_archived_status": archivedStatus,
	}
	if idempotencyKey != "" {
		cfUpdates["reconciliation_idempotency_key"] = idempotencyKey
	}

	// Step 11: Apply status update with reconciliation metadata directly to the
	// frontmatter and DB. A targeted write is used in place of updateArtifactUngated
	// because the full pipeline requires a loaded registry and header-def that may
	// not be present in bare workspaces (e.g. reconciliation callers that hold only
	// a RootPath+DB). The reconciliation contract only needs status + custom_fields;
	// hook firing and relocation routing are intentionally skipped here.
	if err := setItemStatusAndMeta(ctx, database, ws, itemID, targetStatus, cfUpdates); err != nil {
		if blerrors.IsWriteIndeterminate(err) {
			// Possibly applied — do NOT rollback; the caller must reconcile externally.
			wrErr := fmt.Errorf("update status %s: write outcome indeterminate: %w", itemID, err)
			return ReconciliationItemResult{ID: itemID, Outcome: ReconciliationIndeterminate, Error: wrErr.Error()}, wrErr
		}
		// Update failed cleanly; re-archive to restore the original archived state.
		if _, archErr := ArchiveItem(ctx, database, ws, itemID, WithCascade(false), WithTopLevel(false)); archErr != nil {
			wrErr := fmt.Errorf("update %s failed and rollback archive also failed: update: %v; rollback: %w", itemID, err, archErr)
			return ReconciliationItemResult{ID: itemID, Outcome: ReconciliationPartial, Error: wrErr.Error()}, wrErr
		}
		wrErr := fmt.Errorf("update status %s: %w", itemID, err)
		return ReconciliationItemResult{ID: itemID, Outcome: ReconciliationPartial, Error: wrErr.Error()}, wrErr
	}

	// Step 12: Re-archive the item — archived_status is stamped as targetStatus by ArchiveItem.
	if _, err := ArchiveItem(ctx, database, ws, itemID, WithCascade(false), WithTopLevel(false)); err != nil {
		if blerrors.IsWriteIndeterminate(err) {
			wrErr := fmt.Errorf("re-archive %s: write outcome indeterminate: %w", itemID, err)
			return ReconciliationItemResult{ID: itemID, Outcome: ReconciliationIndeterminate, Error: wrErr.Error()}, wrErr
		}
		// Forward-recovery: item is at targetStatus in queue; operator must archive manually.
		_ = appendItemEventErr(ctx, ws, itemID, "lifecycle_reconciliation_forward_recovery", map[string]any{
			"reason":                   reason,
			"actor":                    actor,
			"original_archived_status": archivedStatus,
			"target_status":            targetStatus,
			"forward_recovery_error":   err.Error(),
		})
		return ReconciliationItemResult{
			ID:      itemID,
			Outcome: ReconciliationCompleted,
			Error:   fmt.Sprintf("re-archive failed (forward-recovery: item at %s in queue): %v", targetStatus, err),
		}, nil
	}

	// Step 13: Append a durable lifecycle_reconciliation event to the item log.
	_ = appendItemEventErr(ctx, ws, itemID, "lifecycle_reconciliation", map[string]any{
		"reason":                   reason,
		"actor":                    actor,
		"original_archived_status": archivedStatus,
		"new_archived_status":      targetStatus,
		"idempotency_key":          idempotencyKey,
	})

	return ReconciliationItemResult{ID: itemID, Outcome: ReconciliationCompleted}, nil
}

// setItemStatusAndMeta writes a targeted status + custom_fields update directly
// to an artifact's frontmatter and DB index. It is a lightweight alternative to
// updateArtifactUngated for the reconciliation path, which only needs to change
// two fields on an already-unarchived item without triggering registry loading,
// header-def validation, status-routing relocation, or hook execution.
//
// The write is atomic via replaceFileWithOptions. On a successful file write
// followed by a DB failure the file is restored from its pre-write snapshot so
// the caller's rollback path (re-archive) sees the original status.
func setItemStatusAndMeta(ctx context.Context, database *sql.DB, ws *Workspace, itemID, status string, cfUpdates map[string]any) error {
	artifactPath, err := FindArtifactPath(ctx, ws, itemID)
	if err != nil {
		return fmt.Errorf("find artifact: %w", err)
	}
	rawBefore, err := os.ReadFile(artifactPath)
	if err != nil {
		return fmt.Errorf("read artifact: %w", err)
	}
	fm, body, err := models.ParseFrontmatter(string(rawBefore))
	if err != nil {
		return fmt.Errorf("parse artifact: %w", err)
	}

	fm["status"] = status
	fm["updated_at"] = models.NowUTC()

	existing, _ := fm["custom_fields"].(map[string]any)
	if existing == nil {
		existing = map[string]any{}
	}
	for k, v := range cfUpdates {
		existing[k] = v
	}
	fm["custom_fields"] = existing

	newContent := models.SerializeFrontmatter(fm, body)
	if writeErr := replaceFileWithOptions(ws, artifactPath, []byte(newContent)); writeErr != nil {
		// Surface indeterminate or definite write failures unchanged so the
		// caller can distinguish them and apply the correct recovery strategy.
		return writeErr
	}

	// Update the DB index to reflect the new status. Re-parse the written file
	// so the artifact struct is authoritative. On failure, restore the original
	// file content so the caller's rollback path (ArchiveItem) sees archivedStatus.
	artifact, _, parseErr := parseFile(artifactPath)
	if parseErr != nil {
		_ = replaceFileWithOptions(ws, artifactPath, rawBefore) // best-effort restore
		return fmt.Errorf("re-parse artifact after write: %w", parseErr)
	}
	if upsertErr := db.UpsertItem(ctx, database, artifact); upsertErr != nil {
		_ = replaceFileWithOptions(ws, artifactPath, rawBefore) // best-effort restore
		return fmt.Errorf("upsert item: %w", upsertErr)
	}
	return nil
}

// computeReconciliationOutcome derives the overall batch outcome from the
// per-item results. Precedence (highest to lowest): indeterminate → partial →
// completed → no_op.
func computeReconciliationOutcome(items []ReconciliationItemResult) ReconciliationOutcome {
	if len(items) == 0 {
		return ReconciliationNoOp
	}
	var hasCompleted, hasPartial, hasIndeterminate bool
	for _, item := range items {
		switch item.Outcome {
		case ReconciliationCompleted:
			hasCompleted = true
		case ReconciliationPartial:
			hasPartial = true
		case ReconciliationIndeterminate:
			hasIndeterminate = true
		}
	}
	if hasIndeterminate {
		return ReconciliationIndeterminate
	}
	if hasPartial {
		return ReconciliationPartial
	}
	if hasCompleted {
		return ReconciliationCompleted
	}
	return ReconciliationNoOp
}
