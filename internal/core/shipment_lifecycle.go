package core

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	bldb "github.com/softwaresalt/backlogit/internal/db"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/hooks"
	"github.com/softwaresalt/backlogit/internal/models"
)

var deliberationIDPattern = regexp.MustCompile(`\b(?:DL\d+|[0-9]+(?:\.[0-9]+)*-DL)\b`)

type shipmentMemberCascadeBoundaryContextKey struct{}

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
	globalUnlock, err := lockShipmentMembership(ctx, ws, shipmentLifecycleGlobalLockID)
	if err != nil {
		return nil, fmt.Errorf("lock shipment lifecycle: %w", err)
	}
	defer func() {
		if unlockErr := globalUnlock(); unlockErr != nil {
			slog.WarnContext(ctx, "release shipment lifecycle lock", "shipment_id", shipmentID, "error", unlockErr)
		}
	}()
	ctx = context.WithValue(ctx, shipmentLifecycleGlobalLockContextKey{}, struct{}{})
	if err := recoverPendingShipmentOperations(ctx, ws); err != nil {
		return nil, fmt.Errorf("recover pending shipment operations before claim: %w", err)
	}

	membershipUnlock, err := lockShipmentMembership(ctx, ws, shipmentID)
	if err != nil {
		return nil, fmt.Errorf("lock shipment %s membership: %w", shipmentID, err)
	}
	defer func() {
		if unlockErr := membershipUnlock(); unlockErr != nil {
			slog.WarnContext(ctx, "release shipment membership lock", "shipment_id", shipmentID, "error", unlockErr)
		}
	}()

	current, err := GetShipment(ctx, ws, shipmentID)
	if err != nil {
		return nil, err
	}
	if current.Status != models.StatusQueued {
		return nil, fmt.Errorf("claim shipment %s from %s: %w", shipmentID, current.Status, blerrors.ErrShipmentConflict)
	}
	if err := ensureShipmentActiveSlotAvailable(ws, shipmentID); err != nil {
		return nil, fmt.Errorf("claim shipment %s: %w", shipmentID, err)
	}

	memberIDs := NormalizeShipmentItems(current)
	relatedIDs, err := shipmentClaimAncestorIDs(ctx, ws, memberIDs)
	if err != nil {
		return nil, fmt.Errorf("collect shipment %s claim ancestors: %w", shipmentID, err)
	}
	memberSet := make(map[string]struct{}, len(memberIDs)+1)
	memberSet[shipmentID] = struct{}{}
	for _, memberID := range memberIDs {
		memberSet[memberID] = struct{}{}
	}
	filteredRelatedIDs := make([]string, 0, len(relatedIDs))
	for _, relatedID := range relatedIDs {
		if _, exists := memberSet[relatedID]; !exists {
			filteredRelatedIDs = append(filteredRelatedIDs, relatedID)
		}
	}
	lockIDs := uniqueNonEmptyStrings(append(append([]string{shipmentID}, memberIDs...), filteredRelatedIDs...))
	lockedCtx, artifactUnlock, err := lockArtifactMutations(ctx, ws, lockIDs)
	if err != nil {
		return nil, fmt.Errorf("lock shipment %s claim aggregate: %w", shipmentID, err)
	}
	defer func() {
		if unlockErr := artifactUnlock(); unlockErr != nil {
			slog.WarnContext(ctx, "release shipment claim aggregate locks", "shipment_id", shipmentID, "error", unlockErr)
		}
	}()

	current, err = findArtifact(lockedCtx, ws, shipmentID)
	if err != nil {
		return nil, fmt.Errorf("reload shipment %s under claim locks: %w", shipmentID, err)
	}
	normalizeShipmentArtifact(current)
	if current.Status != models.StatusQueued || !slices.Equal(NormalizeShipmentItems(current), memberIDs) {
		return nil, fmt.Errorf("shipment %s changed while acquiring claim locks: %w", shipmentID, blerrors.ErrShipmentConflict)
	}

	preimage := shipmentLifecyclePreimage{
		Shipment: cloneArtifact(current),
		Members:  make([]*models.Artifact, 0, len(memberIDs)),
		Related:  make([]*models.Artifact, 0, len(filteredRelatedIDs)),
	}
	for _, memberID := range memberIDs {
		member, loadErr := findArtifact(lockedCtx, ws, memberID)
		if loadErr != nil {
			return nil, fmt.Errorf("load shipment %s claim member %s: %w", shipmentID, memberID, loadErr)
		}
		preimage.Members = append(preimage.Members, cloneArtifact(member))
	}
	for _, relatedID := range filteredRelatedIDs {
		related, loadErr := findArtifact(lockedCtx, ws, relatedID)
		if loadErr != nil {
			return nil, fmt.Errorf("load shipment %s claim related artifact %s: %w", shipmentID, relatedID, loadErr)
		}
		preimage.Related = append(preimage.Related, cloneArtifact(related))
	}

	snapshots, err := snapshotShipArtifacts(lockedCtx, ws, lockIDs)
	if err != nil {
		return nil, fmt.Errorf("snapshot shipment %s claim aggregate: %w", shipmentID, err)
	}
	var correlationBytes [16]byte
	if _, err := rand.Read(correlationBytes[:]); err != nil {
		return nil, fmt.Errorf("generate claim shipment correlation id: %w", err)
	}
	correlationID := hex.EncodeToString(correlationBytes[:])
	journal := shipmentLifecycleJournal{
		SchemaVersion:  "shipment-operation/v1",
		CorrelationID:  correlationID,
		Phase:          "intent",
		Operation:      "claim",
		RecoveryPolicy: "rollback",
		ShipmentID:     shipmentID,
		Target:         string(ShipmentActive),
		Preimage:       preimage,
	}
	journalName := shipmentLifecycleJournalName(correlationID)
	journalPath, err := writeShipmentLifecycleJournalForWorkspace(ws, journalName, journal)
	if err != nil {
		return nil, fmt.Errorf("persist claim shipment %s intent: %w", shipmentID, err)
	}

	operationCtx := withShipmentOperation(lockedCtx, correlationID)
	rollback := func(cause error) error {
		return rollbackShipmentClaim(operationCtx, ws, journalPath, journal, snapshots, cause)
	}
	activationCtx := context.WithValue(operationCtx, governedShipmentActivationContextKey{}, struct{}{})
	if err := MoveShipmentStatus(activationCtx, ws, shipmentID, ShipmentActive); err != nil {
		return nil, rollback(err)
	}

	shipment, err := GetShipment(operationCtx, ws, shipmentID)
	if err != nil {
		return nil, rollback(fmt.Errorf("reload shipment after activation: %w", err))
	}
	for _, member := range preimage.Members {
		if member.Status != models.StatusQueued {
			continue
		}
		if _, setErr := setArtifactStatus(operationCtx, ws, member.ID, models.StatusActive, "shipment claimed"); setErr != nil {
			return nil, rollback(fmt.Errorf("activate item %s: %w", member.ID, setErr))
		}
	}

	journal.Phase = "committed"
	if _, err := writeShipmentLifecycleJournalForWorkspace(ws, journalName, journal); err != nil {
		return nil, rollback(fmt.Errorf("persist claim shipment %s commit: %w", shipmentID, err))
	}
	removeShipmentOperationJournal(operationCtx, journalPath)
	return shipment, nil
}

