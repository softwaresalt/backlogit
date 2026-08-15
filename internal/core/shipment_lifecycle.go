package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	bldb "github.com/softwaresalt/backlogit/internal/db"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/hooks"
	"github.com/softwaresalt/backlogit/internal/models"
)

var deliberationIDPattern = regexp.MustCompile(`\b(?:DL\d+|[0-9]+(?:\.[0-9]+)*-DL)\b`)

// CommitMetadata captures the merge or release commit that closed a shipment.
type CommitMetadata struct {
	SHA     string `json:"sha,omitempty"`
	Message string `json:"message,omitempty"`
	Author  string `json:"author,omitempty"`
}

// ShipShipmentResult summarizes shipment release hygiene actions.
type ShipShipmentResult struct {
	ShipmentID     string   `json:"shipment_id"`
	ShipmentStatus string   `json:"shipment_status"`
	ArchivedIDs    []string `json:"archived_ids"`
	ReturnedIDs    []string `json:"returned_ids"`
	CommitSHA      string   `json:"commit_sha,omitempty"`
}

// ClaimShipment moves a queued shipment to active and marks the included work
// scope active. Activation is all-or-nothing: if any item fails to load or
// activate mid-flight, the shipment and every already-activated item (plus any
// cascade-activated parent) are restored to their pre-claim state so no
// partial/torn activation is left behind.
func ClaimShipment(ctx context.Context, ws *Workspace, shipmentID string) (*models.Artifact, error) {
	// Snapshot the pre-claim shipment before any mutation so a mid-flight
	// failure can be rolled back to a fully queued state.
	current, err := GetShipment(ctx, ws, shipmentID)
	if err != nil {
		return nil, err
	}
	preClaimShipment := cloneArtifact(current)

	if err := MoveShipmentStatus(ctx, ws, shipmentID, ShipmentActive); err != nil {
		return nil, err
	}

	shipment, err := GetShipment(ctx, ws, shipmentID)
	if err != nil {
		// The shipment is active but no items have been activated yet; restore
		// it to queued so a failed read-back does not leave a torn state.
		return nil, rollbackShipmentClaim(ctx, ws, shipmentID, preClaimShipment, nil,
			fmt.Errorf("reload shipment after activation: %w", err))
	}

	// activatedIDs records every item this claim transitioned to active, in
	// application order, so rollback can revert them (newest first) on failure.
	var activatedIDs []string
	for _, itemID := range NormalizeShipmentItems(shipment) {
		item, loadErr := loadArtifact(ctx, ws, itemID)
		if loadErr != nil {
			return nil, rollbackShipmentClaim(ctx, ws, shipmentID, preClaimShipment, activatedIDs,
				fmt.Errorf("load item %s: %w", itemID, loadErr))
		}
		if item.Status == models.StatusQueued {
			// Record the item as activation-attempted *before* mutating it:
			// setArtifactStatus persists the item active before cascading parent
			// statuses, so a failure mid-call can leave the item active on disk.
			// Tracking it up front guarantees rollback reverts it; a queued->queued
			// revert is a safe no-op when activation never landed.
			activatedIDs = append(activatedIDs, itemID)
			if _, setErr := setArtifactStatus(ctx, ws, itemID, models.StatusActive, "shipment claimed"); setErr != nil {
				return nil, rollbackShipmentClaim(ctx, ws, shipmentID, preClaimShipment, activatedIDs,
					fmt.Errorf("activate item %s: %w", itemID, setErr))
			}
		}
	}

	// The shipment artifact itself is not mutated by item activation (its
	// manifest items are children of the feature, not of the shipment), so the
	// snapshot loaded above already reflects the post-activation truth. Return
	// it directly rather than performing another read-back: a read-back here
	// could fail after every item is active, leaving a torn state with no
	// remaining operation to roll back. Eliminating it keeps the claim
	// all-or-nothing by construction.
	return shipment, nil
}

// rollbackShipmentClaim reverts a partially applied shipment claim. Each item
// the claim activated is returned to queued in reverse order so child statuses
// settle before their parents are recomputed by the cascade, and the shipment
// is restored to its pre-claim snapshot. The original claim error is wrapped
// together with any rollback error so the caller sees the full failure context.
func rollbackShipmentClaim(ctx context.Context, ws *Workspace, shipmentID string, preClaimShipment *models.Artifact, activatedIDs []string, claimErr error) error {
	// Guard against a nil triggering error: rollback must never collapse to a
	// nil return that silently drops the failure (a future caller that passes
	// nil would otherwise hide a torn-state rollback behind a success).
	if claimErr == nil {
		claimErr = fmt.Errorf("claim rollback invoked without a triggering error")
	}
	var rollbackErrs []error
	for i := len(activatedIDs) - 1; i >= 0; i-- {
		if _, err := setArtifactStatus(ctx, ws, activatedIDs[i], models.StatusQueued, "shipment claim rolled back"); err != nil {
			rollbackErrs = append(rollbackErrs, fmt.Errorf("revert item %s: %w", activatedIDs[i], err))
		}
	}
	if preClaimShipment != nil {
		if err := persistArtifact(ctx, ws, preClaimShipment, true); err != nil {
			rollbackErrs = append(rollbackErrs, fmt.Errorf("restore shipment %s: %w", shipmentID, err))
		}
	}

	slog.WarnContext(ctx, "shipment claim rolled back",
		"shipment_id", shipmentID, "reverted_items", len(activatedIDs), "error", claimErr)
	appendItemEvent(ctx, ws, shipmentID, "shipment_claim_rolled_back", map[string]any{
		"reverted_items": len(activatedIDs),
	})

	if len(rollbackErrs) > 0 {
		return fmt.Errorf("claim shipment %s: %w; rollback failed: %w", shipmentID, claimErr, errors.Join(rollbackErrs...))
	}
	return fmt.Errorf("claim shipment %s: %w", shipmentID, claimErr)
}

type shipArtifactSnapshot struct {
	artifact *models.Artifact
	file     fileSnapshot
	eventLog fileSnapshot
}

func snapshotShipArtifacts(ctx context.Context, ws *Workspace, ids []string) (map[string]shipArtifactSnapshot, error) {
	snapshots := make(map[string]shipArtifactSnapshot)
	for _, id := range uniqueNonEmptyStrings(ids) {
		artifact, err := findArtifact(ctx, ws, id)
		if err != nil {
			return nil, fmt.Errorf("snapshot artifact %s: %w", id, err)
		}
		path, err := FindArtifactPath(ctx, ws, id)
		if err != nil {
			return nil, fmt.Errorf("snapshot artifact %s path: %w", id, err)
		}
		file, err := snapshotFile(path)
		if err != nil {
			return nil, fmt.Errorf("snapshot artifact %s file: %w", id, err)
		}
		_, unlockItemLog, lockErr := events.LockItemLogCrossProcess(ctx, WorkspaceLogsRoot(ws.RootPath), id)
		if lockErr != nil {
			return nil, fmt.Errorf("lock artifact %s event log: %w", id, lockErr)
		}
		eventLog, err := snapshotFile(events.LogPathForItem(WorkspaceLogsRoot(ws.RootPath), id))
		unlockItemLog()
		if err != nil {
			return nil, fmt.Errorf("snapshot artifact %s event log: %w", id, err)
		}
		snapshots[id] = shipArtifactSnapshot{
			artifact: cloneArtifact(artifact),
			file:     file,
			eventLog: eventLog,
		}
	}
	return snapshots, nil
}

