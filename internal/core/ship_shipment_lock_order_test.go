package core

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/events"
)

// 200.004-T (CY-B B4): lock-order probe for ShipShipment.
//
// The canonical order is lifecycle global, then A, then C (item log, sorted by
// item ID), then B (batch artifact-mutation locks). ShipShipment holds the batch
// B locks for its release scope through the archive loop, so any item-log lock C
// it acquires while those B locks are held is a B-then-C overlap.
//
// The probe uses only the existing test-only barriers. The item-log barrier
// (events.SetItemLogLockBarrierHookForTest) fires immediately before the
// cross-process item-log file lock C is acquired. At that instant the probe
// try-locks the B lock of every ship-scope artifact with lockTaskFile, which
// returns ErrTaskBusy without blocking when ShipShipment's own goroutine already
// holds it. A busy B lock at a C acquisition is a B-held-while-C-acquired
// violation. A C that the caller already holds in its context does not fire the
// barrier, so a ship that takes C for its whole archive set before B reports no
// violation. No production hook is added. `go test -race` alone is not proof of
// lock ordering, which is why the barriers force the observation.
//
// This test installs package-global hooks and therefore must not call t.Parallel.
func TestShipShipment_NeverAcquiresItemLogLockWhileBatchArtifactLocksHeld(t *testing.T) {
	fixture := newShipDurabilityFixture(t, false)
	ws := fixture.ws
	scope := []string{fixture.shipmentID, fixture.taskOneID, fixture.taskTwoID, fixture.featureID}

	var (
		mu         sync.Mutex
		violations = map[string]struct{}{}
		probeErrs  []error
		cRequested = map[string]struct{}{}
	)
	restoreItemLogHook := events.SetItemLogLockBarrierHookForTest(func(itemID string) {
		mu.Lock()
		defer mu.Unlock()
		cRequested[itemID] = struct{}{}
		for _, artifactID := range scope {
			stableKey, pathErr := artifactMutationLockPath(ws, artifactID)
			if pathErr != nil {
				probeErrs = append(probeErrs, pathErr)
				continue
			}
			unlock, lockErr := lockTaskFile(stableKey)
			switch {
			case lockErr == nil:
				if unlockErr := unlock(); unlockErr != nil {
					probeErrs = append(probeErrs, unlockErr)
				}
			case errors.Is(lockErr, ErrTaskBusy):
				violations[fmt.Sprintf("item-log lock C for %s acquired while batch lock B for %s is held", itemID, artifactID)] = struct{}{}
			default:
				probeErrs = append(probeErrs, lockErr)
			}
		}
	})
	t.Cleanup(restoreItemLogHook)

	result, err := shipWithWatchdog(t, ws, fixture.shipmentID)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Contains(t, result.ArchivedIDs, fixture.shipmentID)

	mu.Lock()
	defer mu.Unlock()
	require.NoError(t, errors.Join(probeErrs...), "the B-held probe itself must not fail")
	require.Empty(t, violations,
		"ShipShipment must acquire C for its archive set before B and never request a new C while B is held")

	// Non-vacuity: the probe only means something if the barrier fired, so the
	// ship's own C set must have been requested through it.
	require.NotEmpty(t, cRequested, "the item-log barrier must fire for the ship's C set, or the probe observed nothing")
	// The feature is a NON-member parent of both member tasks. The bounded
	// parent cascade stops at it (cascadePersistedParentStatuses), so ShipShipment
	// never appends events for it and never needs its C.
	require.NotContains(t, cRequested, fixture.featureID,
		"a non-member parent must not be touched by the ship's parent cascade")
}

// TestShipShipmentLockOrderProbe_BDetectionIsInProcess proves the B probe above
// cannot pass vacuously: lockTaskFile reports ErrTaskBusy for a held B through
// its in-process per-path mutex (TryLock), which is platform independent, before
// any OS-level flock/LockFileEx is consulted.
func TestShipShipmentLockOrderProbe_BDetectionIsInProcess(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	item, err := CreateArtifact(t.Context(), ws, "probe control", "feature")
	require.NoError(t, err)

	_, release, err := lockArtifactMutations(t.Context(), ws, []string{item.ID})
	require.NoError(t, err)
	stableKey, err := artifactMutationLockPath(ws, item.ID)
	require.NoError(t, err)

	_, busyErr := lockTaskFile(stableKey)
	require.ErrorIs(t, busyErr, ErrTaskBusy, "a held B must read as busy to the probe")

	require.NoError(t, release())
	unlock, freeErr := lockTaskFile(stableKey)
	require.NoError(t, freeErr, "a released B must read as free to the probe")
	require.NoError(t, unlock())
}