// rollbackShipmentClaim restores the exact pre-claim artifact and event
// snapshots. The durable intent remains recoverable until compensation reaches
// a terminal phase.
func rollbackShipmentClaim(
	ctx context.Context,
	ws *Workspace,
	journalPath string,
	journal shipmentLifecycleJournal,
	snapshots map[string]shipArtifactSnapshot,
	claimErr error,
) error {
	if claimErr == nil {
		claimErr = fmt.Errorf("claim rollback invoked without a triggering error")
	}
	unrestored, rollbackErr := restoreShipArtifactsDetailed(ctx, ws, snapshots)
	compensationState := "compensated"
	if rollbackErr != nil {
		compensationState = "partially-compensated"
	} else {
		journal.Phase = "compensated"
		if _, err := writeShipmentLifecycleJournalForWorkspace(
			ws,
			filepath.Base(journalPath),
			journal,
		); err != nil {
			rollbackErr = fmt.Errorf("persist claim compensation journal: %w", err)
		} else {
			removeShipmentOperationJournal(ctx, journalPath)
		}
	}
	if rollbackErr != nil {
		cause := errors.Join(claimErr, rollbackErr)
		if len(unrestored) > 0 {
			cause = errors.Join(cause, fmt.Errorf("claim compensation could not restore: %s", strings.Join(unrestored, ", ")))
		}
		return &blerrors.MutationPartialError{
			Completed:         []string{"claim-activation"},
			FailedStep:        "claim-compensation",
			CompensationState: compensationState,
			Class:             "double-fault",
			Cause:             cause,
		}
	}
	return fmt.Errorf("claim shipment %s: %w", journal.ShipmentID, claimErr)
}

