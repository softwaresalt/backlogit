package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bldb "github.com/softwaresalt/backlogit/internal/db"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/hooks"
	"github.com/softwaresalt/backlogit/internal/models"
)

// 143.001-T (Unit 1): RED harness for the governed shipped-event durability
// contract. Every scenario here is injected through the per-workspace
// ws.shipmentEventAppend seam, never through the real filesystem:
// snapshotShipArtifacts locks and snapshots the same item log BEFORE the
// active->shipped transition runs, so a planted directory would abort the ship
// earlier and leave shipSnapshots empty, short-circuiting the rollback defer.
//
// These tests MUST NOT call t.Parallel(): ShipShipment reads the package
// globals persistArtifactWriteFn and mkdirDirSyncFn, which sibling tests
// override.
//
// The failed-step name is asserted against the string literal
// "shipped-event-append" rather than blerrors.StepShippedEventAppend, because
// the exported constant lands in Unit 2 (143.002-T) and this harness predates
// it. Unit 2's acceptance criterion is that the constant equals this literal.
const shippedEventAppendStep = "shipped-event-append"

// shipDurabilityFixture is the two-task shipment used by the durability
// scenarios. The covering feature is deliberately NOT an explicit shipment
// member, so shipping both of its only children fires the cascade rollup the
// indeterminate branch must revert in-lock.
type shipDurabilityFixture struct {
	ws         *Workspace
	featureID  string
	taskOneID  string
	taskTwoID  string
	shipmentID string
}

func newShipDurabilityFixture(t *testing.T, durableWrites bool) *shipDurabilityFixture {
	t.Helper()
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	// Pin durability explicitly so classification never depends silently on
	// workspace configuration.
	require.NotNil(t, ws.Config, "fixture workspace must carry a config")
	ws.Config.DurableWrites = durableWrites

	feature, err := CreateArtifact(ctx, ws, "Durability covering feature", "feature")
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, feature))

	taskOne, err := CreateArtifact(ctx, ws, "Durability task one", "task", WithParent(feature.ID))
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, taskOne))

	taskTwo, err := CreateArtifact(ctx, ws, "Durability task two", "task", WithParent(feature.ID))
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, taskTwo))

	shipment, err := CreateShipment(ctx, ws, "Durability shipment", []string{taskOne.ID, taskTwo.ID})
	require.NoError(t, err)
	_, err = ClaimShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)

	return &shipDurabilityFixture{
		ws:         ws,
		featureID:  feature.ID,
		taskOneID:  taskOne.ID,
		taskTwoID:  taskTwo.ID,
		shipmentID: shipment.ID,
	}
}

// injectShippedAppend arms the seam so only the shipment's own
// shipment_status_changed:shipped append fails. Every other append flows
// through the real error-returning path.
func (f *shipDurabilityFixture) injectShippedAppend(t *testing.T, arm func(ctx context.Context) error) {
	t.Helper()
	f.ws.shipmentEventAppend = func(ctx context.Context, ws *Workspace, itemID, eventType string, delta map[string]any) error {
		if itemID == f.shipmentID && eventType == "shipment_status_changed" && delta["status"] == string(ShipmentShipped) {
			return arm(ctx)
		}
		return appendItemEventErr(ctx, ws, itemID, eventType, delta)
	}
}

// shipWithWatchdog runs ShipShipment under an explicit deadline so a lock
// regression surfaces as a test failure rather than as a suite timeout.
func shipWithWatchdog(t *testing.T, ws *Workspace, shipmentID string) (*ShipShipmentResult, error) {
	t.Helper()
	type shipOutcome struct {
		result *ShipShipmentResult
		err    error
	}
	done := make(chan shipOutcome, 1)
	go func() {
		result, err := ShipShipment(context.Background(), ws, shipmentID, nil)
		done <- shipOutcome{result: result, err: err}
	}()
	watchdog := 90 * time.Second
	if deadline, ok := t.Deadline(); ok {
		if remaining := time.Until(deadline) - 5*time.Second; remaining > 0 && remaining < watchdog {
			watchdog = remaining
		}
	}
	select {
	case outcome := <-done:
		return outcome.result, outcome.err
	case <-time.After(watchdog):
		t.Fatalf("ShipShipment did not return within %s: the item-log lock was taken by the harness or the appender dropped its locked context", watchdog)
		return nil, nil
	}
}

func requireShippedAppendPartial(t *testing.T, err error) *blerrors.MutationPartialError {
	t.Helper()
	require.Error(t, err, "the governed ship path must surface the injected shipped-event append failure")
	assert.NotContains(t, err.Error(), "snapshot release scope",
		"the failure must arrive from the shipped-event append, not from snapshotting")
	var partial *blerrors.MutationPartialError
	require.ErrorAs(t, err, &partial, "a shipped-event append failure must be a structured MutationPartialError")
	assert.Equal(t, shippedEventAppendStep, partial.FailedStep)
	return partial
}