func restoreShipArtifacts(ctx context.Context, ws *Workspace, snapshots map[string]shipArtifactSnapshot) error {
	ctx = context.WithoutCancel(ctx)
	logsDir := WorkspaceLogsRoot(ws.RootPath)
	operationID := shipmentOperationID(ctx)
	ids := make([]string, 0, len(snapshots))
	for id := range snapshots {
		ids = append(ids, id)
	}
	var errs []error
	for _, id := range depthSortedIDs(ids) {
		itemCtx, unlockItemLog, lockErr := events.LockItemLogCrossProcess(ctx, logsDir, id)
		if lockErr != nil {
			errs = append(errs, fmt.Errorf("lock artifact %s event log: %w", id, lockErr))
			continue
		}
		snapshot := snapshots[id]
		currentEvents, readErr := events.ReadAllEvents(itemCtx, logsDir, id)
		var preservedEvents []events.Event
		if readErr != nil {
			errs = append(errs, fmt.Errorf("read mutated artifact %s event log: %w", id, readErr))
		} else if preservedEvents, readErr = eventsSinceSnapshot(snapshot.eventLog, id, currentEvents, operationID); readErr != nil {
			errs = append(errs, fmt.Errorf("identify concurrent events for %s: %w", id, readErr))
		}
		if currentPath, err := FindArtifactPath(ctx, ws, id); err == nil && currentPath != snapshot.file.Path {
			if removeErr := os.Remove(currentPath); removeErr != nil && !os.IsNotExist(removeErr) {
				errs = append(errs, fmt.Errorf("remove mutated artifact %s: %w", id, removeErr))
			}
		} else if err != nil && !errors.Is(err, blerrors.ErrNotFound) {
			errs = append(errs, fmt.Errorf("locate mutated artifact %s: %w", id, err))
		}
		if err := restoreSnapshot(snapshot.file); err != nil {
			errs = append(errs, fmt.Errorf("restore artifact %s file: %w", id, err))
		}
		if err := restoreSnapshot(snapshot.eventLog); err != nil {
			errs = append(errs, fmt.Errorf("restore artifact %s event log: %w", id, err))
		}
		writer := NewWorkspaceEventWriter(ws, logsDir)
		for _, event := range preservedEvents {
			if err := writer.AppendEvent(itemCtx, event); err != nil {
				errs = append(errs, fmt.Errorf("restore artifact %s concurrent event: %w", id, err))
			}
		}
		if err := bldb.ReindexItemLog(itemCtx, ws.DB, logsDir, id); err != nil {
			errs = append(errs, fmt.Errorf("restore artifact %s event index: %w", id, err))
		}
		if err := bldb.UpsertItem(itemCtx, ws.DB, snapshot.artifact); err != nil {
			errs = append(errs, fmt.Errorf("restore artifact %s index: %w", id, err))
		}
		unlockItemLog()
	}
	return errors.Join(errs...)
}

// eventsSinceSnapshot returns events appended after the ship snapshot that are
// not part of this ship operation. Ship-generated events carry an operation
// marker in their delta, allowing unrelated concurrent audit events to survive
// rollback. Gate-blocked evidence is always retained because it explains why
// the shipment could not complete even when its writer used a separate path.
func guardEventsSinceSnapshot(snapshot fileSnapshot, itemID string, current []events.Event) ([]events.Event, error) {
	preserved, err := eventsSinceSnapshot(snapshot, itemID, current, "")
	if err != nil {
		return nil, err
	}
	guards := make([]events.Event, 0, len(preserved))
	for _, event := range preserved {
		if event.EventType == EventGateBlocked {
			guards = append(guards, event)
		}
	}
	return guards, nil
}

func eventsSinceSnapshot(snapshot fileSnapshot, itemID string, current []events.Event, operationID string) ([]events.Event, error) {
	baseline := make(map[string]int)
	for _, line := range strings.Split(string(snapshot.Content), "\n") {
		event, ok, err := events.ParseEventLine(line, itemID)
		if err != nil || !ok {
			continue
		}
		key, err := json.Marshal(event)
		if err != nil {
			return nil, fmt.Errorf("marshal baseline event: %w", err)
		}
		baseline[string(key)]++
	}

	var extras []events.Event
	for _, event := range current {
		key, err := json.Marshal(event)
		if err != nil {
			return nil, fmt.Errorf("marshal current event: %w", err)
		}
		keyString := string(key)
		if baseline[keyString] > 0 {
			baseline[keyString]--
			continue
		}
		if event.EventType == EventGateBlocked || !isShipmentOperationEvent(event, operationID) {
			extras = append(extras, event)
		}
	}
	return extras, nil
}