type shipArtifactSnapshot struct {
	artifact *models.Artifact
	dbItem   *models.Artifact
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
		dbItem, err := bldb.GetItem(ctx, ws.DB, id)
		if err != nil {
			return nil, fmt.Errorf("snapshot artifact %s index: %w", id, err)
		}
		path, err := FindArtifactPath(ctx, ws, id)
		if err != nil {
			return nil, fmt.Errorf("snapshot artifact %s path: %w", id, err)
		}
		file, err := snapshotFile(path)
		if err != nil {
			return nil, fmt.Errorf("snapshot artifact %s file: %w", id, err)
		}
		_, unlockItemLog, lockErr := events.LockItemLogCrossProcess(ctx, WorkspaceLocksRoot(ws.RootPath), WorkspaceLogsRoot(ws.RootPath), id)
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
			dbItem:   cloneArtifact(dbItem),
			file:     file,
			eventLog: eventLog,
		}
	}
	return snapshots, nil
}

// shipRestoreRetryWindow bounds the per-CALL retry budget the compensation loop
// spends re-acquiring item-log locks. It is expressed as a wall-clock deadline
// as well as an attempt count because the in-process acquisition inside
// events.LockItemLogCrossProcess is itself unbounded (a plain, uncancellable
// mutex.Lock); the deadline is therefore a best-effort bound on the retry LOOP,
// not a hard bound on any single acquisition. The budget is per call, not per
// item, because this loop runs inside the ship closure's rollback defer with the
// membership lock and every artifact lock held. Two cross-process wait periods
// (3 seconds each) is the ceiling.
const shipRestoreRetryWindow = 6 * time.Second

const shipmentRecoveryTimeout = shipRestoreRetryWindow

// shipRestoreRetryAttempts is the number of EXTRA lock acquisitions the whole
// call may spend, shared across all items.
const shipRestoreRetryAttempts = 2

var restoreShipmentSnapshotFn = restoreSnapshot

func boundedShipmentRecoveryContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), shipmentRecoveryTimeout)
}

// restoreShipArtifacts compensates a failed ship by restoring every snapshot.
// It returns only the joined error; callers that must report which items could
// not be restored use restoreShipArtifactsDetailed.
func restoreShipArtifacts(ctx context.Context, ws *Workspace, snapshots map[string]shipArtifactSnapshot) error {
	_, err := restoreShipArtifactsDetailed(ctx, ws, snapshots)
	return err
}