func hasShippedEvent(t *testing.T, ws *Workspace, shipmentID string) bool {
	t.Helper()
	logsDir := WorkspaceLogsRoot(ws.RootPath)
	itemEvents, err := events.ReadAllEvents(context.Background(), logsDir, shipmentID)
	require.NoError(t, err)
	for _, event := range itemEvents {
		if event.EventType != "shipment_status_changed" {
			continue
		}
		if status, ok := event.Delta["status"].(string); ok && status == string(ShipmentShipped) {
			return true
		}
	}
	return false
}

// Scenario 1: a PROVEN not-applied append outcome compensates. The lock failure
// raised inside the appender before any writer call, and any writer error
// explicitly tagged blerrors.ErrWriteNotApplied, are the only two outcomes whose
// pre-write status is proven, so they are the only two that may roll back.
func TestShipShipment_ProvenNotAppliedShippedEventCompensates(t *testing.T) {
	fixture := newShipDurabilityFixture(t, false)
	ctx := context.Background()

	injected := fmt.Errorf("open item log: %w: %w", blerrors.ErrWriteNotApplied, errors.New("injected pre-write failure"))
	fixture.injectShippedAppend(t, func(context.Context) error { return injected })

	result, err := shipWithWatchdog(t, fixture.ws, fixture.shipmentID)
	assert.Nil(t, result)

	partial := requireShippedAppendPartial(t, err)
	assert.Equal(t, "not-applied", partial.Class)
	assert.Equal(t, "compensated", partial.CompensationState)
	assert.ErrorIs(t, err, blerrors.ErrWriteNotApplied)

	shipment, loadErr := loadArtifact(ctx, fixture.ws, fixture.shipmentID)
	require.NoError(t, loadErr)
	assert.Equal(t, models.ArtifactStatus(ShipmentActive), shipment.Status,
		"a proven not-applied append must restore the shipment to active")

	for _, id := range []string{fixture.taskOneID, fixture.taskTwoID} {
		item, loadErr := loadArtifact(ctx, fixture.ws, id)
		require.NoError(t, loadErr)
		assert.NotEqual(t, models.StatusDone, item.Status,
			"release scope item %s must not stay completed after compensation", id)
		assert.NotEqual(t, models.StatusArchived, item.Status,
			"release scope item %s must not be archived after compensation", id)
	}

	archiveDir := filepath.Join(WorkspaceStorageRoot(fixture.ws.RootPath), "archive")
	entries, readErr := os.ReadDir(archiveDir)
	if readErr == nil {
		assert.Empty(t, entries, "a compensated refusal must archive nothing")
	}
}

// Scenario 2: an indeterminate or unclassified append outcome NEVER rolls back.
// Indeterminate dominates, so an error carrying both sentinels classifies
// indeterminate. The covering feature is restored in-lock and re-read from disk;
// archival is halted and the shipment is deliberately left shipped-and-unarchived.
func TestShipShipment_IndeterminateShippedEventNeverRollsBack(t *testing.T) {
	cases := []struct {
		name     string
		injected error
	}{
		{
			name:     "tagged_indeterminate",
			injected: fmt.Errorf("append shipped event: %w: %w", blerrors.ErrWriteIndeterminate, errors.New("injected fsync failure")),
		},
		{
			name:     "untagged_unproven",
			injected: errors.New("injected untagged append failure"),
		},
		{
			name:     "both_sentinels_indeterminate_dominates",
			injected: fmt.Errorf("append shipped event: %w: %w", blerrors.ErrWriteIndeterminate, blerrors.ErrWriteNotApplied),
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newShipDurabilityFixture(t, false)
			ctx := context.Background()

			preShipFeature, loadErr := loadArtifact(ctx, fixture.ws, fixture.featureID)
			require.NoError(t, loadErr)
			preShipFeatureStatus := preShipFeature.Status

			fixture.injectShippedAppend(t, func(context.Context) error { return testCase.injected })

			result, err := shipWithWatchdog(t, fixture.ws, fixture.shipmentID)
			assert.Nil(t, result)

			partial := requireShippedAppendPartial(t, err)
			assert.Equal(t, "indeterminate", partial.Class)
			assert.Equal(t, "not-compensated", partial.CompensationState)

			shipment, loadErr := loadArtifact(ctx, fixture.ws, fixture.shipmentID)
			require.NoError(t, loadErr)
			assert.Equal(t, models.StatusShipped, shipment.Status,
				"an indeterminate append must leave the shipment shipped, not roll it back")
			assert.Empty(t, shipment.ArchivedStatus,
				"an indeterminate append must halt archival, leaving shipped-and-unarchived residue")

			for _, id := range []string{fixture.taskOneID, fixture.taskTwoID} {
				item, loadErr := loadArtifact(ctx, fixture.ws, id)
				require.NoError(t, loadErr)
				assert.Equal(t, models.StatusDone, item.Status,
					"release scope item %s must NOT be rolled back on an indeterminate outcome", id)
			}

			// Re-read from disk rather than trusting an in-memory value: the
			// non-member covering feature must be restored synchronously, while
			// the artifact locks are still held.
			restoredFeature, loadErr := loadArtifact(ctx, fixture.ws, fixture.featureID)
			require.NoError(t, loadErr)
			assert.Equal(t, preShipFeatureStatus, restoredFeature.Status,
				"non-member covering feature must be restored to its pre-ship status on the indeterminate branch")
		})
	}
}

