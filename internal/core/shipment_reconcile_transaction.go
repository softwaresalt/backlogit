package core

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	bldb "github.com/softwaresalt/backlogit/internal/db"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/models"
)

// shipmentReconcileManifestDigestDomain domain-separates the manifest-member
// digest recorded on the EventShipmentReconciledShipped evidence sub-object
// from every other shipmentReconcileDigestHex consumer (167.008-T).
const shipmentReconcileManifestDigestDomain = "backlogit/shipment-reconcile/manifest/v1"

// reconcileShipmentToShippedImpl is the real implementation behind the
// gated ReconcileShipmentToShipped declaration (167.003-T panic body,
// 167.008-T behavior). See the function's package doc comment in
// shipment_reconcile.go for the exported contract; this file implements the
// governed two-phase (A / B / C / D) transaction described by 167.008-T's
// backlog task body:
//
//   - PHASE A: acquire lockShipmentMembership(shipmentID), HELD ACROSS EVERY
//     later phase; take a cheap, lock-free read of the shipment manifest to
//     freeze the "Phase-A member set" used both to size the Phase B/D lock
//     batch and as the authoritative membership snapshot Phase D re-checks
//     against; compute the cheap request-identity digest (no file I/O).
//   - PHASE B: acquire the item-log lock C, then the sorted artifact-mutation
//     batch B (C-then-B, matching AssociateCommit); reload the shipment from
//     Markdown and re-run the identity/location + classifier gates under that
//     lock. Non-append outcomes (no_op/conflict/indeterminate) finalize and
//     return here (releasing B, C, then A). A no_op with the durable event
//     still absent (the "2b resume" case) appends the previously prepared
//     event here, under the same held C+B. Only the reconciled outcome
//     releases B and C (keeping A held) and continues to Phase C.
//   - PHASE C: (2d only, A alone held) the slow evidence gathering — merge
//     commit verification, closure evidence read/hash, canonical prepared
//     event construction — via prepareShipmentReconcileEvidence.
//   - PHASE D: re-acquire C then B; re-read the manifest and require the
//     member set still equal the frozen Phase-A set (else indeterminate);
//     re-check member terminality; re-classify (must still be reconciled,
//     else conflict/no_op/indeterminate); snapshot, write the archive
//     frontmatter, and durably append the prepared event, all under the same
//     held C+B, rolling back on ErrWriteNotApplied and leaving state
//     untouched (no restore, no append) on ErrWriteIndeterminate.
func reconcileShipmentToShippedImpl(ctx context.Context, ws *Workspace, req ShipmentShippedReconcileRequest) (ShipmentShippedReconcileResult, error) {
	shipmentID := strings.TrimSpace(req.ShipmentID)
	// Normalize the idempotency key once, up front, before it is used for
	// classification, log messages, or frontmatter persistence anywhere
	// downstream. shipmentReconcileRequestIdentityDigest independently
	// trims this same field when computing the digest (via
	// normalizeShipmentReconcileRequest), so a whitespace-padded key and its
	// unpadded twin already produce an IDENTICAL digest; without this
	// normalization here too, the classifier's separate raw string-equality
	// check on the idempotency key itself could still see them as
	// DIFFERENT keys and misclassify a legitimate same-key replay as a
	// conflict, and the persisted frontmatter value would be
	// non-canonical. Every downstream use of req.IdempotencyKey in this
	// file (classifier calls, log messages, frontmatter persistence) must
	// see this same trimmed value.
	req.IdempotencyKey = strings.TrimSpace(req.IdempotencyKey)
	result := ShipmentShippedReconcileResult{ShipmentID: shipmentID, DryRun: req.DryRun}

	if ws == nil {
		return result, fmt.Errorf("reconcile shipment to shipped: workspace is required: %w", blerrors.ErrValidation)
	}
	if shipmentID == "" {
		return result, fmt.Errorf("reconcile shipment to shipped: shipment id is required: %w", blerrors.ErrValidation)
	}

	// Cheap, lock-free, request-scalars-only digest (167.001-T). Also
	// validates the request's own required scalar fields (reason, actor,
	// idempotency_key, merge_sha shape, approver separation) fail-fast,
	// before any lock is ever acquired.
	requestIdentityDigest, err := shipmentReconcileRequestIdentityDigest(req)
	if err != nil {
		return result, fmt.Errorf("reconcile shipment to shipped: %w", err)
	}

	// PHASE A: membership lock, held across every later phase.
	unlockA, err := lockShipmentMembership(ctx, ws, shipmentID)
	if err != nil {
		return result, fmt.Errorf("reconcile shipment to shipped: acquire shipment %s membership lock: %w", shipmentID, err)
	}
	defer func() {
		if uerr := unlockA(); uerr != nil {
			slog.WarnContext(ctx, "reconcile shipment to shipped: release membership lock failed", "shipment_id", shipmentID, "error", uerr)
		}
	}()

	initialShipment, err := findArtifact(ctx, ws, shipmentID)
	if err != nil {
		return result, fmt.Errorf("reconcile shipment to shipped: load shipment %s: %w", shipmentID, err)
	}
	// The frozen "Phase-A set": every later phase re-validates membership
	// against exactly this snapshot, never a later re-read, so a concurrent
	// membership rewrite is detected as a mismatch (indeterminate) rather
	// than silently adopted mid-transaction. The RAW manifest is validated
	// (not silently normalized/repaired) before this set is frozen — see
	// shipmentReconcileValidateRawManifestItems.
	phaseAMemberIDs, err := shipmentReconcileValidateRawManifestItems(initialShipment)
	if err != nil {
		return result, err
	}

	return reconcileShipmentPhaseB(ctx, ws, req, shipmentID, phaseAMemberIDs, requestIdentityDigest, result)
}