// restoreShipArtifactsDetailed restores every snapshot and additionally reports
// the IDs it could NOT restore, so a caller can promote the outcome to
// CompensationState "partially-compensated" instead of silently skipping an
// item. Compensation is all-or-nothing or it says so.
//
// The per-item body runs inside a closure with a NIL-GUARDED deferred unlock.
// An early return inside the loop is forbidden: unlockItemLog is a plain
// statement in the original loop, not a defer, so an early return would leak the
// process-global item-log mutex, which has no deadline. A blanket
// `defer unlockItemLog()` is equally unsafe because
// events.LockItemLogCrossProcess returns a NIL unlock on error.
func restoreShipArtifactsDetailed(ctx context.Context, ws *Workspace, snapshots map[string]shipArtifactSnapshot) ([]string, error) {
	ctx, cancel := boundedShipmentRecoveryContext(ctx)
	defer cancel()
	logsDir := WorkspaceLogsRoot(ws.RootPath)
	locksRoot := WorkspaceLocksRoot(ws.RootPath)
	operationID := shipmentOperationID(ctx)
	ids := make([]string, 0, len(snapshots))
	for id := range snapshots {
		ids = append(ids, id)
	}
	var errs []error
	unrestored := make(map[string]struct{})
	budget := &shipRestoreBudget{deadline: time.Now().Add(shipRestoreRetryWindow), attempts: shipRestoreRetryAttempts}
	for _, id := range depthSortedIDs(ids) {
		func() {
			itemCtx, unlockItemLog, lockErr := acquireItemLogWithBudget(ctx, locksRoot, logsDir, id, budget)
			if lockErr != nil {
				errs = append(errs, fmt.Errorf("lock artifact %s event log: %w", id, lockErr))
				unrestored[id] = struct{}{}
				return
			}
			defer func() {
				if unlockItemLog != nil {
					unlockItemLog()
				}
			}()
			itemFailed := false
			fail := func(err error) {
				errs = append(errs, err)
				itemFailed = true
			}
			defer func() {
				if itemFailed {
					unrestored[id] = struct{}{}
				}
			}()

			snapshot := snapshots[id]
			currentEvents, readErr := events.ReadAllEvents(itemCtx, logsDir, id)
			var preservedEvents []events.Event
			eventLogRestorable := readErr == nil
			if readErr != nil {
				fail(fmt.Errorf("read mutated artifact %s event log: %w", id, readErr))
			} else if preservedEvents, readErr = eventsSinceSnapshot(snapshot.eventLog, id, currentEvents, operationID); readErr != nil {
				eventLogRestorable = false
				fail(fmt.Errorf("identify concurrent events for %s: %w", id, readErr))
			}
			if currentPath, err := FindArtifactPath(ctx, ws, id); err == nil && currentPath != snapshot.file.Path {
				if removeErr := os.Remove(currentPath); removeErr != nil && !os.IsNotExist(removeErr) {
					fail(fmt.Errorf("remove mutated artifact %s: %w", id, removeErr))
				}
			} else if err != nil && !errors.Is(err, blerrors.ErrNotFound) {
				fail(fmt.Errorf("locate mutated artifact %s: %w", id, err))
			}
			if err := restoreShipmentSnapshotFn(snapshot.file); err != nil {
				fail(fmt.Errorf("restore artifact %s file: %w", id, err))
			}
			if eventLogRestorable {
				if err := restoreShipmentSnapshotFn(snapshot.eventLog); err != nil {
					fail(fmt.Errorf("restore artifact %s event log: %w", id, err))
				} else {
					writer := NewWorkspaceEventWriter(ws, logsDir)
					for _, event := range preservedEvents {
						if err := writer.AppendEvent(itemCtx, event); err != nil {
							fail(fmt.Errorf("restore artifact %s concurrent event: %w", id, err))
						}
					}
					if err := bldb.ReindexItemLog(itemCtx, ws.DB, locksRoot, logsDir, id); err != nil {
						fail(fmt.Errorf("restore artifact %s event index: %w", id, err))
					}
				}
			}
			dbItem := snapshot.dbItem
			if dbItem == nil {
				dbItem = snapshot.artifact
			}
			if dbItem == nil {
				fail(fmt.Errorf("restore artifact %s index: snapshot is incomplete: %w", id, blerrors.ErrValidation))
			} else if err := bldb.UpsertItem(itemCtx, ws.DB, dbItem); err != nil {
				fail(fmt.Errorf("restore artifact %s index: %w", id, err))
			}
		}()
	}
	unrestoredIDs := make([]string, 0, len(unrestored))
	for id := range unrestored {
		unrestoredIDs = append(unrestoredIDs, id)
	}
	sort.Strings(unrestoredIDs)
	return unrestoredIDs, errors.Join(errs...)
}