// ShipShipment closes a shipped scope, returns untouched descendants to backlog,
// archives the released artifacts, and records the closing commit in item logs.
func ShipShipment(ctx context.Context, ws *Workspace, shipmentID string, commit *CommitMetadata) (result *ShipShipmentResult, err error) {
	shipment, err := GetShipment(ctx, ws, shipmentID)
	if err != nil {
		return nil, err
	}
	if shipment.Status != models.StatusActive {
		return nil, fmt.Errorf("ship shipment %s: %w", shipmentID, blerrors.ErrShipmentConflict)
	}
	ctx = withShipmentOperation(ctx, shipmentID)

	// Fire pre-ship hooks (top-level).
	if ws.HookRunner != nil {
		hookCtx := hooks.HookContext{
			ItemID:       shipmentID,
			ArtifactType: "shipment",
			OldValues:    map[string]any{"status": string(shipment.Status)},
			NewValues:    map[string]any{"status": string(ShipmentShipped)},
			Actor:        "backlogit",
			Workspace:    ws.RootPath,
			TopLevel:     true,
		}
		if err := ws.HookRunner.FirePre(ctx, hooks.HookShipShipment, hookCtx); err != nil {
			return nil, fmt.Errorf("pre-ship hook: %w", err)
		}
	}

	// Membership lock (106-F F1 review finding, hardened across two review
	// passes): held from the release-scope snapshot all the way through the
	// shipment's OWN status transition OUT of "active" — the only status
	// during which AddItemToShipment/ReturnBlockedItem are permitted to
	// mutate membership at all (shipmentMutationBlocked refuses on any other
	// status). Once moveShipmentStatusWithTopLevel below completes, further
	// membership mutation is independently blocked by that status check, so
	// the lock does not need to be held any longer than this.
	//
	// An earlier version of this lock released as soon as the manifest proof
	// was signed inside gateShipmentCompletion. That left an UNPROTECTED
	// window between signing and this function's own status transition,
	// during which a concurrent AddItemToShipment could still acquire the
	// (by-then-released) lock, add an unvalidated member, and return
	// successfully — reopening the manifest TOCTOU immediately after signing
	// rather than before it (106-F F1 review finding, second pass). Every
	// step that reads or acts on shipment membership (feature-scope
	// resolution, the non-member-feature snapshot, release-scope
	// completion, and the shipment's own status write) now runs inside this
	// SAME locked closure so no such window remains.
	var explicitScope, releaseScope, featureIDs, returnedIDs []string
	var explicitScopeSet map[string]struct{}
	var nonMemberFeatureSnapshots map[string]featureStatusSnapshot
	var shipSnapshots map[string]shipArtifactSnapshot
	shipRollbackAttempted := false
	// archivedIDs and restored are declared here (rather than at their first
	// assignment) so both the locked closure below and the deferred fallback
	// registered immediately after it observe/mutate the same variables.
	// featureScopeRoots only discovers a non-member feature by walking UP
	// from an explicitly listed descendant, so a feature nested UNDER an
	// explicit-member root (reachable via AdoptItem re-parenting, e.g. a
	// dotted "002.001-F") is captured as "non-member" even though
	// collectArchiveCandidateIDs later sweeps it into archivedIDs anyway, as
	// a genuine descendant of that explicit-member root. Restoring such a
	// feature would revert an archival this same call just legitimately
	// performed (review-fix, 133.004-T).
	var archivedIDs []string
	// restored tracks whether the explicit, in-line restore call later in
	// this function (on the successful path) already ran, so the deferred
	// fallback becomes a no-op instead of invoking
	// restoreRolledUpNonMemberFeatures twice.
	restored := false
	// 133.004-T: always attempt the revert, even if a later step in this
	// function fails and returns early -- a partial/aborted ship must not
	// leave a non-member covering feature stranded mid-rollup. A restore
	// failure is joined onto (never silently drops) the function's error.
	// review-fix (PR #327): this defer is now a fallback for early-return
	// paths only. On the successful path, the explicit call further below
	// runs the restore BEFORE VerifyPostShipConsistency and the post-ship
	// hooks, so consistency checks and external integrations never observe
	// the covering feature in its transient, incorrectly-rolled-up
	// done/archived state. Relying solely on this defer would let it fire
	// only during return unwinding -- strictly after those in-line
	// statements already ran to completion.
	defer func() {
		if restored || shipRollbackAttempted {
			return
		}
		if restoreErr := restoreRolledUpNonMemberFeatures(ctx, ws, nonMemberFeatureSnapshots, archivedIDs); restoreErr != nil {
			err = errors.Join(err, fmt.Errorf("ship shipment %s: restore non-member covering feature scope: %w", shipmentID, restoreErr))
		}
	}()

	var releaseArtifactLocks func() error
	defer func() {
		if releaseArtifactLocks != nil {
			_ = releaseArtifactLocks()
		}
	}()

	lockErr := func() (closureErr error) {
		unlock, lockErr := lockShipmentMembership(ctx, ws, shipmentID)
		if lockErr != nil {
			return fmt.Errorf("acquire membership lock: %w", lockErr)
		}
		defer func() { _ = unlock() }()
		// Roll back before releasing the membership lock. Otherwise a concurrent
		// membership mutation could land after the failed release and be
		// overwritten by restoration of the pre-ship snapshot.
		defer func() {
			if closureErr == nil || len(shipSnapshots) == 0 {
				return
			}
			shipRollbackAttempted = true
			if rollbackErr := restoreShipArtifacts(ctx, ws, shipSnapshots); rollbackErr != nil {
				closureErr = fmt.Errorf("%w; rollback failed: %w", closureErr, rollbackErr)
			}
		}()

		explicitScope = uniqueNonEmptyStrings(NormalizeShipmentItems(shipment))
		explicitScopeSet = toIDSet(explicitScope)
		var scopeErr error
		releaseScope, scopeErr = releaseScopeItemIDs(ctx, ws, explicitScope)
		if scopeErr != nil {
			return fmt.Errorf("resolve release scope: %w", scopeErr)
		}

		// Two-level shipment gate (082-F ST4.2): validate member-task gate
		// evidence and run a shipment-level autoharness gate check over the
		// full diff BEFORE completing the release scope, so an ungated
		// member is never auto-completed. A refusal leaves shipment state
		// unchanged. explicitScope (captured above, before any of this
		// runs, and now additionally protected by the membership lock held
		// for this entire closure) is re-checked against a fresh reload
		// immediately before the manifest-binding proof is signed, so a
		// concurrent membership mutation landing after this snapshot cannot
		// ride inside a signed proof whose members were never actually
		// validated (106-F F1 review finding F3).
		//
		// gatedHead is the HEAD gateShipmentCompletion's own pre/post
		// headDriftError bracket validated as stable -- "" whenever that
		// bracket did not run or is legacy-inert (no broker, gate not
		// enforced, genuine no-repo). It is carried through every remaining
		// in-process step below (release-scope completion, feature
		// return/status cascades) and re-checked ONE MORE TIME immediately
		// before this closure's own status-transition persist, narrowing
		// the residual window a concurrent commit could otherwise land in
		// between this call returning and that persist (106.033-T).
		gatedHead, err := gateShipmentCompletion(ctx, ws, shipmentID, releaseScope, explicitScope)
		if err != nil {
			return err
		}

		// 133.004-T: resolve covering-feature ancestry BEFORE completing the
		// release scope. completeReleaseScope's status writes trigger the
		// generic cascadePersistedParentStatuses rollup (harness_status.go),
		// which marks a parent done -- and, per the registry's status-based
		// directory routing, relocates it into the archive directory --
		// purely because its currently recorded children happen to all be
		// terminal. That rollup does not know about shipment membership, so
		// it can fire for a covering feature that was never itself listed
		// as an explicit shipment member. Snapshotting here captures the
		// pre-ship status of every non-member feature so the rollup can be
		// detected and reverted once the ship completes (133-F).
		var featureErr error
		featureIDs, featureErr = featureScopeRoots(ctx, ws, explicitScope)
		if featureErr != nil {
			return fmt.Errorf("resolve feature scope: %w", featureErr)
		}
		var snapshotErr error
		nonMemberFeatureSnapshots, snapshotErr = snapshotNonMemberFeatureStatuses(ctx, ws, featureIDs, explicitScopeSet)
		if snapshotErr != nil {
			return fmt.Errorf("snapshot covering feature scope: %w", snapshotErr)
		}

		rollbackIDs := append([]string{shipmentID}, releaseScope...)
		rollbackIDs = append(rollbackIDs, featureIDs...)
		for _, featureID := range featureIDs {
			descendants, descendantsErr := descendantItems(ctx, ws, featureID)
			if descendantsErr != nil {
				return fmt.Errorf("snapshot feature descendants for %s: %w", featureID, descendantsErr)
			}
			for _, descendant := range descendants {
				rollbackIDs = append(rollbackIDs, descendant.ID)
			}
		}
		var artifactLockErr error
		ctx, releaseArtifactLocks, artifactLockErr = lockArtifactMutations(ctx, ws, rollbackIDs)
		if artifactLockErr != nil {
			return fmt.Errorf("lock release scope artifacts: %w", artifactLockErr)
		}
		// Re-run the completion gate after acquiring the artifact locks so no
		// concurrent artifact mutation can land between validation and snapshot.
		if gatedHead, err = gateShipmentCompletion(ctx, ws, shipmentID, releaseScope, explicitScope); err != nil {
			return err
		}
		shipSnapshots, snapshotErr = snapshotShipArtifacts(ctx, ws, rollbackIDs)
		if snapshotErr != nil {
			return fmt.Errorf("snapshot release scope: %w", snapshotErr)
		}

		if err := completeReleaseScope(ctx, ws, releaseScope); err != nil {
			return fmt.Errorf("complete release scope: %w", err)
		}

		returnedIDs = make([]string, 0)
		releaseScopeSet := toIDSet(releaseScope)
		for _, featureID := range featureIDs {
			returned, returnErr := returnUnreleasedFeatureItems(ctx, ws, featureID, releaseScopeSet)
			if returnErr != nil {
				return fmt.Errorf("return unreleased feature items for %s: %w", featureID, returnErr)
			}
			returnedIDs = append(returnedIDs, returned...)
			// 133.004-T: only a covering feature that is itself an explicit
			// shipment member is marked done here. A feature that is merely
			// an ancestor of some shipped item, but was never itself a
			// member, must be left alone -- its lifecycle is independent of
			// this partial release, and any unintended rollup it already
			// picked up from completeReleaseScope's cascade is reverted by
			// the deferred restore above (133-F).
			if _, isMember := explicitScopeSet[featureID]; isMember {
				if _, setErr := setArtifactStatus(ctx, ws, featureID, models.StatusDone, "feature released"); setErr != nil {
					return fmt.Errorf("mark feature %s done: %w", featureID, setErr)
				}
			}
		}

		return moveShipmentStatusWithHeadGuard(ctx, ws, shipmentID, ShipmentShipped, false, gatedHead)
	}()
	if lockErr != nil {
		return nil, fmt.Errorf("ship shipment %s: %w", shipmentID, lockErr)
	}

	archiveIDs, err := collectArchiveCandidateIDs(ctx, ws, shipmentID, releaseScope, featureIDs, returnedIDs, explicitScopeSet)
	if err != nil {
		return nil, fmt.Errorf("ship shipment %s: collect archive scope: %w", shipmentID, err)
	}

	if err := attachCommitToItems(ctx, ws, archiveIDs, commit); err != nil {
		return nil, fmt.Errorf("ship shipment %s: record commit traceability: %w", shipmentID, err)
	}

	archivedIDs, err = archiveItems(ctx, ws, archiveIDs)
	if err != nil {
		return nil, fmt.Errorf("ship shipment %s: archive release scope: %w", shipmentID, err)
	}

	// review-fix (PR #327): revert any unintended non-member covering-feature
	// rollup NOW, on the successful path, before VerifyPostShipConsistency and
	// the post-ship hooks run. Leaving this to the deferred fallback alone
	// would let consistency verification and any post-ship hook (webhook,
	// custom integration) observe the feature in its transient,
	// incorrectly-rolled-up done/archived state, even though it is reverted
	// moments later when the function returns.
	if restoreErr := restoreRolledUpNonMemberFeatures(ctx, ws, nonMemberFeatureSnapshots, archivedIDs); restoreErr != nil {
		return nil, fmt.Errorf("ship shipment %s: restore non-member covering feature scope: %w", shipmentID, restoreErr)
	}
	restored = true

	if err := VerifyPostShipConsistency(ctx, ws, archivedIDs); err != nil {
		return nil, fmt.Errorf("ship shipment %s: post-ship consistency: %w", shipmentID, err)
	}

	// Fire post-ship hooks (top-level).
	if ws.HookRunner != nil {
		hookCtx := hooks.HookContext{
			ItemID:       shipmentID,
			ArtifactType: "shipment",
			OldValues:    map[string]any{"status": string(shipment.Status)},
			NewValues:    map[string]any{"status": string(ShipmentShipped)},
			Actor:        "backlogit",
			Workspace:    ws.RootPath,
			TopLevel:     true,
		}
		ws.HookRunner.FirePost(ctx, hooks.HookShipShipment, hookCtx)
	}

	return &ShipShipmentResult{
		ShipmentID:     shipmentID,
		ShipmentStatus: string(ShipmentShipped),
		ArchivedIDs:    archivedIDs,
		ReturnedIDs:    uniqueNonEmptyStrings(returnedIDs),
		CommitSHA:      commitSHA(commit),
	}, nil
}

