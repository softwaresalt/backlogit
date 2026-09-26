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

	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core/gate"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

type shipmentLockOrderDiagnostic interface {
	error
	LockID() string
	Caller() string
	HeldLocks() []string
}

func TestShipmentLifecycleLockOrder_DiagnosticAndReentrantBarrier(t *testing.T) {
	t.Run("out_of_order_context_returns_structured_diagnostic", func(t *testing.T) {
		ws := setupShipmentWorkspace(t)
		ctx := withArtifactMutationLocks(context.Background(), []string{"artifact-z", "artifact-a"})
		ctx = context.WithValue(ctx, shipmentLifecycleGlobalLockContextKey{}, struct{}{})

		_, unlock, err := lockShipmentLifecycleGlobal(ctx, ws)
		if unlock != nil {
			t.Cleanup(func() { require.NoError(t, unlock()) })
		}

		require.Error(t, err)
		var diagnostic shipmentLockOrderDiagnostic
		require.ErrorAs(t, err, &diagnostic)
		require.Equal(t, shipmentLifecycleGlobalLockID, diagnostic.LockID())
		require.Contains(t, diagnostic.Caller(), "TestShipmentLifecycleLockOrder")
		require.Equal(t, []string{
			"artifact-mutation:artifact-a",
			"artifact-mutation:artifact-z",
		}, diagnostic.HeldLocks())
		require.Contains(t, err.Error(), shipmentLifecycleGlobalLockID)
		require.Contains(t, err.Error(), diagnostic.Caller())
		for _, held := range diagnostic.HeldLocks() {
			require.Contains(t, err.Error(), held)
		}
	})

	t.Run("missing_global_token_returns_structured_diagnostic_without_waiting", func(t *testing.T) {
		ws := setupShipmentWorkspace(t)
		ctx := withArtifactMutationLocks(context.Background(), []string{"artifact-b", "artifact-a"})

		_, unlock, err := lockShipmentLifecycleGlobal(ctx, ws)
		if unlock != nil {
			t.Cleanup(func() { require.NoError(t, unlock()) })
		}

		require.Error(t, err)
		var diagnostic shipmentLockOrderDiagnostic
		require.ErrorAs(t, err, &diagnostic)
		require.Equal(t, shipmentLifecycleGlobalLockID, diagnostic.LockID())
		require.Equal(t, []string{
			"artifact-mutation:artifact-a",
			"artifact-mutation:artifact-b",
		}, diagnostic.HeldLocks())
	})

	t.Run("verified_reentry_is_a_no_op", func(t *testing.T) {
		ws := setupShipmentWorkspace(t)
		attempts := make(chan string, 4)
		ctx := context.WithValue(context.Background(), shipmentLifecycleGlobalLockHookContextKey{}, func(phase string) {
			attempts <- phase
		})

		lockedCtx, unlock, err := lockShipmentLifecycleGlobal(ctx, ws)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, unlock()) })
		require.Equal(t, "attempt", <-attempts)
		require.Equal(t, "acquired", <-attempts)

		reenteredCtx, reentryUnlock, err := lockShipmentLifecycleGlobal(lockedCtx, ws)
		require.NoError(t, err)
		require.Same(t, lockedCtx, reenteredCtx)
		require.NoError(t, reentryUnlock())
		select {
		case phase := <-attempts:
			t.Fatalf("verified re-entry attempted physical lock acquisition: %s", phase)
		default:
		}
	})
}

type shipmentLockOrderGateRunner struct {
	globalAcquired <-chan struct{}
}

func (r shipmentLockOrderGateRunner) Run(
	_ context.Context,
	_ []string,
	_ string,
	_ []string,
) (gate.GateResult, error) {
	select {
	case <-r.globalAcquired:
		return gate.GateResult{ExitCode: 0, Stdout: []byte(`{}`)}, nil
	default:
		return gate.GateResult{}, errors.New("gate artifact lock was entered before shipment lifecycle global lock")
	}
}