// shipRestoreBudget carries the per-call retry allowance shared by every item in
// one compensation pass.
type shipRestoreBudget struct {
	deadline time.Time
	attempts int
}

// acquireItemLogWithBudget re-acquires an item-log lock, spending the shared
// per-call retry budget when the first attempt fails. It never blocks beyond the
// budget's wall-clock deadline for the retry loop itself.
func acquireItemLogWithBudget(ctx context.Context, locksRoot, logsDir, id string, budget *shipRestoreBudget) (context.Context, func(), error) {
	itemCtx, unlock, lockErr := events.LockItemLogCrossProcess(ctx, locksRoot, logsDir, id)
	for lockErr != nil && budget.attempts > 0 && time.Now().Before(budget.deadline) {
		budget.attempts--
		time.Sleep(50 * time.Millisecond)
		itemCtx, unlock, lockErr = events.LockItemLogCrossProcess(ctx, locksRoot, logsDir, id)
	}
	return itemCtx, unlock, lockErr
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
	lockedCtx, globalUnlock, globalLockErr := lockShipmentLifecycleGlobal(ctx, ws)
	if globalLockErr != nil {
		return nil, fmt.Errorf("lock shipment lifecycle for ship %s: %w", shipmentID, globalLockErr)
	}
	defer func() {
		if unlockErr := globalUnlock(); unlockErr != nil {
			slog.WarnContext(ctx, "release shipment lifecycle lock after ship", "shipment_id", shipmentID, "error", unlockErr)
		}
	}()
	ctx = lockedCtx

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
	// resolution, release-scope completion, and the shipment's own status
	// write) now runs inside this SAME locked closure so no such window
	// remains.
	var explicitScope, releaseScope, featureIDs []string
	var explicitScopeSet map[string]struct{}
	var shipSnapshots map[string]shipArtifactSnapshot
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
			// 143.004-T: classify a governed shipped-event append failure BEFORE
			// the len(shipSnapshots) guard below. Placing it after would make the
			// branch dead code whenever snapshotting did not populate the map.
			// restoreShipArtifacts stays behind that guard.
			var appendErr *shipmentEventAppendError
			if errors.As(closureErr, &appendErr) {
				outcome := classifyShippedEventAppendFailure(ctx, ws, shipmentID, appendErr, shipSnapshots)
				closureErr = outcome.err
				return
			}
			if closureErr == nil || len(shipSnapshots) == 0 {
				return
			}
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
		_, gateErr := gateShipmentCompletion(ctx, ws, shipmentID, releaseScope, explicitScope)
		if gateErr != nil {
			return gateErr
		}

		var featureErr error
		featureIDs, featureErr = featureScopeRoots(ctx, ws, explicitScope)
		if featureErr != nil {
			return fmt.Errorf("resolve feature scope: %w", featureErr)
		}

		rollbackIDs := append([]string{shipmentID}, releaseScope...)
		var artifactLockErr error
		ctx, releaseArtifactLocks, artifactLockErr = lockArtifactMutations(ctx, ws, rollbackIDs)
		if artifactLockErr != nil {
			return fmt.Errorf("lock release scope artifacts: %w", artifactLockErr)
		}
		scopedCtx := context.WithValue(ctx, shipmentMemberCascadeBoundaryContextKey{}, explicitScopeSet)
		// Re-run the completion gate after acquiring the artifact locks so no
		// concurrent artifact mutation can land between validation and snapshot.
		gatedHead, gateErr := gateShipmentCompletion(ctx, ws, shipmentID, releaseScope, explicitScope)
		if gateErr != nil {
			return gateErr
		}
		var snapshotErr error
		shipSnapshots, snapshotErr = snapshotShipArtifacts(ctx, ws, rollbackIDs)
		if snapshotErr != nil {
			return fmt.Errorf("snapshot release scope: %w", snapshotErr)
		}

		if err := completeReleaseScope(scopedCtx, ws, releaseScope); err != nil {
			return fmt.Errorf("complete release scope: %w", err)
		}

		for _, featureID := range featureIDs {
			// A member feature is completed directly even when a non-member
			// feature between it and a member descendant stops the bounded
			// parent cascade.
			if _, isMember := explicitScopeSet[featureID]; isMember {
				if _, setErr := setArtifactStatus(scopedCtx, ws, featureID, models.StatusDone, "feature released"); setErr != nil {
					return fmt.Errorf("mark feature %s done: %w", featureID, setErr)
				}
			}
		}

		return moveShipmentStatusWithHeadGuard(ctx, ws, shipmentID, ShipmentShipped, false, gatedHead)
	}()
	if lockErr != nil {
		return nil, fmt.Errorf("ship shipment %s: %w", shipmentID, lockErr)
	}

	archiveIDs, err := collectArchiveCandidateIDs(ctx, ws, shipmentID, releaseScope, featureIDs, explicitScopeSet)
	if err != nil {
		return nil, fmt.Errorf("ship shipment %s: collect archive scope: %w", shipmentID, err)
	}

	if err := attachCommitToItems(ctx, ws, archiveIDs, commit); err != nil {
		return nil, fmt.Errorf("ship shipment %s: record commit traceability: %w", shipmentID, err)
	}

	archivedIDs, err := archiveItems(ctx, ws, archiveIDs)
	if err != nil {
		return nil, fmt.Errorf("ship shipment %s: archive release scope: %w", shipmentID, err)
	}

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