// reconcileShipmentPhaseB acquires the item-log lock (C) then the sorted
// artifact-mutation batch (B), in that order, and re-validates the shipment
// under the held locks. Non-append outcomes finalize and return within this
// function (releasing B, C, and — via the deferred unlock installed by the
// caller — eventually A too). The reconciled outcome releases B and C before
// returning so Phase C's slow evidence I/O never runs under either lock.
func reconcileShipmentPhaseB(ctx context.Context, ws *Workspace, req ShipmentShippedReconcileRequest, shipmentID string, phaseAMemberIDs []string, requestIdentityDigest string, result ShipmentShippedReconcileResult) (ShipmentShippedReconcileResult, error) {
	lockIDs := append([]string{shipmentID}, phaseAMemberIDs...)
	ctxCB, releaseCB, err := lockShipmentReconcileCThenB(ctx, ws, shipmentID, lockIDs)
	if err != nil {
		return result, fmt.Errorf("reconcile shipment to shipped: acquire shipment %s item-log/artifact locks: %w", shipmentID, err)
	}
	released := false
	release := func() {
		if released {
			return
		}
		released = true
		if uerr := releaseCB(); uerr != nil {
			slog.WarnContext(ctx, "reconcile shipment to shipped: release item-log/artifact locks failed", "shipment_id", shipmentID, "error", uerr)
		}
	}
	defer release()

	shipment, frontmatter, err := loadShipmentReconcileArchivedShipment(ctxCB, ws, shipmentID)
	if err != nil {
		return result, err
	}
	// Identity/location gate only, unconditionally: safe to run before
	// classification because reconcile never mutates the top-level `status`
	// field. The legacy archived_status-value check is deferred past
	// classification (see the ShipmentReconcileOutcomeReconciled case below)
	// so that no_op/conflict replay outcomes against an already-shipped
	// shipment are recognized by the classifier instead of being rejected
	// here first.
	if err := validateShipmentReconcileShipmentIdentity(shipment); err != nil {
		return result, err
	}

	logBytes, err := shipmentReconcileReadItemLog(ws, shipmentID)
	if err != nil {
		return result, fmt.Errorf("reconcile shipment to shipped: read shipment %s item log: %w", shipmentID, err)
	}

	outcome, classifyErr := classifyShipmentReconcileState(logBytes, frontmatter, req.IdempotencyKey, requestIdentityDigest, shipmentID)
	if classifyErr != nil {
		result.Outcome = ShipmentReconcileOutcomeIndeterminate
		result.Message = fmt.Sprintf("shipment %s reconciliation state could not be determined", shipmentID)
		return result, fmt.Errorf("reconcile shipment to shipped: classify shipment %s: %w", shipmentID, classifyErr)
	}

	switch outcome {
	case ShipmentReconcileOutcomeConflict:
		result.Outcome = outcome
		result.Message = fmt.Sprintf("shipment %s reconciliation request conflicts with a different already-recorded reconciliation for idempotency key %q", shipmentID, req.IdempotencyKey)
		return result, nil
	case ShipmentReconcileOutcomeIndeterminate:
		// Defensive: classifyShipmentReconcileState always returns a non-nil
		// error alongside this outcome today; handled here in case that
		// contract is ever loosened.
		result.Outcome = outcome
		result.Message = fmt.Sprintf("shipment %s reconciliation state could not be determined", shipmentID)
		return result, fmt.Errorf("reconcile shipment to shipped: shipment %s classified as indeterminate: %w", shipmentID, blerrors.ErrValidation)
	case ShipmentReconcileOutcomeNoOp:
		return reconcileShipmentPhaseBNoOp(ctxCB, ws, req, shipmentID, frontmatter, logBytes, result)
	case ShipmentReconcileOutcomeReconciled:
		// Fresh-repair path only: the legacy archived_status-value check
		// guards against an unsupported/unexpected starting archived_status
		// (neither "active" nor "shipped") that classifyShipmentReconcileState's
		// EventAbsent branch would otherwise silently treat as "reconciled".
		// Deferred to here (past classification) rather than run
		// unconditionally in Phase B, so it never re-fires against an
		// already-shipped shipment on a no_op/conflict replay.
		if err := validateShipmentReconcileShipmentLegacyPreState(shipment); err != nil {
			return result, err
		}
		beforeArchivedStatus := shipment.ArchivedStatus
		// 2d normal-repair (live or dry-run alike): release B and C now (keep
		// A held) before Phase C's slow, un-lockable evidence I/O. A dry run
		// still runs Phase C's evidence verification and Phase D's
		// manifest/classification re-validation in full (167.008-T, PR #440
		// review finding 5): --dry-run evaluates EVERY precondition and only
		// skips the final writes, so it must not short-circuit here before
		// those preconditions ever run.
		release()
		return reconcileShipmentPhaseCAndD(ctx, ws, req, shipmentID, phaseAMemberIDs, requestIdentityDigest, beforeArchivedStatus, req.DryRun, result)
	default:
		return result, fmt.Errorf("reconcile shipment to shipped: shipment %s classifier returned unrecognized outcome %q", shipmentID, outcome)
	}
}

