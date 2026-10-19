package core

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// 200.001-T (CY-B B1): characterization of ReconcileShipmentToShipped running
// against a concurrent membership writer (AddItemToShipment, ReturnBlockedItem).
//
// The interleaving is forced, not left to the scheduler. The existing test-only
// barrier artifactMutationLockBarrierHook fires immediately after the batch
// artifact-mutation lock (B) is acquired, so the reconcile goroutine is paused
// while it holds the lifecycle global lock, the shipment membership lock (A),
// the item-log lock (C) and B. The membership writer is then started with the
// existing shipmentLifecycleGlobalLockHookContextKey context hook, which reports
// "contended" when the writer's lifecycle-global acquisition finds the lock
// busy. That signal is the deterministic proof the writer is excluded while the
// closure runs. `go test -race` alone is not proof of lock ordering.
//
// This test installs a package-global hook and therefore must not call
// t.Parallel.

const interleaveWait = 30 * time.Second

// reconcileInterleaveObservation records what the forced interleaving saw.
type reconcileInterleaveObservation struct {
	// membershipAtPause is the manifest while reconcile held its locks.
	membershipAtPause []string
	// membershipAtReconcileReturn is the manifest at the instant reconcile
	// returned, before the membership writer was allowed to finish.
	membershipAtReconcileReturn []string
	// writerContended is true when the writer's lifecycle-global acquisition
	// reported contention while reconcile was paused holding its locks.
	writerContended bool
	// reconcileSeq and writerSeq order the two completions.
	reconcileSeq int64
	writerSeq    int64
	reconcileErr error
	writerErr    error
}

type reconcileMembershipWriter func(ctx context.Context, ws *Workspace, shipmentID, memberID, extraID string) error

func observeReconcileMembershipInterleave(t *testing.T, writer reconcileMembershipWriter) reconcileInterleaveObservation {
	t.Helper()
	ws, shipmentID, memberID := u20ReconcileFixture(t)
	extra, err := CreateArtifact(context.Background(), ws, "membership interleave extra", "feature")
	require.NoError(t, err)

	readMembership := func() []string {
		shipment, readErr := findArtifact(context.Background(), ws, shipmentID)
		require.NoError(t, readErr)
		return NormalizeShipmentItems(shipment)
	}

	paused := make(chan struct{})
	resume := make(chan struct{})
	var pauseOnce, resumeOnce sync.Once
	closeResume := func() { resumeOnce.Do(func() { close(resume) }) }

	previousHook := artifactMutationLockBarrierHook
	artifactMutationLockBarrierHook = func(string) {
		pauseOnce.Do(func() {
			close(paused)
			select {
			case <-resume:
			case <-time.After(interleaveWait):
			}
		})
	}

	var seq atomic.Int64
	reconcileDone := make(chan struct{})
	writerDone := make(chan struct{})
	var reconcileStarted, writerStarted bool
	// Registered after nothing else so it runs first on an early t.Fatal: it
	// releases the paused reconcile, drains both goroutines, and only then
	// restores the package-global hook they read.
	t.Cleanup(func() {
		closeResume()
		if reconcileStarted {
			select {
			case <-reconcileDone:
			case <-time.After(interleaveWait):
				t.Errorf("reconcile goroutine did not exit during cleanup")
			}
		}
		if writerStarted {
			select {
			case <-writerDone:
			case <-time.After(interleaveWait):
				t.Errorf("membership writer goroutine did not exit during cleanup")
			}
		}
		artifactMutationLockBarrierHook = previousHook
	})

	var obs reconcileInterleaveObservation
	req := validShipmentReconcilePreconditionRequest(shipmentID)
	reconcileStarted = true
	go func() {
		defer close(reconcileDone)
		_, obs.reconcileErr = ReconcileShipmentToShipped(context.Background(), ws, req)
		obs.reconcileSeq = seq.Add(1)
		obs.membershipAtReconcileReturn = readMembership()
	}()

	select {
	case <-paused:
	case <-time.After(interleaveWait):
		t.Fatal("reconcile never reached artifact-mutation lock B")
	}
	obs.membershipAtPause = readMembership()

	contended := make(chan struct{}, 1)
	writerCtx := context.WithValue(context.Background(), shipmentLifecycleGlobalLockHookContextKey{}, func(phase string) {
		if phase == "contended" {
			select {
			case contended <- struct{}{}:
			default:
			}
		}
	})
	writerStarted = true
	go func() {
		defer close(writerDone)
		obs.writerErr = writer(writerCtx, ws, shipmentID, memberID, extra.ID)
		obs.writerSeq = seq.Add(1)
	}()

	select {
	case <-contended:
		obs.writerContended = true
	case <-time.After(interleaveWait):
	}
	closeResume()

	for name, done := range map[string]chan struct{}{"reconcile": reconcileDone, "writer": writerDone} {
		select {
		case <-done:
		case <-time.After(interleaveWait):
			t.Fatalf("%s did not finish after reconcile was resumed", name)
		}
	}
	return obs
}