// Scenario 3: compensation reports itself when it cannot complete. A proven
// not-applied append plus one release-scope item whose log lock cannot be
// re-acquired must surface partially-compensated naming the un-restored ID,
// never a silently skipped item.
//
// Contention is armed from INSIDE the seam callback by planting a directory at
// the item's lock sidecar path, which fails openItemLogLockHandle immediately.
// The harness must never call events.LockItemLog or LockItemLogCrossProcess to
// create it: holding the in-process mutex would block restoreShipArtifacts on an
// uncancellable, deadline-free mutex.Lock and hang the test binary.
func TestShipShipment_UnrestorableItemReportsPartialCompensation(t *testing.T) {
	fixture := newShipDurabilityFixture(t, false)

	logsDir := WorkspaceLogsRoot(fixture.ws.RootPath)
	blockedID := fixture.taskOneID
	sidecar := filepath.Join(logsDir, "."+blockedID+".jsonl.lock")

	injected := fmt.Errorf("open item log: %w: %w", blerrors.ErrWriteNotApplied, errors.New("injected pre-write failure"))
	fixture.injectShippedAppend(t, func(context.Context) error {
		// snapshotShipArtifacts has already taken and released its own locks by
		// the time the seam runs, so planting the directory here cannot abort
		// the ship before the rollback map is populated.
		if err := os.RemoveAll(sidecar); err != nil {
			return fmt.Errorf("clear lock sidecar: %w", err)
		}
		if err := os.MkdirAll(sidecar, 0o755); err != nil {
			return fmt.Errorf("plant lock sidecar directory: %w", err)
		}
		return injected
	})
	t.Cleanup(func() { _ = os.RemoveAll(sidecar) })

	result, err := shipWithWatchdog(t, fixture.ws, fixture.shipmentID)
	assert.Nil(t, result)

	partial := requireShippedAppendPartial(t, err)
	assert.Equal(t, "not-applied", partial.Class)
	assert.Equal(t, "partially-compensated", partial.CompensationState,
		"an un-restorable release-scope item must degrade the compensation state, never be skipped silently")
	assert.True(t, strings.Contains(err.Error(), blockedID),
		"the un-restored ID %q must be named in the returned error, got: %v", blockedID, err)
}

// 143.003-T (Unit 3): the fail-closed governed shipped path intentionally
// suppresses the HookMoveShipmentStatus POST hook, which today always fires
// because the append error is swallowed. Firing a "status changed to shipped"
// post-hook for a transition that is about to be compensated (or reported as
// indeterminate) would misinform external integrations. The suppression is
// asserted with a recording hook runner rather than left to the doc comment.
func TestShipShipment_FailClosedShippedAppendSuppressesMoveStatusPostHook(t *testing.T) {
	fixture := newShipDurabilityFixture(t, false)
	fixture.ws.HookRunner = hooks.NewHookRunner()

	movePostHookFired := false
	fixture.ws.HookRunner.Register(hooks.HookMoveShipmentStatus, hooks.PhasePost, hooks.HookRegistration{
		Name:     "record_move_shipment_status_post",
		Priority: 100,
		Fn: func(_ context.Context, hc hooks.HookContext) error {
			if hc.ItemID == fixture.shipmentID && hc.NewValues["status"] == string(ShipmentShipped) {
				movePostHookFired = true
			}
			return nil
		},
	})

	injected := fmt.Errorf("append shipped event: %w: %w", blerrors.ErrWriteIndeterminate, errors.New("injected fsync failure"))
	fixture.injectShippedAppend(t, func(context.Context) error { return injected })

	_, err := shipWithWatchdog(t, fixture.ws, fixture.shipmentID)
	require.Error(t, err)
	assert.False(t, movePostHookFired,
		"the move-shipment-status post hook must not fire for a shipped transition whose audit append failed")
}