func collectArchiveCandidateIDs(ctx context.Context, ws *Workspace, shipmentID string, releaseScope, featureIDs []string, explicitScope map[string]struct{}) ([]string, error) {
	candidates := []string{shipmentID}

	for _, itemID := range releaseScope {
		item, err := loadArtifact(ctx, ws, itemID)
		if err != nil {
			return nil, err
		}
		if item.Status == models.StatusArchived {
			continue
		}
		if isTerminalReleaseStatus(item.Status) {
			candidates = append(candidates, item.ID)
		}
	}

	for _, featureID := range featureIDs {
		// A covering feature is archived only when it is itself an explicit
		// shipment member. Its descendants and linked deliberations remain
		// independent unless their own IDs are explicit members.
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
	}

	return uniqueNonEmptyStrings(candidates), nil
}

// shippedEventFailureOutcome carries the classified result of a governed
// shipped-event append failure back to the rollback defer.
type shippedEventFailureOutcome struct {
	// err is the *blerrors.MutationPartialError the closure returns.
	err error
	// rollbackAttempted reports whether restoreShipArtifacts ran.
	rollbackAttempted bool
}

// classifyShippedEventAppendFailure classifies a governed shipped-event append
// failure and decides whether the ship compensates or halts.
//
// The durability guarantee this implements is PATH-SCOPED, and narrower still
// within that path. It covers the governed ShipShipment archival path and,
// inside it, only the shipment's OWN terminal status transition. The same call
// also appends best-effort item-level events -- status_changed via
// setArtifactStatus, returned_to_backlog, and the parent cascades -- and those
// stay best-effort by design. Non-ShipShipment producers (generic
// UpdateArtifactWithGate and generic ArchiveItem callers) are not prevented from
// reaching archived_status: shipped without a shipped event at all; preventing
// them is deferred, and they are covered report-only by the doctor
// shipped-event reconciliation audit.
//
// Two divergences from MutationEnvelope are deliberate and must not be "fixed"
// back:
//
//  1. An UNTAGGED error is treated as unproven and therefore indeterminate,
//     the inverse of MutationEnvelope's untagged default. The envelope's default
//     is safe because its persist step routes through a primitive that tags both
//     classes explicitly; the default non-durable append path does not tag at
//     all, and events.EventWriter.AppendEvent exposes only error while its
//     non-durable append discards the byte count. Compensating over a
//     possibly-applied append to an append-only log is the one outcome the
//     AppendEvent contract forbids.
//  2. This branch HALTS rather than continuing the remaining steps. Archival is
//     deliberately not attempted, leaving a detectable, documented
//     shipped-and-unarchived residue instead of a permanently missing audit
//     record.
//
// Classification precedence is indeterminate-first: an error carrying BOTH
// sentinels classifies indeterminate and can never be compensated.
func classifyShippedEventAppendFailure(
	ctx context.Context,
	ws *Workspace,
	shipmentID string,
	appendErr *shipmentEventAppendError,
	shipSnapshots map[string]shipArtifactSnapshot,
) shippedEventFailureOutcome {
	completed := []string{"complete-release-scope", "persist-shipment-status"}

	if blerrors.IsWriteIndeterminate(appendErr) || !blerrors.IsWriteNotApplied(appendErr) {
		// Indeterminate: suppress restoreShipArtifacts entirely and halt
		// archival because the shipment event may already be durable.
		logShippedEventAppendFailure(ctx, shipmentID, "indeterminate", "not-compensated", nil, appendErr)
		return shippedEventFailureOutcome{
			err: &blerrors.MutationPartialError{
				Completed:         completed,
				FailedStep:        blerrors.StepShippedEventAppend,
				CompensationState: "not-compensated",
				Class:             "indeterminate",
				Cause:             appendErr,
			},
			rollbackAttempted: false,
		}
	}

	// Proven not-applied: compensate through the existing snapshot rollback.
	// Compensation is all-or-nothing or it says so: an item the loop could not
	// restore promotes the result to "partially-compensated" and is named, never
	// silently skipped.
	compensationState := "compensated"
	var unrestoredIDs []string
	cause := error(appendErr)
	if len(shipSnapshots) > 0 {
		unrestored, rollbackErr := restoreShipArtifactsDetailed(ctx, ws, shipSnapshots)
		if rollbackErr != nil {
			cause = errors.Join(cause, rollbackErr)
		}
		if len(unrestored) > 0 {
			compensationState = "partially-compensated"
			unrestoredIDs = unrestored
			cause = errors.Join(cause, fmt.Errorf("compensation could not restore: %s", strings.Join(unrestored, ", ")))
		}
	}
	logShippedEventAppendFailure(ctx, shipmentID, "not-applied", compensationState, unrestoredIDs, cause)
	return shippedEventFailureOutcome{
		err: &blerrors.MutationPartialError{
			Completed:         completed,
			FailedStep:        blerrors.StepShippedEventAppend,
			CompensationState: compensationState,
			Class:             "not-applied",
			Cause:             cause,
		},
		rollbackAttempted: true,
	}
}