func completeReleaseScope(ctx context.Context, ws *Workspace, releaseScope []string) error {
	for _, itemID := range depthSortedIDs(releaseScope) {
		item, err := loadArtifact(ctx, ws, itemID)
		if err != nil {
			return fmt.Errorf("load item %s: %w", itemID, err)
		}
		if item.Status == models.StatusBlocked {
			return fmt.Errorf("item %s is blocked and cannot ship: %w", itemID, blerrors.ErrShipmentConflict)
		}
		if isTerminalReleaseStatus(item.Status) {
			continue
		}
		if _, err := setArtifactStatus(ctx, ws, itemID, models.StatusDone, "shipment released"); err != nil {
			return fmt.Errorf("mark item %s done: %w", itemID, err)
		}
	}
	return nil
}

func returnUnreleasedFeatureItems(ctx context.Context, ws *Workspace, featureID string, releaseScope map[string]struct{}) ([]string, error) {
	descendants, err := descendantItems(ctx, ws, featureID)
	if err != nil {
		return nil, err
	}

	var returned []string
	for _, item := range descendants {
		if _, ok := releaseScope[item.ID]; ok {
			continue
		}
		if isTerminalReleaseStatus(item.Status) {
			continue
		}
		if item.Status != models.StatusQueued {
			if _, err := setArtifactStatus(ctx, ws, item.ID, models.StatusQueued, "returned to backlog after release"); err != nil {
				return nil, err
			}
		}
		// Clear parent_id so the orphaned item is visible as unparented backlog.
		// The hierarchical ID prefix preserves provenance without implying ownership.
		if err := clearParentID(ctx, ws, item.ID); err != nil {
			return nil, err
		}
		appendItemEvent(ctx, ws, item.ID, "returned_to_backlog", map[string]any{
			"feature_id": featureID,
		})
		returned = append(returned, item.ID)
	}
	return uniqueNonEmptyStrings(returned), nil
}

func collectArchiveCandidateIDs(ctx context.Context, ws *Workspace, shipmentID string, releaseScope, featureIDs, returnedIDs []string, explicitScope map[string]struct{}) ([]string, error) {
	candidates := []string{shipmentID}
	returnedSet := toIDSet(returnedIDs)

	for _, itemID := range releaseScope {
		item, err := loadArtifact(ctx, ws, itemID)
		if err != nil {
			return nil, err
		}
		if _, returned := returnedSet[itemID]; returned || item.Status == models.StatusArchived {
			continue
		}
		if isTerminalReleaseStatus(item.Status) {
			candidates = append(candidates, item.ID)
		}
	}

	for _, featureID := range featureIDs {
		// 133.004-T: a covering feature (and its descendants/linked
		// deliberations) is only archived here when the feature is itself an
		// explicit shipment member. An ancestor feature that is merely
		// upstream of some shipped item, but was never listed in the
		// manifest, must be left out of the archive scope entirely -- its
		// own lifecycle and any of its other descendants are independent of
		// this partial release (133-F).
		if _, isMember := explicitScope[featureID]; !isMember {
			continue
		}

		feature, err := loadArtifact(ctx, ws, featureID)
		if err != nil {
			return nil, err
		}
		if feature.Status != models.StatusArchived {
			candidates = append(candidates, feature.ID)
		}

		descendants, err := descendantItems(ctx, ws, featureID)
		if err != nil {
			return nil, err
		}
		for _, item := range descendants {
			if _, returned := returnedSet[item.ID]; returned || item.Status == models.StatusArchived {
				continue
			}
			if isTerminalReleaseStatus(item.Status) {
				candidates = append(candidates, item.ID)
			}
		}

		deliberations, err := linkedDeliberationIDs(ctx, ws, feature)
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, deliberations...)
	}

	return uniqueNonEmptyStrings(candidates), nil
}

// featureStatusSnapshot captures a covering feature's pre-ship status so a
// later unintended parent-status rollup (fired by completeReleaseScope's
// generic cascade in cascadePersistedParentStatuses) can be detected and
// reverted. See snapshotNonMemberFeatureStatuses / restoreRolledUpNonMemberFeatures.
type featureStatusSnapshot struct {
	status models.ArtifactStatus
}

// snapshotNonMemberFeatureStatuses records the pre-ship status of every
// covering feature in featureIDs that is NOT itself an explicit member of the
// shipment manifest (explicitScope). completeReleaseScope's generic
// parent-status cascade (cascadePersistedParentStatuses -> ComputeParentStatus)
// can roll such a feature to done -- and, per the registry's status-based
// directory routing, relocate its file into the archive directory -- purely
// because its currently recorded children happen to all be terminal, even
// though the feature was never listed as a shipment member. Recording the
// prior status here lets restoreRolledUpNonMemberFeatures revert that
// unintended side effect once the ship completes (133.004-T / 133-F).
func snapshotNonMemberFeatureStatuses(ctx context.Context, ws *Workspace, featureIDs []string, explicitScope map[string]struct{}) (map[string]featureStatusSnapshot, error) {
	snapshots := make(map[string]featureStatusSnapshot)
	for _, featureID := range featureIDs {
		if _, isMember := explicitScope[featureID]; isMember {
			continue
		}
		item, err := loadArtifact(ctx, ws, featureID)
		if err != nil {
			return nil, fmt.Errorf("snapshot feature %s: %w", featureID, err)
		}
		snapshots[featureID] = featureStatusSnapshot{status: item.Status}
	}
	return snapshots, nil
}