// reconcileShipmentPhaseBNoOp handles the classifier's NoOp outcome, which
// covers BOTH the "1b already fully durable" case (a matching event is
// already logged) and the "2b resume" case (no event logged yet, but
// complete, matching resume markers are already persisted in frontmatter —
// this function appends the persisted prepared event now, still under the
// caller's held C+B). It must run under the SAME held C+B the caller
// acquired for Phase B; it does not acquire or release any lock itself.
func reconcileShipmentPhaseBNoOp(ctx context.Context, ws *Workspace, req ShipmentShippedReconcileRequest, shipmentID string, frontmatter map[string]any, logBytes []byte, result ShipmentShippedReconcileResult) (ShipmentShippedReconcileResult, error) {
	result.Outcome = ShipmentReconcileOutcomeNoOp

	persisted, err := parseShipmentReconcilePersistedState(frontmatter)
	if err != nil {
		result.Outcome = ShipmentReconcileOutcomeIndeterminate
		return result, fmt.Errorf("reconcile shipment to shipped: re-parse shipment %s persisted resume state: %w", shipmentID, err)
	}
	preparedEventBytes, eventDigest := persisted.preparedEventBytesAndDigestIfPresent()

	loggedEvents, _, err := scanShipmentReconcileLog(logBytes, shipmentID, preparedEventBytes, eventDigest)
	if err != nil {
		// The classifier already parsed this exact log successfully; an
		// error here would mean the log changed shape between reads while
		// the item-log lock C was held, which should not be possible. Fail
		// closed rather than silently guessing.
		result.Outcome = ShipmentReconcileOutcomeIndeterminate
		return result, fmt.Errorf("reconcile shipment to shipped: re-scan shipment %s item log: %w", shipmentID, err)
	}
	if len(loggedEvents) > 0 {
		// 1b: already durably recorded — no further action.
		result.Message = fmt.Sprintf("shipment %s reconciliation already durably recorded for idempotency key %q; no action taken", shipmentID, req.IdempotencyKey)
		return result, nil
	}

	// 2b resume.
	if req.DryRun {
		result.Message = fmt.Sprintf("dry run: shipment %s has a previously prepared reconciliation event that would be appended to resume", shipmentID)
		return result, nil
	}

	preparedEventText, err := readRequiredRawStringField(persisted.preparedEvent, shipmentReconcilePreparedEventField)
	if err != nil {
		result.Outcome = ShipmentReconcileOutcomeIndeterminate
		return result, fmt.Errorf("reconcile shipment to shipped: read shipment %s persisted prepared event: %w", shipmentID, err)
	}

	if err := appendShipmentReconcileEvent(ctx, ws, shipmentID, []byte(preparedEventText)); err != nil {
		if blerrors.IsWriteIndeterminate(err) {
			result.Outcome = ShipmentReconcileOutcomeIndeterminate
		}
		return result, fmt.Errorf("reconcile shipment to shipped: resume-append shipment %s prepared event: %w", shipmentID, err)
	}

	result.Message = fmt.Sprintf("shipment %s resumed: previously prepared reconciliation event appended", shipmentID)
	return result, nil
}