func TestCheckChildrenTerminal_GatedWriterUsesGlobalThenArtifactOrder(t *testing.T) {
	ws := newGateTestWorkspace(t)
	ctx := context.Background()
	parent, err := CreateArtifact(ctx, ws, "lock-order parent", "feature")
	require.NoError(t, err)
	child, err := CreateArtifact(ctx, ws, "lock-order child", "task", WithParent(parent.ID))
	require.NoError(t, err)
	_, err = UpdateArtifact(ctx, ws, child.ID, map[string]any{"status": "active"})
	require.NoError(t, err)
	require.ErrorIs(t, CheckChildrenTerminal(ctx, ws.DB, parent.ID), blerrors.ErrChildrenNotTerminal)

	globalAcquired := make(chan struct{})
	hook := func(phase string) {
		if phase == "acquired" {
			close(globalAcquired)
		}
	}
	observedCtx := context.WithValue(ctx, shipmentLifecycleGlobalLockHookContextKey{}, hook)
	injectBroker(
		ws,
		gate.EnabledAuto,
		shipmentLockOrderGateRunner{globalAcquired: globalAcquired},
		fakeVersion{v: okVersion},
	)

	_, _, err = UpdateArtifactWithGate(
		observedCtx,
		ws,
		child.ID,
		map[string]any{"status": "done"},
		TransitionOptions{},
	)

	require.NoError(t, err)
	require.NoError(t, CheckChildrenTerminal(ctx, ws.DB, parent.ID))
}

func TestP021ClaimSerialization_RepeatedMembershipAndGenericWritersContend(t *testing.T) {
	const iterations = 8
	for iteration := 0; iteration < iterations; iteration++ {
		t.Run(fmt.Sprintf("iteration_%02d", iteration), func(t *testing.T) {
			ws := setupShipmentWorkspace(t)
			ctx := context.Background()
			feature, err := CreateArtifact(ctx, ws, "repeated contention feature", "feature")
			require.NoError(t, err)
			member, err := CreateArtifact(ctx, ws, "repeated contention member", "task", WithParent(feature.ID))
			require.NoError(t, err)
			shipment, err := CreateShipment(ctx, ws, "repeated contention shipment", nil)
			require.NoError(t, err)

			unlock, err := lockShipmentMembership(ctx, ws, shipmentLifecycleGlobalLockID)
			require.NoError(t, err)
			locked := true
			t.Cleanup(func() {
				if locked {
					require.NoError(t, unlock())
				}
			})

			membershipCtx, membershipAttempted, membershipAcquired := p021ObserveGlobalLock(ctx)
			genericCtx, genericAttempted, genericAcquired := p021ObserveGlobalLock(ctx)
			membershipResult := make(chan error, 1)
			genericResult := make(chan error, 1)
			go func() {
				membershipResult <- AddItemToShipment(membershipCtx, ws, shipment.ID, member.ID)
			}()
			go func() {
				_, updateErr := UpdateArtifact(genericCtx, ws, shipment.ID, map[string]any{
					"title": fmt.Sprintf("generic writer iteration %d", iteration),
				})
				genericResult <- updateErr
			}()

			<-membershipAttempted
			<-genericAttempted
			for name, acquired := range map[string]<-chan struct{}{
				"membership": membershipAcquired,
				"generic":    genericAcquired,
			} {
				select {
				case <-acquired:
					t.Fatalf("%s writer acquired the held global lock", name)
				default:
				}
			}

			require.NoError(t, unlock())
			locked = false
			for name, result := range map[string]<-chan error{
				"membership": membershipResult,
				"generic":    genericResult,
			} {
				select {
				case writerErr := <-result:
					require.NoError(t, writerErr, "%s writer failed", name)
				case <-time.After(defaultGateLockBoundedWait + 2*time.Second):
					t.Fatalf("%s writer did not finish after global lock release", name)
				}
			}
			require.Equal(t, []string{member.ID}, NormalizeShipmentItems(loadURCanonicalArtifact(t, ws, shipment.ID)))
			require.True(t, strings.HasPrefix(loadURCanonicalArtifact(t, ws, shipment.ID).Title, "generic writer iteration"))
		})
	}
}