// logShippedEventAppendFailure emits the fixed, greppable slog record that is
// the only non-MCP measurement surface for the shipped-event SLIs.
// MutationPartialError.Error() does not render CompensationState, so
// compensation_state and unrestored_ids are required attributes here.
func logShippedEventAppendFailure(ctx context.Context, shipmentID, class, compensationState string, unrestoredIDs []string, cause error) {
	slog.ErrorContext(ctx, "shipment shipped-event append failed",
		"shipment_id", shipmentID,
		"class", class,
		"failed_step", blerrors.StepShippedEventAppend,
		"compensation_state", compensationState,
		"unrestored_ids", unrestoredIDs,
		"cause", cause,
	)
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

// archiveItems archives every item in itemIDs, deepest-first, and returns the
// IDs it successfully archived even when a later item fails.
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
	return uniqueNonEmptyStrings(itemIDs), nil
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

func shipmentClaimAncestorIDs(ctx context.Context, ws *Workspace, memberIDs []string) ([]string, error) {
	memberSet := make(map[string]struct{}, len(memberIDs))
	for _, memberID := range memberIDs {
		memberSet[memberID] = struct{}{}
	}
	seen := make(map[string]struct{})
	ancestors := make([]string, 0)
	for _, memberID := range uniqueNonEmptyStrings(memberIDs) {
		member, err := loadArtifact(ctx, ws, memberID)
		if err != nil {
			return nil, fmt.Errorf("load claim member %s hierarchy: %w", memberID, err)
		}
		chain := make(map[string]struct{})
		for ancestorID := member.ParentID; ancestorID != ""; {
			if _, cycle := chain[ancestorID]; cycle {
				return nil, fmt.Errorf("claim member %s hierarchy contains a cycle at %s: %w",
					memberID, ancestorID, blerrors.ErrValidation)
			}
			chain[ancestorID] = struct{}{}
			ancestor, loadErr := loadArtifact(ctx, ws, ancestorID)
			if loadErr != nil {
				return nil, fmt.Errorf("load claim member %s ancestor %s: %w", memberID, ancestorID, loadErr)
			}
			if _, isMember := memberSet[ancestorID]; !isMember {
				if _, exists := seen[ancestorID]; !exists {
					seen[ancestorID] = struct{}{}
					ancestors = append(ancestors, ancestorID)
				}
			}
			ancestorID = ancestor.ParentID
		}
	}
	return ancestors, nil
}

func setArtifactStatus(ctx context.Context, ws *Workspace, itemID string, newStatus models.ArtifactStatus, reason string) (*models.Artifact, error) {
	lockedCtx, globalUnlock, lockErr := lockShipmentLifecycleGlobal(ctx, ws)
	if lockErr != nil {
		return nil, fmt.Errorf("lock shipment lifecycle for status update %s: %w", itemID, lockErr)
	}
	defer func() {
		if unlockErr := globalUnlock(); unlockErr != nil {
			slog.WarnContext(ctx, "release shipment lifecycle lock after status update", "artifact_id", itemID, "error", unlockErr)
		}
	}()
	ctx = lockedCtx

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
	lockedCtx, globalUnlock, lockErr := lockShipmentLifecycleGlobal(ctx, ws)
	if lockErr != nil {
		return fmt.Errorf("lock shipment lifecycle for status cascade %s: %w", itemID, lockErr)
	}
	defer func() {
		if unlockErr := globalUnlock(); unlockErr != nil {
			slog.WarnContext(ctx, "release shipment lifecycle lock after status cascade", "artifact_id", itemID, "error", unlockErr)
		}
	}()
	ctx = lockedCtx

	item, err := loadArtifact(ctx, ws, itemID)
	if err != nil {
		return err
	}
	if item.ParentID == "" {
		return nil
	}
	if boundary, bounded := ctx.Value(shipmentMemberCascadeBoundaryContextKey{}).(map[string]struct{}); bounded {
		if _, isMember := boundary[item.ParentID]; !isMember {
			return nil
		}
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

func lockAdoptionEventLogs(ctx context.Context, ws *Workspace, oldID, newID string) (context.Context, func(), error) {
	ids := []string{oldID, newID}
	sort.Strings(ids)
	unlocks := make([]func(), 0, len(ids))
	lockedCtx := ctx
	for _, id := range ids {
		var unlock func()
		var err error
		lockedCtx, unlock, err = events.LockItemLogCrossProcess(lockedCtx, WorkspaceLocksRoot(ws.RootPath), WorkspaceLogsRoot(ws.RootPath), id)
		if err != nil {
			for i := len(unlocks) - 1; i >= 0; i-- {
				unlocks[i]()
			}
			return ctx, nil, err
		}
		unlocks = append(unlocks, unlock)
	}
	return lockedCtx, func() {
		for i := len(unlocks) - 1; i >= 0; i-- {
			unlocks[i]()
		}
	}, nil
}

// AdoptItem sets an orphaned or unparented item's parent_id to a new feature,
// atomically rewriting its hierarchical ID, renaming files, updating dependency
// and link edges, and syncing the index. The return value includes the new ID
// so callers can update their own references. Adoption rewrites internal
// backlogit references only; external references are the caller's responsibility.
func AdoptItem(ctx context.Context, ws *Workspace, itemID, newParentID string) (*AdoptItemResult, error) {
	lockedCtx, releaseArtifactLocks, lockErr := lockArtifactMutations(ctx, ws, []string{itemID, newParentID})
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

	var releaseEventLocks func()
	if newID != oldID {
		lockedCtx, releaseEventLocks, lockErr = lockAdoptionEventLogs(ctx, ws, oldID, newID)
		if lockErr != nil {
			return nil, fmt.Errorf("adopt item %s: acquire event-log locks: %w", itemID, lockErr)
		}
		defer releaseEventLocks()
		ctx = lockedCtx
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