// reconcileShipmentPhaseCAndD runs Phase C (slow evidence I/O, membership
// lock A alone held) followed by Phase D (re-acquire C then B, re-validate,
// and durably commit). It is only ever reached from the 2d normal-repair
// branch, with B and C already released and only A still held by the
// caller's own deferred unlock.
//
// dryRun routes the SAME Phase C evidence-verification and Phase D
// manifest/terminality/classification re-validation a live call performs
// (167.008-T, PR #440 review finding 5): the command's own documented
// contract is that --dry-run evaluates every precondition and only skips
// WRITES, so a dry run must not short-circuit before Phase C/D preconditions
// ever run (an invalid merge SHA, unresolvable closure evidence, or a
// non-terminal member must still fail a dry run exactly as it would fail a
// live call). When dryRun is true, this function releases the Phase D locks
// and returns the dry-run result IMMEDIATELY BEFORE the first write
// (snapshotShipmentReconcile / writeShipmentReconcileArchiveFile /
// appendShipmentReconcileEvent) — it never snapshots, writes the archive
// frontmatter, or appends the event.
func reconcileShipmentPhaseCAndD(ctx context.Context, ws *Workspace, req ShipmentShippedReconcileRequest, shipmentID string, phaseAMemberIDs []string, requestIdentityDigest string, beforeArchivedStatus string, dryRun bool, result ShipmentShippedReconcileResult) (ShipmentShippedReconcileResult, error) {
	manifestDigest := shipmentReconcileManifestDigest(phaseAMemberIDs)

	evidence, err := prepareShipmentReconcileEvidence(ctx, ws, req, shipmentReconcileEvidenceInput{
		ManifestDigest:       manifestDigest,
		MemberTerminalIDs:    phaseAMemberIDs,
		BeforeArchivedStatus: beforeArchivedStatus,
	})
	if err != nil {
		return result, fmt.Errorf("reconcile shipment to shipped: prepare shipment %s evidence: %w", shipmentID, err)
	}

	// PHASE D: re-acquire C then B, held together for the entire commit (or,
	// for a dry run, for the full read-only re-validation).
	lockIDs := append([]string{shipmentID}, phaseAMemberIDs...)
	ctxCB, releaseCB, err := lockShipmentReconcileCThenB(ctx, ws, shipmentID, lockIDs)
	if err != nil {
		return result, fmt.Errorf("reconcile shipment to shipped: re-acquire shipment %s item-log/artifact locks: %w", shipmentID, err)
	}
	released := false
	release := func() {
		if released {
			return
		}
		released = true
		if uerr := releaseCB(); uerr != nil {
			slog.WarnContext(ctx, "reconcile shipment to shipped: release phase D locks failed", "shipment_id", shipmentID, "error", uerr)
		}
	}
	defer release()

	shipment, frontmatter, err := loadShipmentReconcileArchivedShipment(ctxCB, ws, shipmentID)
	if err != nil {
		return result, err
	}
	// Phase D is only ever reached via Phase B's Reconciled branch (both
	// checks already passed there), so re-running the combined identity +
	// legacy-value check here unconditionally is correctly scoped and kept
	// as-is for clarity/symmetry with the pre-fix code.
	if err := validateShipmentReconcileShipmentPreState(shipment); err != nil {
		return result, err
	}

	currentMemberIDs, err := shipmentReconcileValidateRawManifestItems(shipment)
	if err != nil {
		return result, err
	}
	if !shipmentReconcileMemberSetsEqual(phaseAMemberIDs, currentMemberIDs) {
		result.Outcome = ShipmentReconcileOutcomeIndeterminate
		result.Message = fmt.Sprintf("shipment %s manifest member set changed since it was locked; refusing to commit", shipmentID)
		return result, fmt.Errorf("reconcile shipment to shipped: shipment %s manifest member set changed under lock (phase A set != phase D set): %w", shipmentID, blerrors.ErrValidation)
	}
	if err := validateShipmentReconcileManifestMembers(ctx, ws, currentMemberIDs); err != nil {
		return result, err
	}

	logBytes, err := shipmentReconcileReadItemLog(ws, shipmentID)
	if err != nil {
		return result, fmt.Errorf("reconcile shipment to shipped: re-read shipment %s item log: %w", shipmentID, err)
	}
	outcome, classifyErr := classifyShipmentReconcileState(logBytes, frontmatter, req.IdempotencyKey, requestIdentityDigest, shipmentID)
	if classifyErr != nil {
		result.Outcome = ShipmentReconcileOutcomeIndeterminate
		result.Message = fmt.Sprintf("shipment %s reconciliation state could not be determined", shipmentID)
		return result, fmt.Errorf("reconcile shipment to shipped: re-classify shipment %s: %w", shipmentID, classifyErr)
	}
	if outcome != ShipmentReconcileOutcomeReconciled {
		result.Outcome = outcome
		switch outcome {
		case ShipmentReconcileOutcomeNoOp:
			result.Message = fmt.Sprintf("shipment %s reconciliation was already completed by a concurrent transaction; no action taken", shipmentID)
			return result, nil
		case ShipmentReconcileOutcomeConflict:
			result.Message = fmt.Sprintf("shipment %s reconciliation request conflicts with a different already-recorded reconciliation for idempotency key %q", shipmentID, req.IdempotencyKey)
			return result, nil
		default:
			result.Message = fmt.Sprintf("shipment %s reconciliation state could not be determined", shipmentID)
			return result, fmt.Errorf("reconcile shipment to shipped: shipment %s re-classified as indeterminate under commit lock: %w", shipmentID, blerrors.ErrValidation)
		}
	}

	if dryRun {
		// Every Phase C/D precondition above has now been fully evaluated
		// (evidence verification, membership/terminality re-validation, and
		// re-classification all agree "reconciled"); release the locks and
		// report the planned outcome without ever reaching a write.
		release()
		result.Outcome = outcome
		result.Message = fmt.Sprintf("dry run: shipment %s would be reconciled from archived_status=%q to archived_status=%q", shipmentID, beforeArchivedStatus, string(ShipmentShipped))
		return result, nil
	}

	snapshot, err := snapshotShipmentReconcile(ctxCB, ws, shipmentID)
	if err != nil {
		return result, fmt.Errorf("reconcile shipment to shipped: snapshot shipment %s: %w", shipmentID, err)
	}

	newArchiveContent, err := shipmentReconcileBuildArchiveContent(shipmentID, snapshot.FileBytes, req.IdempotencyKey, requestIdentityDigest, evidence)
	if err != nil {
		return result, fmt.Errorf("reconcile shipment to shipped: build shipment %s archive content: %w", shipmentID, err)
	}

	if writeErr := writeShipmentReconcileArchiveFile(ctxCB, ws, shipmentID, newArchiveContent); writeErr != nil {
		if blerrors.IsWriteIndeterminate(writeErr) {
			result.Outcome = ShipmentReconcileOutcomeIndeterminate
			result.Message = fmt.Sprintf("shipment %s reconciliation write outcome is indeterminate; manual review required", shipmentID)
			return result, fmt.Errorf("reconcile shipment to shipped: write shipment %s archive file: %w", shipmentID, writeErr)
		}
		if restoreErr := restoreShipmentReconcile(ctxCB, ws, snapshot); restoreErr != nil {
			return result, fmt.Errorf("reconcile shipment to shipped: write shipment %s archive file not applied (%v) and rollback also failed: %w", shipmentID, writeErr, restoreErr)
		}
		return result, fmt.Errorf("reconcile shipment to shipped: write shipment %s archive file not applied, rolled back: %w", shipmentID, writeErr)
	}

	// The Markdown archive frontmatter is now durably the source of truth
	// for archived_status=shipped; keep the SQLite items row in sync
	// opportunistically (Copilot PR #440 review, finding 5), mirroring
	// appendShipmentReconcileEventImpl's own "index failure does not
	// un-reconcile" precedent: a sync failure here is logged as a warning
	// and never rolls back the already-durable Markdown write. Markdown
	// remains authoritative (doctor reads it directly); the index is
	// repaired by the next sync/reindex.
	syncShipmentReconcileItemIndex(ctxCB, ws, shipmentID, newArchiveContent)

	if appendErr := appendShipmentReconcileEvent(ctxCB, ws, shipmentID, evidence.EventBytes); appendErr != nil {
		if blerrors.IsWriteIndeterminate(appendErr) {
			result.Outcome = ShipmentReconcileOutcomeIndeterminate
			result.Message = fmt.Sprintf("shipment %s reconciliation append outcome is indeterminate; manual review required", shipmentID)
			return result, fmt.Errorf("reconcile shipment to shipped: append shipment %s reconciliation event: %w", shipmentID, appendErr)
		}
		if restoreErr := restoreShipmentReconcile(ctxCB, ws, snapshot); restoreErr != nil {
			return result, fmt.Errorf("reconcile shipment to shipped: append shipment %s reconciliation event not applied (%v) and rollback also failed: %w", shipmentID, appendErr, restoreErr)
		}
		return result, fmt.Errorf("reconcile shipment to shipped: append shipment %s reconciliation event not applied, rolled back: %w", shipmentID, appendErr)
	}

	result.Outcome = ShipmentReconcileOutcomeReconciled
	result.Message = fmt.Sprintf("shipment %s reconciled: archived_status set to %q", shipmentID, string(ShipmentShipped))
	return result, nil
}