func TestShipmentLifecycleBarrier_SingularTypedMemberWriterWaits(t *testing.T) {
	t.Run("non_shipment_member_waits", func(t *testing.T) {
		ws := setupShipmentWorkspace(t)
		ctx := context.Background()
		feature, err := CreateArtifact(ctx, ws, "singular member barrier feature", "feature")
		require.NoError(t, err)
		member, err := CreateArtifact(ctx, ws, "singular member barrier task", "task", WithParent(feature.ID))
		require.NoError(t, err)
		dependency, err := CreateArtifact(ctx, ws, "singular member barrier dependency", "task", WithParent(feature.ID))
		require.NoError(t, err)
		_, err = CreateShipment(ctx, ws, "singular member barrier shipment", []string{member.ID})
		require.NoError(t, err)
		require.NotEqual(t, "shipment", member.ArtifactType,
			"the regression must exercise a non-shipment lifecycle member")

		unlockGlobal, err := lockShipmentMembership(ctx, ws, shipmentLifecycleGlobalLockID)
		require.NoError(t, err)
		globalLocked := true
		t.Cleanup(func() {
			if globalLocked {
				require.NoError(t, unlockGlobal())
			}
		})

		observedCtx, attempted, acquired := p021ObserveGlobalLock(ctx)
		result := make(chan error, 1)
		go func() {
			result <- AddDependency(observedCtx, ws, member.ID, dependency.ID, "blocks")
		}()

		attemptedBarrier := false
		completedWhileHeld := false
		var writerErr error
		select {
		case <-attempted:
			attemptedBarrier = true
		case writerErr = <-result:
			completedWhileHeld = true
		case <-time.After(time.Second):
		}
		if attemptedBarrier {
			select {
			case <-acquired:
				t.Error("singular member writer acquired the held lifecycle-global barrier")
			case writerErr = <-result:
				completedWhileHeld = true
			case <-time.After(150 * time.Millisecond):
			}
		}

		require.NoError(t, unlockGlobal())
		globalLocked = false
		if !completedWhileHeld {
			select {
			case writerErr = <-result:
			case <-time.After(defaultGateLockBoundedWait + 2*time.Second):
				t.Fatal("singular member writer did not finish after the lifecycle-global barrier was released")
			}
		}

		require.True(t, attemptedBarrier,
			"singular writer for a non-shipment lifecycle member bypassed the global barrier")
		require.False(t, completedWhileHeld,
			"singular writer for a non-shipment lifecycle member completed while the global barrier was held")
		require.NoError(t, writerErr)
		select {
		case <-acquired:
		default:
			t.Fatal("singular member writer never acquired the released lifecycle-global barrier")
		}
	})

	t.Run("standalone_non_member_remains_unbarriered", func(t *testing.T) {
		ws := setupShipmentWorkspace(t)
		ctx := context.Background()
		feature, err := CreateArtifact(ctx, ws, "standalone barrier feature", "feature")
		require.NoError(t, err)
		standalone, err := CreateArtifact(ctx, ws, "standalone barrier task", "task", WithParent(feature.ID))
		require.NoError(t, err)
		dependency, err := CreateArtifact(ctx, ws, "standalone barrier dependency", "task", WithParent(feature.ID))
		require.NoError(t, err)

		unlockGlobal, err := lockShipmentMembership(ctx, ws, shipmentLifecycleGlobalLockID)
		require.NoError(t, err)
		globalLocked := true
		t.Cleanup(func() {
			if globalLocked {
				require.NoError(t, unlockGlobal())
			}
		})

		observedCtx, attempted, _ := p021ObserveGlobalLock(ctx)
		result := make(chan error, 1)
		go func() {
			result <- AddDependency(observedCtx, ws, standalone.ID, dependency.ID, "blocks")
		}()

		barrierAttempted := false
		completedWhileHeld := false
		timedOut := false
		var writerErr error
		select {
		case writerErr = <-result:
			completedWhileHeld = true
		case <-attempted:
			barrierAttempted = true
		case <-time.After(2 * time.Second):
			timedOut = true
		}

		require.NoError(t, unlockGlobal())
		globalLocked = false
		if !completedWhileHeld {
			select {
			case writerErr = <-result:
			case <-time.After(defaultGateLockBoundedWait + 2*time.Second):
				t.Fatal("standalone non-member writer did not converge after the global barrier was released")
			}
		}

		require.False(t, barrierAttempted,
			"standalone non-member writer was routed through the lifecycle-global barrier")
		require.False(t, timedOut,
			"standalone non-member writer did not complete while the unrelated global barrier was held")
		require.True(t, completedWhileHeld,
			"standalone non-member writer waited on the unrelated global barrier")
		require.NoError(t, writerErr)
	})
}

