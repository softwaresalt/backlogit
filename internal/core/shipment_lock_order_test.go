package core

import (
	"context"
	"errors"
	"fmt"
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