// shipmentReconcileBuildArchiveContent re-reads the shipment's current raw
// Markdown bytes (frontmatter + body) and returns the full replacement
// content with the reconciliation frontmatter fields applied: archived_status
// is set to shipped, custom_fields carries the idempotency key, and the
// top-level resume markers (request-identity digest, canonical prepared
// event bytes, event digest) are stamped so a crash between this write and
// the following durable append can resume via the 2b path instead of
// re-running Phase C. It preserves every other existing frontmatter field
// and the body verbatim.
// shipmentReconcileBuildArchiveContent parses the shipment's CURRENT raw
// Markdown bytes (frontmatter + body) — passed in by the caller from the
// same securely-snapshotted read (snapshotShipmentReconcile's
// snapshot.FileBytes), never re-read here by a fresh pathname operation —
// and returns the full replacement content with the reconciliation
// frontmatter fields applied: archived_status is set to shipped,
// custom_fields carries the idempotency key, and the top-level resume
// markers (request-identity digest, canonical prepared event bytes, event
// digest) are stamped so a crash between this write and the following
// durable append can resume via the 2b path instead of re-running Phase C.
// It preserves every other existing frontmatter field and the body
// verbatim.
//
// rawContent MUST come from the same no-follow, handle-relative snapshot
// read already taken under the held Phase D locks (snapshot.FileBytes) —
// re-deriving and re-reading the archive path here via FindArtifactPath/
// os.ReadFile would reintroduce exactly the pathname TOCTOU window the
// snapshot primitive was hardened to close: an attacker or concurrent
// filesystem actor could replace the archive entry between the snapshot
// read and this second read, and the new content would be built from
// bytes that were never actually verified.
func shipmentReconcileBuildArchiveContent(shipmentID string, rawContent []byte, idempotencyKey, requestIdentityDigest string, evidence shipmentReconcileEvidenceResult) ([]byte, error) {
	if len(rawContent) == 0 {
		return nil, fmt.Errorf("build shipment %s archive content: snapshot bytes are empty; the archive file must exist for a reconcile-to-shipped repair", shipmentID)
	}
	frontmatter, body, err := models.ParseFrontmatter(string(rawContent))
	if err != nil {
		return nil, fmt.Errorf("parse shipment %s archive frontmatter: %w", shipmentID, err)
	}
	if frontmatter == nil {
		frontmatter = map[string]any{}
	}

	customFields, _ := frontmatter["custom_fields"].(map[string]any)
	if customFields == nil {
		customFields = map[string]any{}
	}
	customFields[shipmentReconcileIdempotencyKeyField] = idempotencyKey

	frontmatter["archived_status"] = string(ShipmentShipped)
	frontmatter["custom_fields"] = customFields
	frontmatter[shipmentReconcileRequestIdentityDigestField] = requestIdentityDigest
	frontmatter[shipmentReconcilePreparedEventField] = string(evidence.EventBytes)
	frontmatter[shipmentReconcileEventDigestField] = evidence.EventDigest

	return []byte(models.SerializeFrontmatter(frontmatter, body)), nil
}