func TestShipmentLifecycleLockOrder_ReconcileAndAddUseGlobalFirst(t *testing.T) {
	ws, shipmentID, _ := u20ReconcileFixture(t)
	ctx := context.Background()
	feature, err := CreateArtifact(ctx, ws, "reconcile add contention feature", "feature")
	require.NoError(t, err)
	candidate, err := CreateArtifact(ctx, ws, "reconcile add contention candidate", "task", WithParent(feature.ID))
	require.NoError(t, err)

	_, releaseItemLogGate, err := lockShipmentReconcileItemLog(ctx, ws, shipmentID)
	require.NoError(t, err)
	itemLogGateHeld := true
	t.Cleanup(func() {
		if itemLogGateHeld {
			require.NoError(t, releaseItemLogGate())
		}
	})

	reconcileCtx, _, reconcileGlobalAcquired := p021ObserveGlobalLock(ctx)
	reconcileResult := make(chan error, 1)
	go func() {
		_, reconcileErr := ReconcileShipmentToShipped(
			reconcileCtx,
			ws,
			validShipmentReconcilePreconditionRequest(shipmentID),
		)
		reconcileResult <- reconcileErr
	}()

	membershipSidecar := taskLockSidecarPath(filepath.Join(
		WorkspaceStorageRoot(ws.RootPath),
		shipmentMembershipLocksDirName,
		shipmentID,
	))
	membershipHeld := false
	membershipDeadline := time.Now().Add(time.Second)
	for time.Now().Before(membershipDeadline) {
		if _, statErr := os.Stat(membershipSidecar); statErr == nil {
			membershipHeld = true
			break
		} else if !errors.Is(statErr, os.ErrNotExist) {
			require.NoError(t, statErr)
		}
		time.Sleep(5 * time.Millisecond)
	}

	addCtx, addGlobalAttempted, addGlobalAcquired := p021ObserveGlobalLock(ctx)
	addResult := make(chan error, 1)
	go func() {
		addResult <- AddItemToShipment(addCtx, ws, shipmentID, candidate.ID)
	}()

	addAttempted := false
	select {
	case <-addGlobalAttempted:
		addAttempted = true
	case <-time.After(time.Second):
	}

	addAcquiredBeforeRelease := false
	select {
	case <-addGlobalAcquired:
		addAcquiredBeforeRelease = true
	case <-time.After(150 * time.Millisecond):
	}
	reconcileAcquiredBeforeRelease := false
	select {
	case <-reconcileGlobalAcquired:
		reconcileAcquiredBeforeRelease = true
	default:
	}

	reconcileCompletedWhileGated := false
	var reconcileErr error
	select {
	case reconcileErr = <-reconcileResult:
		reconcileCompletedWhileGated = true
	default:
	}
	addCompletedWhileGated := false
	var addErr error
	select {
	case addErr = <-addResult:
		addCompletedWhileGated = true
	default:
	}

	require.NoError(t, releaseItemLogGate())
	itemLogGateHeld = false
	if !reconcileCompletedWhileGated {
		select {
		case reconcileErr = <-reconcileResult:
		case <-time.After(defaultGateLockBoundedWait + 5*time.Second):
			t.Fatal("reconcile did not converge after the item-log gate was released")
		}
	}
	if !addCompletedWhileGated {
		select {
		case addErr = <-addResult:
		case <-time.After(defaultGateLockBoundedWait + 5*time.Second):
			t.Fatal("add did not converge after the item-log gate was released")
		}
	}

	require.True(t, membershipHeld,
		"reconcile never reached its membership lock before the item-log gate")
	require.True(t, addAttempted,
		"add never attempted the lifecycle-global lock while reconcile was gated")
	require.True(t, reconcileAcquiredBeforeRelease,
		"reconcile reached membership/item-log locking before lifecycle-global")
	require.False(t, addAcquiredBeforeRelease,
		"add acquired lifecycle-global while reconcile held membership, reproducing the inverse-order cycle")
	require.False(t, reconcileCompletedWhileGated,
		"reconcile unexpectedly bypassed the held item-log gate")
	require.False(t, addCompletedWhileGated,
		"add unexpectedly completed while reconcile held the membership layer")
	require.NotErrorIs(t, reconcileErr, blerrors.ErrGateInProgress)
	require.NotErrorIs(t, reconcileErr, ErrShipmentReconcileLockBusy)
	require.NotErrorIs(t, addErr, blerrors.ErrGateInProgress)
	require.NotErrorIs(t, addErr, ErrShipmentReconcileLockBusy)
	select {
	case <-addGlobalAcquired:
	default:
		t.Fatal("add never acquired lifecycle-global after reconcile released it")
	}
}