// restoreRolledUpNonMemberFeatures reverts any non-member covering feature
// whose status changed during the ship -- an unintended side effect of
// completeReleaseScope's generic parent-status cascade -- back to its
// pre-ship status (and, via setArtifactStatus's relocate-on-change persist,
// its pre-ship directory). Features whose status is unchanged are left
// untouched. archivedIDs is the confirmed set of item IDs this same
// ShipShipment call genuinely archived via archiveItems/ArchiveItem
// (complete with archived_from/archived_status provenance and a real move
// under .backlogit/archive/). A feature can appear in snapshots (because
// featureScopeRoots only walks UP from explicitly-listed items, so a feature
// nested under an explicit-member root is "non-member" by that narrower
// test) while ALSO being a genuine descendant swept into archivedIDs by
// collectArchiveCandidateIDs's broader descendant-based sweep. Restoring
// such a feature would revert an archival this exact call just legitimately
// performed, corrupting its archive provenance without reversing the
// already-applied stash-archival side effects -- so any featureID present in
// archivedIDs is always skipped here, regardless of its snapshotted status
// (review-fix for 133.004-T). Restoration is best-effort per feature:
// individual failures are joined together so one failure does not mask or
// block reverting the others, and any feature left un-restored after a
// joined error remains detectable by the doctor over-archived-covering-
// feature audit (CheckOverArchivedFeatures, 133.005-T) for a follow-up
// remediation pass.
//
// Restoration order is deepest-first (see depthSortedIDs below), not simple
// map iteration: setArtifactStatus's own cascade recomputes and can silently
// overwrite an ancestor's status whenever a descendant's status changes, so
// every child covering feature must be restored before its parent covering
// feature is (review-fix for 133.004-T; PR #327 Copilot finding).
//
// ShipShipment's deferred cleanup calls this with its own ctx and is
// documented to "always attempt the revert, even if a later step ... fails
// and returns early" -- the case in which ctx is most likely already
// canceled or past its deadline. Detach from the caller's cancellation/
// deadline up front (mirroring rollbackQueueMove, queue.go:365-372) so this
// best-effort cleanup is not itself defeated by the very failure condition
// it exists to clean up after (review-fix, PR #327 Copilot finding).
func restoreRolledUpNonMemberFeatures(ctx context.Context, ws *Workspace, snapshots map[string]featureStatusSnapshot, archivedIDs []string) error {
	ctx = context.WithoutCancel(ctx)
	archived := make(map[string]struct{}, len(archivedIDs))
	for _, id := range archivedIDs {
		archived[id] = struct{}{}
	}
	// setArtifactStatus unconditionally cascades every status change UP to
	// the parent (cascadePersistedParentStatuses), so restoring a parent's
	// snapshot before its child's would let the child's later restore
	// re-cascade and silently overwrite the parent's just-restored value.
	// Iterate deepest-first (children before parents, the same ordering
	// completeReleaseScope already relies on depthSortedIDs for) so each
	// feature's own restore is always the last write to touch it, instead
	// of depending on Go's unspecified map iteration order (review-fix for
	// 133.004-T).
	ids := make([]string, 0, len(snapshots))
	for id := range snapshots {
		ids = append(ids, id)
	}
	var errs []error
	for _, featureID := range depthSortedIDs(ids) {
		snapshot := snapshots[featureID]
		if _, wasArchived := archived[featureID]; wasArchived {
			continue
		}
		current, err := loadArtifact(ctx, ws, featureID)
		if err != nil {
			errs = append(errs, fmt.Errorf("restore feature %s: reload: %w", featureID, err))
			continue
		}
		if current.Status == snapshot.status {
			continue
		}
		if _, err := setArtifactStatus(ctx, ws, featureID, snapshot.status, "reverted unintended rollup from partial-feature ship"); err != nil {
			errs = append(errs, fmt.Errorf("restore feature %s to %s: %w", featureID, snapshot.status, err))
		}
	}
	return errors.Join(errs...)
}

func attachCommitToItems(ctx context.Context, ws *Workspace, itemIDs []string, commit *CommitMetadata) error {
	if commit == nil || strings.TrimSpace(commit.SHA) == "" {
		return nil
	}
	for _, itemID := range uniqueNonEmptyStrings(itemIDs) {
		// Load from Markdown (source of truth): one authoritative read for
		// both the archived-status guard and the mutate-then-persist operation.
		// The DB fast-path carries neither item_links (stored in the separate
		// item_links table) nor archive provenance (unindexed), so using it
		// would drop those fields on re-persist and could give a stale status
		// on the archived-skip guard when the index has not been rehydrated.
		artifact, err := findArtifact(ctx, ws, itemID)
		if err != nil {
			if errors.Is(err, blerrors.ErrNotFound) {
				return fmt.Errorf("reload item %s from markdown: %w", itemID, blerrors.ErrNotFound)
			}
			return fmt.Errorf("reload item %s from markdown: %w", itemID, err)
		}
		// 129.001-T: skip already-archived items — stamping a new shipment
		// commit on a pre-existing archived artifact is semantically wrong
		// (the artifact belonged to an earlier shipment), and the write-
		// boundary guard would refuse the re-persist without provenance anyway.
		if artifact.Status == models.StatusArchived {
			continue
		}
		// 129.002-T: the Markdown-loaded artifact carries item_links and
		// archive provenance; set commit and persist it so those fields survive
		// the rewrite (mirrors the MoveInQueue / serializer_provenance precedent).
		artifact.Commit = commit.SHA
		artifact.UpdatedAt = models.NowUTC()
		if err := persistArtifact(ctx, ws, artifact, false); err != nil {
			return fmt.Errorf("persist item %s commit: %w", itemID, err)
		}
		if err := LinkCommit(ctx, ws.DB, ws, itemID, commit.SHA, commit.Message, commit.Author); err != nil {
			return fmt.Errorf("link commit for %s: %w", itemID, err)
		}
	}
	return nil
}

// archiveItems archives every item in itemIDs, deepest-first, and always
// returns the IDs it successfully archived even when a later item fails
// (review-fix, PR #327 Copilot finding). ShipShipment assigns this return
// value directly to its archivedIDs exclusion set before an error triggers
// its own early return, so its deferred restoreRolledUpNonMemberFeatures
// relies on this partial list to avoid reverting a nested feature this same
// call already legitimately archived moments before a later, unrelated item
// failed. Discarding the accumulated IDs on error (returning nil) would
// reopen that exact corruption via a partial-failure path.
func archiveItems(ctx context.Context, ws *Workspace, itemIDs []string) ([]string, error) {
	ordered := depthSortedIDs(itemIDs)
	archived := make([]string, 0, len(ordered))
	for _, itemID := range ordered {
		item, err := loadArtifact(ctx, ws, itemID)
		if err != nil {
			return archived, fmt.Errorf("load item %s for archive: %w", itemID, err)
		}
		if item.Status == models.StatusArchived {
			continue
		}
		if _, err := ArchiveItem(ctx, ws.DB, ws, itemID, WithTopLevel(false)); err != nil {
			return archived, fmt.Errorf("archive item %s: %w", itemID, err)
		}
		archived = append(archived, itemID)
	}
	return archived, nil
}

func releaseScopeItemIDs(ctx context.Context, ws *Workspace, itemIDs []string) ([]string, error) {
	ids := make([]string, 0, len(itemIDs))
	seen := make(map[string]struct{}, len(itemIDs))
	for _, itemID := range uniqueNonEmptyStrings(itemIDs) {
		if _, ok := seen[itemID]; !ok {
			seen[itemID] = struct{}{}
			ids = append(ids, itemID)
		}
		descendants, err := descendantItems(ctx, ws, itemID)
		if err != nil {
			return nil, err
		}
		for _, child := range descendants {
			if _, ok := seen[child.ID]; ok {
				continue
			}
			seen[child.ID] = struct{}{}
			ids = append(ids, child.ID)
		}
	}
	return ids, nil
}

func featureScopeRoots(ctx context.Context, ws *Workspace, itemIDs []string) ([]string, error) {
	seen := map[string]struct{}{}
	var featureIDs []string
	for _, itemID := range uniqueNonEmptyStrings(itemIDs) {
		currentID := itemID
		for currentID != "" {
			item, err := loadArtifact(ctx, ws, currentID)
			if err != nil {
				return nil, err
			}
			if item.ArtifactType == "feature" {
				if _, ok := seen[item.ID]; !ok {
					seen[item.ID] = struct{}{}
					featureIDs = append(featureIDs, item.ID)
				}
			}
			currentID = item.ParentID
		}
	}
	return featureIDs, nil
}

func linkedDeliberationIDs(ctx context.Context, ws *Workspace, feature *models.Artifact) ([]string, error) {
	if feature == nil {
		return nil, nil
	}
	var ids []string
	if feature.CustomFields != nil {
		if value, ok := feature.CustomFields["source_deliberation_id"].(string); ok && value != "" {
			ids = append(ids, value)
		}
	}
	ids = append(ids, deliberationIDPattern.FindAllString(feature.Description, -1)...)
	for _, ref := range feature.References {
		ids = append(ids, deliberationIDPattern.FindAllString(ref, -1)...)
	}

	unique := uniqueNonEmptyStrings(ids)
	valid := make([]string, 0, len(unique))
	for _, id := range unique {
		item, err := loadArtifact(ctx, ws, id)
		if err != nil {
			if errors.Is(err, blerrors.ErrNotFound) {
				continue
			}
			return nil, err
		}
		if item.ArtifactType == "deliberation" {
			valid = append(valid, id)
		}
	}
	return valid, nil
}