var reconcileMembershipWriters = map[string]reconcileMembershipWriter{
	"add_to_shipment": func(ctx context.Context, ws *Workspace, shipmentID, _, extraID string) error {
		return AddItemToShipment(ctx, ws, shipmentID, extraID)
	},
	"membership_removal": func(ctx context.Context, ws *Workspace, shipmentID, memberID, _ string) error {
		return ReturnBlockedItem(ctx, ws, shipmentID, memberID, "membership interleave probe")
	},
}

// TestReconcileMembershipInterleave_CharacterizesCurrentBehavior pins today's
// behavior: ReconcileShipmentToShipped takes the lifecycle global lock and the
// shipment membership lock (A) in Phase A and holds both across every phase,
// so a membership writer started mid-closure is excluded at the lifecycle
// global lock (the same lock every membership writer takes first) and cannot
// touch the manifest until the closure returns.
func TestReconcileMembershipInterleave_CharacterizesCurrentBehavior(t *testing.T) {
	for name, writer := range reconcileMembershipWriters {
		t.Run(name, func(t *testing.T) {
			obs := observeReconcileMembershipInterleave(t, writer)

			require.True(t, obs.writerContended,
				"membership writer must contend on the lifecycle global lock while reconcile holds it")
			require.NotEmpty(t, obs.membershipAtPause)
			require.Equal(t, obs.membershipAtPause, obs.membershipAtReconcileReturn,
				"manifest must not change while the reconcile closure is in flight")
			if obs.reconcileErr != nil {
				// The fixture's synthetic merge SHA fails Phase C evidence
				// verification; that is a recognized reconcile-domain outcome,
				// never a lock error or a member-set drift error.
				require.ErrorIs(t, obs.reconcileErr, blerrors.ErrShipmentReconcileEvidence)
			}
		})
	}
}

// TestReconcileMembershipInterleave_TargetClosureObservesConsistentSnapshot
// states the TARGET: the closure observes a consistent membership snapshot,
// i.e. a membership writer strictly follows the whole closure and the manifest
// the closure froze in Phase A is the manifest it finishes with.
//
// Skipped until B3 (200.003-T), which either binds reconciliation to the
// membership lock domain or records, with B1's evidence, that it already is.
func TestReconcileMembershipInterleave_TargetClosureObservesConsistentSnapshot(t *testing.T) {
	t.Skip("target assertion is enabled by CY-B B3 (200.003-T)")
	for name, writer := range reconcileMembershipWriters {
		t.Run(name, func(t *testing.T) {
			obs := observeReconcileMembershipInterleave(t, writer)

			require.True(t, obs.writerContended, "writer must be excluded while the closure runs")
			require.Equal(t, obs.membershipAtPause, obs.membershipAtReconcileReturn,
				"closure must finish with the membership snapshot it started with")
			require.Less(t, obs.reconcileSeq, obs.writerSeq,
				"membership writer must complete strictly after the closure")
		})
	}
}
