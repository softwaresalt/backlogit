package core

import (
	"context"
	"database/sql"
	"errors"
)

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
func ReconcileArchivedLifecycle(_ context.Context, _ *sql.DB, _ *Workspace, _ ReconciliationRequest) (*ReconciliationResult, error) {
	return nil, errors.New("backlogit: ReconcileArchivedLifecycle not implemented")
}