// syncShipmentReconcileItemIndex keeps the SQLite items row for shipmentID
// in sync with the just-written archiveContent (Copilot PR #440 review,
// finding 5): without this, the SQLite index/cache is left stale relative
// to the Markdown source of truth (archived_status: shipped, plus the
// resume-marker custom_fields) immediately after a successful
// reconciliation, until the next sync/reindex — a real
// index-vs-source-of-truth drift window this governed transaction should
// close opportunistically, the same way ArchiveItem
// (internal/core/archive.go) re-syncs the items row right after its own
// frontmatter write.
//
// Best-effort, matching appendShipmentReconcileEventImpl's own documented
// "index failure does not un-reconcile" precedent: the Markdown archive
// file (already durably written by the time this is called) remains the
// authoritative source of truth regardless of whether this sync succeeds;
// a failure is logged as a warning and never rolls back or fails the
// caller's transaction. The next sync/reindex repairs any drift.
func syncShipmentReconcileItemIndex(ctx context.Context, ws *Workspace, shipmentID string, archiveContent []byte) {
	frontmatter, body, err := models.ParseFrontmatter(string(archiveContent))
	if err != nil {
		slog.WarnContext(ctx, "reconcile shipment to shipped: parse archive content for index sync failed; index left stale, repaired by next sync/reindex", "shipment_id", shipmentID, "error", err)
		return
	}
	artifact, err := models.ArtifactFromFrontmatter(frontmatter, body)
	if err != nil {
		slog.WarnContext(ctx, "reconcile shipment to shipped: build artifact for index sync failed; index left stale, repaired by next sync/reindex", "shipment_id", shipmentID, "error", err)
		return
	}
	if artifact.ID == "" {
		artifact.ID = shipmentID
	}
	if ws == nil || ws.DB == nil {
		return
	}
	if err := bldb.UpsertItem(ctx, ws.DB, artifact); err != nil {
		slog.WarnContext(ctx, "reconcile shipment to shipped: sync items row failed; JSONL/Markdown remains source of truth, index repaired by next sync/reindex", "shipment_id", shipmentID, "error", err)
	}
}