func descendantItems(ctx context.Context, ws *Workspace, parentID string) ([]*models.Artifact, error) {
	queue := []string{parentID}
	var descendants []*models.Artifact
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		children, err := bldb.QueryItems(ctx, ws.DB, bldb.QueryFilters{
			ParentID:        current,
			IncludeArchived: true,
		})
		if err != nil {
			return nil, fmt.Errorf("query children for %s: %w", current, err)
		}
		for _, child := range children {
			descendants = append(descendants, child)
			queue = append(queue, child.ID)
		}
	}
	return descendants, nil
}

func setArtifactStatus(ctx context.Context, ws *Workspace, itemID string, newStatus models.ArtifactStatus, reason string) (*models.Artifact, error) {
	artifact, err := loadArtifact(ctx, ws, itemID)
	if err != nil {
		return nil, err
	}
	if artifact.Status == newStatus {
		return artifact, nil
	}

	previous := artifact.Status
	artifact.Status = newStatus
	artifact.UpdatedAt = models.NowUTC()
	clearStaleBlockedReason(artifact, previous)
	if err := persistArtifact(ctx, ws, artifact, shouldRelocateOnStatusChange(previous, newStatus)); err != nil {
		return nil, err
	}
	appendItemEvent(ctx, ws, itemID, "status_changed", map[string]any{
		"from":   string(previous),
		"to":     string(newStatus),
		"reason": reason,
	})
	if err := cascadePersistedParentStatuses(ctx, ws, itemID); err != nil {
		return nil, err
	}
	return artifact, nil
}

func cascadePersistedParentStatuses(ctx context.Context, ws *Workspace, itemID string) error {
	item, err := loadArtifact(ctx, ws, itemID)
	if err != nil {
		return err
	}
	if item.ParentID == "" {
		return nil
	}

	newStatus, err := ComputeParentStatus(ctx, ws.DB, item.ParentID)
	if err != nil {
		return err
	}
	parent, err := loadArtifact(ctx, ws, item.ParentID)
	if err != nil {
		return err
	}
	if parent.Status == newStatus {
		return nil
	}

	previous := parent.Status
	parent.Status = newStatus
	parent.UpdatedAt = models.NowUTC()
	clearStaleBlockedReason(parent, previous)
	if err := persistArtifact(ctx, ws, parent, shouldRelocateOnStatusChange(previous, newStatus)); err != nil {
		return err
	}
	appendItemEvent(ctx, ws, parent.ID, "status_changed", map[string]any{
		"from":   string(previous),
		"to":     string(newStatus),
		"reason": "child status rollup",
	})
	return cascadePersistedParentStatuses(ctx, ws, parent.ID)
}

// clearParentID removes the parent_id from an artifact, making it an
// unparented backlog item. The hierarchical ID prefix is preserved as
// provenance; only the active ownership link is severed.
func clearParentID(ctx context.Context, ws *Workspace, itemID string) error {
	artifact, err := loadArtifact(ctx, ws, itemID)
	if err != nil {
		return fmt.Errorf("clear parent_id for %s: %w", itemID, err)
	}
	if artifact.ParentID == "" {
		return nil
	}
	artifact.ParentID = ""
	artifact.UpdatedAt = models.NowUTC()
	if err := persistArtifact(ctx, ws, artifact, false); err != nil {
		return fmt.Errorf("persist cleared parent_id for %s: %w", itemID, err)
	}
	return nil
}

// AdoptItemResult summarizes the outcome of an adopt operation.
type AdoptItemResult struct {
	ItemID               string   `json:"item_id"`
	NewID                string   `json:"new_id,omitempty"`
	NewParentID          string   `json:"new_parent_id"`
	OriginFeature        string   `json:"origin_feature,omitempty"`
	IsOrphan             bool     `json:"was_orphan"`
	RewrittenArtifactIDs []string `json:"rewritten_artifact_ids,omitempty"`
}

