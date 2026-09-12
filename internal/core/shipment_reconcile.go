package core

import "context"

// This file declares the U1 request/result/branch types and the SIGNATURES
// of the reconcile primitives for governed shipment-reconciliation-to-shipped
// (#423, 167-F). Every function body below is a gated DECLARATION (panic),
// not a production stub: each owner task lands its own behavior harness
// (RED against the panic body) and real implementation in a strictly later
// wave, per workflow-policies.md P-002.1/P-002.6:
//
//   - classifyShipmentReconcileState -> 167.010-T (classifier)
//   - ReconcileShipmentToShipped     -> 167.008-T (transaction)
//   - writeShipmentReconcileArchiveFile -> 167.006-T (handle-relative atomic writer)
//   - lockShipmentReconcileItemLog      -> 167.011-T (item-log lock)
//   - appendShipmentReconcileEvent      -> 167.007-T (durable fsync appender)
//   - snapshotShipmentReconcile / restoreShipmentReconcile -> 167.016-T (snapshot/rollback)
//
// The signatures reference only this task's own request/result/branch types
// plus stdlib (context.Context, *Workspace, []byte, error) — the U3 event
// constant (167.002-T) and reconcile.trusted_refs config (167.012-T) are
// consumed only inside the later-landed bodies, not these signatures, so
// this file stays dependency-free (no cycle).

// ShipmentReconcileOutcome enumerates the exhaustive, mutually-exclusive
// terminal branches of a governed shipment reconciliation attempt (#423,
// U1). The full branch derivation lands in 167.010-T.
type ShipmentReconcileOutcome string

const (
	// ShipmentReconcileOutcomeReconciled is the normal-repair success branch (2d).
	ShipmentReconcileOutcomeReconciled ShipmentReconcileOutcome = "reconciled"
	// ShipmentReconcileOutcomeNoOp is the idempotent-replay branch (1b/2b).
	ShipmentReconcileOutcomeNoOp ShipmentReconcileOutcome = "no_op"
	// ShipmentReconcileOutcomeConflict is a same-key/different-request-identity
	// or unsupported-history branch (1c/2c).
	ShipmentReconcileOutcomeConflict ShipmentReconcileOutcome = "conflict"
	// ShipmentReconcileOutcomeIndeterminate covers unreadable/path-unsafe logs,
	// malformed JSONL, torn resume state, and ErrWriteIndeterminate writes.
	ShipmentReconcileOutcomeIndeterminate ShipmentReconcileOutcome = "indeterminate"
)

// ShipmentShippedReconcileRequest is the caller-supplied input to
// ReconcileShipmentToShipped (#423, U1/U2/U4).
type ShipmentShippedReconcileRequest struct {
	ShipmentID      string
	Reason          string
	Actor           string
	SecondApprover  string
	IdempotencyKey  string
	MergeSHA        string
	ClosureEvidence string
	EvidenceRefs    []string
	DryRun          bool
}

// ShipmentShippedReconcileResult is the outcome of a governed shipment
// reconciliation attempt (#423, U1).
type ShipmentShippedReconcileResult struct {
	Outcome    ShipmentReconcileOutcome
	ShipmentID string
	DryRun     bool
	Message    string
}

// classifyShipmentReconcileState is the shared, pure, read-only total state
// classifier consumed by both the U2 precondition gate (167.014-T) and the
// U1 transaction (167.008-T). See shipment_reconcile_classifier.go for the
// implementation landed by 167.010-T.
func classifyShipmentReconcileState(log []byte, frontmatter map[string]any, reqIdempotencyKey string, reqRequestIdentityDigest string) (ShipmentReconcileOutcome, error) {
	return classifyShipmentReconcileStateImpl(log, frontmatter, reqIdempotencyKey, reqRequestIdentityDigest)
}

// ReconcileShipmentToShipped is the governed two-phase reconciliation
// transaction entry point (#423, U1). DECLARATION ONLY — panic body gated
// by the source-shape harness; the behavior harness (RED) and
// implementation (GREEN) land in 167.008-T.
func ReconcileShipmentToShipped(ctx context.Context, ws *Workspace, req ShipmentShippedReconcileRequest) (ShipmentShippedReconcileResult, error) {
	panic("not implemented: ReconcileShipmentToShipped (167.008-T)")
}

// writeShipmentReconcileArchiveFile is the handle-relative atomic
// archive-file writer primitive (#423, U1). DECLARATION ONLY — panic body
// gated by the source-shape harness; the behavior harness (RED) and
// implementation (GREEN) land in 167.006-T.
func writeShipmentReconcileArchiveFile(ctx context.Context, ws *Workspace, shipmentID string, content []byte) error {
	return writeShipmentReconcileArchiveFileWithSeams(ctx, ws, shipmentID, content, defaultShipmentReconcileFSSeams())
}

// lockShipmentReconcileItemLog is the handle-relative, stable-identity
// item-log (C) lock primitive (#423, U1). It targets the IDENTICAL canonical
// sidecar resource events.LockItemLogCrossProcess uses (events.ItemLogLockPath,
// 167.017-T) so it mutually excludes existing writers (AssociateCommit,
// ArchiveItem, LinkCommit), while opening that sidecar directory-handle-relative
// with no-follow/reparse-safe semantics rather than by a freshly recomputed
// pathname. See shipment_reconcile_lock.go for the implementation (167.011-T).
func lockShipmentReconcileItemLog(ctx context.Context, ws *Workspace, itemID string) (context.Context, func() error, error) {
	return lockShipmentReconcileItemLogImpl(ctx, ws, itemID)
}

// appendShipmentReconcileEvent is the always-fsync durable event append
// primitive that runs UNDER the caller's already-held item-log lock (#423,
// U1); it MUST NOT re-acquire that lock. See shipment_reconcile_append.go
// for the implementation (167.007-T).
func appendShipmentReconcileEvent(ctx context.Context, ws *Workspace, itemID string, eventBytes []byte) error {
	return appendShipmentReconcileEventImpl(ctx, ws, itemID, eventBytes)
}

// shipmentReconcileSnapshot captures the pre-mutation archive file bytes and
// full SQLite index row consumed by the transactional snapshot/rollback
// primitive (#423, U1).
//
//nolint:unused // gated declaration; owner task 167.016-T lands the real call site
type shipmentReconcileSnapshot struct {
	ShipmentID string
	FileBytes  []byte
	RowPresent bool
	Row        map[string]any
}

// snapshotShipmentReconcile captures pre-mutation state for rollback (#423,
// U1, "snapshot(shipment)"). DECLARATION ONLY — panic body gated by the
// source-shape harness; the behavior harness (RED) and implementation
// (GREEN) land in 167.016-T.
//
//nolint:unused // gated declaration; owner task 167.016-T lands the real call site
func snapshotShipmentReconcile(ctx context.Context, ws *Workspace, shipmentID string) (shipmentReconcileSnapshot, error) {
	panic("not implemented: snapshotShipmentReconcile (167.016-T)")
}

// restoreShipmentReconcile restores a previously captured snapshot (#423,
// U1, "restore(snapshot)"). DECLARATION ONLY — panic body gated by the
// source-shape harness; the behavior harness (RED) and implementation
// (GREEN) land in 167.016-T.
//
//nolint:unused // gated declaration; owner task 167.016-T lands the real call site
func restoreShipmentReconcile(ctx context.Context, ws *Workspace, snapshot shipmentReconcileSnapshot) error {
	panic("not implemented: restoreShipmentReconcile (167.016-T)")
}