// lockShipmentReconcileCThenB acquires the item-log lock (C) for itemLogID,
// then the sorted artifact-mutation batch (B) for artifactIDs, in that
// order — matching AssociateCommit's own C-then-B acquisition order so the
// two writers can never form a reversed-order cycle. The returned release
// function releases B first, then C, and is safe to call at most once (the
// caller owns idempotent-release bookkeeping, matching every other lock
// helper in this file family).
func lockShipmentReconcileCThenB(ctx context.Context, ws *Workspace, itemLogID string, artifactIDs []string) (context.Context, func() error, error) {
	ctxC, unlockC, err := lockShipmentReconcileItemLog(ctx, ws, itemLogID)
	if err != nil {
		return ctx, nil, err
	}
	ctxB, unlockB, err := lockArtifactMutations(ctxC, ws, artifactIDs)
	if err != nil {
		if cerr := unlockC(); cerr != nil {
			slog.WarnContext(ctx, "reconcile shipment to shipped: release item-log lock after failed artifact-lock acquisition", "item_id", itemLogID, "error", cerr)
		}
		return ctx, nil, err
	}
	release := func() error {
		errB := unlockB()
		errC := unlockC()
		return errors.Join(errB, errC)
	}
	return ctxB, release, nil
}

// shipmentReconcileReadItemLog reads itemID's raw item-log bytes, tolerating
// This read is AUTHORITATIVE input to the reconciliation classifier during
// Phase D's final commit (PR #440 review finding 7, hardened further in
// review round 6): a symlinked item-log path must be rejected rather than
// silently followed, or a forged replay event planted outside the
// workspace could be read as if it were this shipment's own durable log.
// It reuses the SAME no-follow, directory-handle-relative single-read
// primitive already hardened for the archive file
// (readShipmentReconcileArchiveSnapshotFile, shipment_reconcile_snapshot.go
// / _unix.go / _windows.go / _other.go) rather than a separate
// EvalSymlinks-then-os.ReadFile pathname pair, which — despite validating
// containment first — still reopens the path a second time and so remains
// vulnerable to the log being replaced by a symlink between the check and
// the read.
func shipmentReconcileReadItemLog(ws *Workspace, itemID string) ([]byte, error) {
	logsDir := WorkspaceLogsRoot(ws.RootPath)
	fileName := itemID + ".jsonl"

	logBytes, err := readShipmentReconcileArchiveSnapshotFile(logsDir, fileName)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// Not-yet-created log: nothing to reject, matches the
			// established nil/nil convention.
			return nil, nil
		}
		return nil, fmt.Errorf("read item log %s: %w", fileName, err)
	}
	return logBytes, nil
}