// AdoptItem sets an orphaned or unparented item's parent_id to a new feature,
// atomically rewriting its hierarchical ID, renaming files, updating dependency
// and link edges, and syncing the index. The return value includes the new ID
// so callers can update their own references. Adoption rewrites internal
// backlogit references only; external references are the caller's responsibility.
func AdoptItem(ctx context.Context, ws *Workspace, itemID, newParentID string) (*AdoptItemResult, error) {
	lockedCtx, releaseArtifactLocks, lockErr := lockArtifactMutations(ctx, ws, []string{itemID})
	if lockErr != nil {
		return nil, fmt.Errorf("adopt item %s: acquire mutation lock: %w", itemID, lockErr)
	}
	defer func() { _ = releaseArtifactLocks() }()
	ctx = lockedCtx

	if newParentID == "" {
		return nil, fmt.Errorf("adopt item %s: new_parent_id is required", itemID)
	}

	// Validate the new parent exists.
	if _, err := loadArtifact(ctx, ws, newParentID); err != nil {
		return nil, fmt.Errorf("adopt item %s: load new parent %s: %w", itemID, newParentID, err)
	}

	artifact, err := loadArtifact(ctx, ws, itemID)
	if err != nil {
		return nil, fmt.Errorf("adopt item %s: %w", itemID, err)
	}

	if artifact.Status == models.StatusArchived {
		return nil, fmt.Errorf("adopt item %s: cannot adopt an archived item", itemID)
	}

	oldParentID := artifact.ParentID

	// Fire pre-adopt hooks.
	if ws.HookRunner != nil {
		hookCtx := hooks.HookContext{
			ItemID:       itemID,
			ArtifactType: artifact.ArtifactType,
			OldValues:    map[string]any{"parent_id": oldParentID},
			NewValues:    map[string]any{"parent_id": newParentID},
			Actor:        "backlogit",
			Workspace:    ws.RootPath,
			TopLevel:     true,
		}
		if err := ws.HookRunner.FirePre(ctx, hooks.HookAdoptItem, hookCtx); err != nil {
			return nil, fmt.Errorf("pre-adopt hook: %w", err)
		}
	}

	wasOrphan := IsOrphan(artifact)

	// Record origin_feature from the ID prefix if not already set.
	originFeature := extractOriginFeatureID(ws, itemID)
	if artifact.CustomFields == nil {
		artifact.CustomFields = map[string]any{}
	}
	if _, exists := artifact.CustomFields["origin_feature"]; !exists && originFeature != "" {
		artifact.CustomFields["origin_feature"] = originFeature
	}

	// Step 1: Generate the new hierarchical ID under the new parent.
	oldID := artifact.ID
	newID := oldID // fallback: keep old ID if we can't generate a new one
	if ws.Config != nil && ws.Config.QueueLayout != nil {
		typeCfg, typeOK := ws.Config.ArtifactTypes[artifact.ArtifactType]
		if typeOK && typeCfg != nil {
			generatedID, idErr := NextTypedHierarchicalID(
				ctx, ws.DB, newParentID, artifact.ArtifactType,
				typeCfg, ws.Config.QueueLayout,
			)
			if idErr == nil {
				newID = generatedID
			}
		}
	}

	// Update the artifact with new parent and ID.
	artifact.ParentID = newParentID
	artifact.ID = newID
	artifact.UpdatedAt = models.NowUTC()

	// Scan for other artifacts that reference oldID in their frontmatter.
	// This is done outside the transaction (read-only) so the walk does not
	// contend with the write transaction that follows.
	crossRefs, crossRefErr := findCrossArtifactReferences(ctx, ws, oldID, newID)
	if crossRefErr != nil {
		return nil, fmt.Errorf("adopt item %s: scan cross-references: %w", oldID, crossRefErr)
	}

	// Step 2: Begin DB transaction for edge rewrites and index sync.
	// durSyncErr accumulates post-mutation directory-fsync failures from the
	// ID-change branch. It is surfaced as ErrWriteIndeterminate just before the
	// successful return (commit-then-surface) so the completed adopt is never
	// rolled back yet the durability signal is not silently discarded.
	var durSyncErr error
	if ws.DB != nil && newID != oldID {
		tx, txErr := ws.DB.BeginTx(ctx, nil)
		if txErr != nil {
			return nil, fmt.Errorf("adopt item %s: begin tx: %w", oldID, txErr)
		}
		defer tx.Rollback() //nolint:errcheck

		// Compute log paths: absolute paths for filesystem ops, relative
		// (to .backlogit/) paths for the DB to match IndexEvent's convention.
		logsDir := WorkspaceLogsRoot(ws.RootPath)
		oldLogPath := filepath.Join(logsDir, oldID+".jsonl")
		newLogPath := filepath.Join(logsDir, newID+".jsonl")
		newRelLogPath := filepath.ToSlash(filepath.Join("logs", newID+".jsonl"))

		// Rewrite dependency and link edges.
		if err := bldb.RewriteDependencyEdges(ctx, tx, oldID, newID); err != nil {
			return nil, fmt.Errorf("adopt item %s: %w", oldID, err)
		}
		if err := bldb.RewriteLinkEdges(ctx, tx, oldID, newID); err != nil {
			return nil, fmt.Errorf("adopt item %s: %w", oldID, err)
		}

		// Rewrite ancillary references (commit_links, stash_links, item_logs,
		// item_log_entries) so the index remains fully self-consistent.
		if err := bldb.RewriteAncillaryReferences(ctx, tx, oldID, newID, newRelLogPath); err != nil {
			return nil, fmt.Errorf("adopt item %s: %w", oldID, err)
		}

		// Delete old index row and insert new one.
		// Use a non-cascading delete — edges are already rewritten above.
		if _, delErr := tx.ExecContext(ctx, `DELETE FROM items WHERE id = ?`, oldID); delErr != nil {
			return nil, fmt.Errorf("adopt item %s: delete old index: %w", oldID, delErr)
		}
		if err := bldb.UpsertItemTx(ctx, tx, artifact); err != nil {
			return nil, fmt.Errorf("adopt item %s: upsert new index: %w", oldID, err)
		}

		// Step 3: File operations — rename .md and .jsonl files.
		oldMDPath, findErr := FindArtifactPath(ctx, ws, oldID)
		if findErr != nil && !errors.Is(findErr, blerrors.ErrNotFound) {
			return nil, fmt.Errorf("adopt item %s: find old md: %w", oldID, findErr)
		}

		var renamedMD, renamedLog bool
		var newMDPath string
		// 060.004-T: Snapshot the old file content before overwriting with the new
		// ID so every rollback path can restore the exact original file content.
		// Without this, the rename-based rollback places a file with frontmatter
		// "id: newID" at oldMDPath, making the artifact undiscoverable by oldID.
		var oldMDRaw []byte

		if findErr == nil {
			// Snapshot the original content before any file operations.
			oldMDRaw, err = os.ReadFile(oldMDPath)
			if err != nil {
				return nil, fmt.Errorf("adopt item %s: read old md: %w", oldID, err)
			}

			// Compute new filename using the configured naming resolver to
			// respect artifact_types[*].file_name_format when configured.
			dir := filepath.Dir(oldMDPath)
			newFileName := newID // default: use the artifact ID as filename
			if ws.Config != nil {
				if typeCfg, ok := ws.Config.ArtifactTypes[artifact.ArtifactType]; ok && typeCfg != nil {
					newFileName = ResolveFileName(typeCfg, newID, artifact.Title, ws.Config.MaxSlugLength)
				}
			}
			newMDPath = filepath.Join(dir, newFileName+".md")

			// Write updated artifact content (with new ID in frontmatter) to new path.
			if writeErr := WriteArtifactFileWithOptions(artifact, newMDPath, WorkspaceDurableWrites(ws)); writeErr != nil {
				return nil, fmt.Errorf("adopt item %s: write new md: %w", oldID, writeErr)
			}
			if newMDPath != oldMDPath {
				if rmErr := os.Remove(oldMDPath); rmErr != nil && !os.IsNotExist(rmErr) {
					// Rollback: remove the new file we just wrote
					_ = os.Remove(newMDPath)
					return nil, fmt.Errorf("adopt item %s: remove old md: %w", oldID, rmErr)
				}
				renamedMD = true
				// Durable same-directory rename: the artifact write fsynced the new
				// dirent, but the old-ID entry was just removed afterward and that
				// removal is not durable until the directory is fsynced again. This
				// runs after the in-tx DB mutations, so a failure is NOT rolled back
				// (the rename likely persisted); it is accumulated and surfaced as
				// ErrWriteIndeterminate after tx.Commit (commit-then-surface).
				if e := durableSyncDirDetailed(ws, filepath.Dir(oldMDPath), "adopt md rename"); e != nil {
					durSyncErr = errors.Join(durSyncErr, e)
				}
			}
		}

		// Rewrite frontmatter in other artifacts that reference oldID.
		if applyErr := applyCrossArtifactRewrites(ctx, tx, ws, crossRefs); applyErr != nil {
			if renamedMD {
				rollbackMDFile(newMDPath, oldMDPath, oldMDRaw)
			}
			return nil, fmt.Errorf("adopt item %s: apply cross-artifact rewrites: %w", oldID, applyErr)
		}

		// Rename log file if it exists.
		if _, statErr := os.Stat(oldLogPath); statErr == nil {
			if renameErr := os.Rename(oldLogPath, newLogPath); renameErr != nil {
				// Rollback MD rename
				if renamedMD {
					rollbackMDFile(newMDPath, oldMDPath, oldMDRaw)
				}
				return nil, fmt.Errorf("adopt item %s: rename log: %w", oldID, renameErr)
			}
			renamedLog = true
			// Durable log rename: both the new and removed dirents live in logsDir;
			// fsync it so the rename is durable. Same commit-then-surface handling
			// as the MD rename above: a failure is accumulated and surfaced as
			// ErrWriteIndeterminate after commit, never rolled back.
			if e := durableSyncDirDetailed(ws, filepath.Dir(oldLogPath), "adopt log rename"); e != nil {
				durSyncErr = errors.Join(durSyncErr, e)
			}
		}

		// Step 4: Commit the transaction now that all file ops succeeded.
		if commitErr := tx.Commit(); commitErr != nil {
			// Rollback file operations
			if renamedLog {
				_ = os.Rename(newLogPath, oldLogPath)
			}
			if renamedMD {
				rollbackMDFile(newMDPath, oldMDPath, oldMDRaw)
			}
			for _, u := range crossRefs {
				tmp := u.filePath + ".rollback-tmp"
				if writeErr := os.WriteFile(tmp, u.snapshotRaw, 0o644); writeErr != nil {
					slog.Warn("adopt item: rollback cross-ref write failed", "path", u.filePath, "error", writeErr)
					continue
				}
				if renameErr := os.Rename(tmp, u.filePath); renameErr != nil {
					slog.Warn("adopt item: rollback cross-ref rename failed", "path", u.filePath, "error", renameErr)
					_ = os.Remove(tmp)
				}
			}
			return nil, fmt.Errorf("adopt item %s: commit tx: %w", oldID, commitErr)
		}
	} else {
		// No ID change or no DB — just persist the artifact with updated parent.
		if err := persistArtifact(ctx, ws, artifact, false); err != nil {
			return nil, fmt.Errorf("adopt item %s: persist: %w", itemID, err)
		}
		if ws.DB != nil {
			if err := bldb.UpsertItem(ctx, ws.DB, artifact); err != nil {
				return nil, fmt.Errorf("adopt item %s: index: %w", itemID, err)
			}
		}
	}

	// Build list of rewritten artifact IDs for the event delta and result.
	rewrittenIDs := make([]string, 0, len(crossRefs))
	for _, u := range crossRefs {
		rewrittenIDs = append(rewrittenIDs, u.artifact.ID)
	}

	appendItemEvent(ctx, ws, newID, "adopted", map[string]any{
		"old_id":                 oldID,
		"new_id":                 newID,
		"new_parent_id":          newParentID,
		"origin_feature":         originFeature,
		"was_orphan":             wasOrphan,
		"rewritten_artifact_ids": rewrittenIDs,
	})

	// Fire post-adopt hooks.
	if ws.HookRunner != nil {
		hookCtx := hooks.HookContext{
			ItemID:       newID,
			ArtifactType: artifact.ArtifactType,
			OldValues:    map[string]any{"parent_id": oldParentID, "id": oldID},
			NewValues:    map[string]any{"parent_id": newParentID, "id": newID},
			Actor:        "backlogit",
			Workspace:    ws.RootPath,
			TopLevel:     true,
		}
		ws.HookRunner.FirePost(ctx, hooks.HookAdoptItem, hookCtx)
	}

	return &AdoptItemResult{
		ItemID:               oldID,
		NewID:                newID,
		NewParentID:          newParentID,
		OriginFeature:        originFeature,
		IsOrphan:             wasOrphan,
		RewrittenArtifactIDs: rewrittenIDs,
	}, adoptDurabilityErr(newID, durSyncErr)
}