// shipmentReconcileManifestDigest returns a domain-separated digest over the
// ordered manifest member ID list, recorded on the reconciliation event's
// evidence sub-object as an audit-time membership fingerprint. It is
// intentionally independent of computeManifestDigest (shipment_gate_manifest.go):
// that function additionally requires a resolvable covering feature and a
// shipment-head SHA, neither of which the legacy-repair reconciliation domain
// this task covers can assume exists.
func shipmentReconcileManifestDigest(memberIDs []string) string {
	return shipmentReconcileDigestHex(
		shipmentReconcileManifestDigestDomain,
		shipmentReconcileListField("member_ids", memberIDs),
	)
}

// shipmentReconcileValidateRawManifestItems validates the shipment's RAW
// `items` frontmatter value (shipment.CustomFields["items"], the value
// NormalizeShipmentItems itself reads) is a well-formed array of unique,
// non-blank strings, returning the validated member IDs in their original
// order (167.008-T, PR #440 review round 2, finding 4).
//
// NormalizeShipmentItems (shipment.go) followed by uniqueNonEmptyStrings
// silently DROPS any non-string element, any blank/whitespace-only string,
// and any duplicate entry from the raw manifest before this reconcile
// transaction ever validates member terminal state. A malformed/corrupt
// legacy manifest — e.g. ["done-member", 42, "", "done-member"] — would
// otherwise be silently reduced to ["done-member"] and reconciled after
// checking only that single surviving valid string, even though the RAW
// manifest was ambiguous/partially corrupt. The reconcile evidence contract
// requires such evidence to fail closed rather than be silently repaired,
// so this validates the raw value BEFORE any normalization/dropping runs.
// It is used everywhere this transaction computes or re-computes the
// governed member-ID set: Phase A's frozen set (reconcileShipmentToShippedImpl),
// Phase D's re-read (reconcileShipmentPhaseCAndD), and the standalone
// precondition gate (validateShipmentReconcilePreconditions) — not just the
// initial Phase A read — so a malformed raw manifest cannot slip through any
// one of those re-validation points either.
func shipmentReconcileValidateRawManifestItems(shipment *models.Artifact) ([]string, error) {
	if shipment == nil || shipment.CustomFields == nil {
		return []string{}, nil
	}
	raw, ok := shipment.CustomFields["items"]
	if !ok || raw == nil {
		return []string{}, nil
	}

	var elements []any
	switch items := raw.(type) {
	case []string:
		elements = make([]any, len(items))
		for i, v := range items {
			elements[i] = v
		}
	case []any:
		elements = items
	default:
		return nil, fmt.Errorf("reconcile shipment to shipped: shipment %s manifest items must be an array, got %T: %w", shipment.ID, raw, blerrors.ErrValidation)
	}

	seen := make(map[string]struct{}, len(elements))
	result := make([]string, 0, len(elements))
	for i, element := range elements {
		value, isString := element.(string)
		if !isString {
			return nil, fmt.Errorf("reconcile shipment to shipped: shipment %s manifest items[%d] is not a string (got %T): raw manifest evidence must fail closed rather than be silently repaired: %w", shipment.ID, i, element, blerrors.ErrValidation)
		}
		if strings.TrimSpace(value) == "" {
			return nil, fmt.Errorf("reconcile shipment to shipped: shipment %s manifest items[%d] is blank/whitespace-only: raw manifest evidence must fail closed rather than be silently repaired: %w", shipment.ID, i, blerrors.ErrValidation)
		}
		if _, dup := seen[value]; dup {
			return nil, fmt.Errorf("reconcile shipment to shipped: shipment %s manifest contains duplicate item %q: raw manifest evidence must fail closed rather than be silently repaired: %w", shipment.ID, value, blerrors.ErrValidation)
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result, nil
}

// shipmentReconcileMemberSetsEqual reports whether a and b contain the same
// set of member IDs, ignoring order and any duplicate entries — used by
// Phase D to detect a manifest membership change since the frozen Phase-A
// snapshot (a mismatch is refused as indeterminate, never silently adopted).
func shipmentReconcileMemberSetsEqual(a, b []string) bool {
	setA := make(map[string]struct{}, len(a))
	for _, v := range a {
		setA[v] = struct{}{}
	}
	setB := make(map[string]struct{}, len(b))
	for _, v := range b {
		setB[v] = struct{}{}
	}
	if len(setA) != len(setB) {
		return false
	}
	for v := range setA {
		if _, ok := setB[v]; !ok {
			return false
		}
	}
	return true
}