// adoptDurabilityErr returns a wrapped ErrWriteIndeterminate when a post-mutation
// directory fsync failed during a committed adopt, and nil otherwise. The adopt
// already applied (files renamed, tx committed, event appended), so the caller
// still receives the fully-built result; wrapping the sentinel lets an err-only
// caller and blerrors.IsWriteIndeterminate see the honest durability signal
// without triggering a rollback of the completed operation.
func adoptDurabilityErr(newID string, durSyncErr error) error {
	if durSyncErr == nil {
		return nil
	}
	return fmt.Errorf("adopt item %s applied but durability is indeterminate: %w: %w",
		newID, blerrors.ErrWriteIndeterminate, durSyncErr)
}

// IsOrphan returns true when an item has no parent_id but its hierarchical ID
// suggests it originated under a parent (contains a dot separator).
func IsOrphan(a *models.Artifact) bool {
	return a.ParentID == "" && strings.Contains(a.ID, ".")
}

// rollbackMDFile undoes a WriteArtifactFile + os.Remove pair that advanced
// the artifact from oldMDPath → newMDPath. It removes newMDPath and restores
// oldMDPath to its original byte content. If oldMDRaw is empty it falls back
// to a simple rename (pre-060.004-T behaviour).
func rollbackMDFile(newMDPath, oldMDPath string, oldMDRaw []byte) {
	if len(oldMDRaw) == 0 {
		// Fallback: rename may place new-ID content at oldMDPath, but it is
		// better than leaving the workspace with no file at all.
		if renameErr := os.Rename(newMDPath, oldMDPath); renameErr != nil {
			slog.Warn("adopt item: rollback md rename failed", "from", newMDPath, "to", oldMDPath, "error", renameErr)
		}
		return
	}

	if removeErr := os.Remove(newMDPath); removeErr != nil && !os.IsNotExist(removeErr) {
		slog.Warn("adopt item: rollback md remove failed", "path", newMDPath, "error", removeErr)
	}
	if restoreErr := os.WriteFile(oldMDPath, oldMDRaw, 0o644); restoreErr != nil {
		slog.Warn("adopt item: rollback md content restore failed", "path", oldMDPath, "error", restoreErr)
	}
}

// extractIDPrefix returns the portion of a hierarchical ID before the last dot
// segment. For "F015.T009" it returns "F015". For "T009" it returns "".
func extractIDPrefix(id string) string {
	idx := strings.LastIndex(id, ".")
	if idx < 0 {
		return ""
	}
	return id[:idx]
}

func extractOriginFeatureID(ws *Workspace, id string) string {
	prefix := extractIDPrefix(id)
	if prefix == "" {
		return ""
	}

	root := prefix
	if idx := strings.Index(prefix, "."); idx >= 0 {
		root = prefix[:idx]
	}
	if strings.Contains(root, "-") {
		return root
	}

	digits := leadingDigits(root)
	if digits == "" || ws == nil || ws.Config == nil {
		return prefix
	}
	featureCfg, ok := ws.Config.ArtifactTypes["feature"]
	if !ok || featureCfg == nil || featureCfg.Suffix == "" {
		return prefix
	}
	return digits + featureCfg.Suffix
}

func shouldRelocateOnStatusChange(previous models.ArtifactStatus, next models.ArtifactStatus) bool {
	return previous != next
}

// isTerminalReleaseStatus reports whether status is releasable for shipment
// relocation/lifecycle transitions. It delegates to the authoritative
// IsReleasableStatus predicate (the 4-status set {done, accepted, rejected,
// archived}); the five release-progression call sites remain behaviorally unchanged.
func isTerminalReleaseStatus(status models.ArtifactStatus) bool {
	return IsReleasableStatus(status)
}

// isDescopeEligibleStatus reports whether a member archived FROM the given status
// is a GENUINE DESCOPE — scaffolded then removed from the release before shipping a
// deliverable — and is therefore exempt from the per-member F4 gate-evidence
// requirement. Two status classes qualify:
//
//   - In-flight statuses (queued, active, blocked, review) never reached completion.
//   - Non-completion terminals (abandoned, rejected) ended the item WITHOUT shipping
//     a deliverable, so there is no completion contract to gate.
//
// COMPLETION statuses (done, accepted, shipped) are NEVER descope-eligible: a member
// driven to completion and then archived MUST still present valid gate evidence, or
// the F4 fail-open evidence predicate would be bypassed (a completed member whose only
// "pass" is an EventGatePassed{ran:false} carries no valid evidence yet could be
// archived after the fact). The archived sink status is excluded because it is not a
// pre-archive provenance value. This predicate is distinct from isTerminalReleaseStatus
// (which governs relocation and lifecycle transitions and MUST NOT change): terminality
// and descope-eligibility are orthogonal — rejected is terminal yet descope-eligible,
// abandoned is non-terminal yet descope-eligible, and shipped is non-terminal yet a
// completion (never descope-eligible).
func isDescopeEligibleStatus(status models.ArtifactStatus) bool {
	switch status {
	case models.StatusQueued, models.StatusActive, models.StatusBlocked,
		models.StatusReview, models.StatusAbandoned, models.StatusRejected:
		return true
	default:
		return false
	}
}

// isRecognizedReleaseStatus reports whether status is one of the known artifact
// lifecycle statuses. Unrecognized (malformed/typo) provenance must be treated as
// unknown so safety-critical callers can fail closed rather than misclassify it:
// isTerminalReleaseStatus returns false for any unknown value, so an exemption gated
// only on !isTerminalReleaseStatus would wrongly treat garbage provenance as a
// non-terminal descope.
func isRecognizedReleaseStatus(status models.ArtifactStatus) bool {
	switch status {
	case models.StatusQueued, models.StatusActive, models.StatusBlocked,
		models.StatusReview, models.StatusDone, models.StatusAccepted,
		models.StatusRejected, models.StatusArchived, models.StatusShipped,
		models.StatusAbandoned:
		return true
	default:
		return false
	}
}

func toIDSet(itemIDs []string) map[string]struct{} {
	set := make(map[string]struct{}, len(itemIDs))
	for _, itemID := range uniqueNonEmptyStrings(itemIDs) {
		set[itemID] = struct{}{}
	}
	return set
}

func depthSortedIDs(itemIDs []string) []string {
	ids := uniqueNonEmptyStrings(itemIDs)
	sort.Slice(ids, func(i, j int) bool {
		leftDepth := strings.Count(ids[i], ".")
		rightDepth := strings.Count(ids[j], ".")
		if leftDepth != rightDepth {
			return leftDepth > rightDepth
		}
		return ids[i] < ids[j]
	})
	return ids
}

func commitSHA(commit *CommitMetadata) string {
	if commit == nil {
		return ""
	}
	return strings.TrimSpace(commit.SHA)
}
