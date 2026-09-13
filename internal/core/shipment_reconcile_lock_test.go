package core

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/events"
)

// 167.011-T behavior harness. lockShipmentReconcileItemLog is the
// reconcile-transaction's own handle-relative acquirer for the item-log
// lock (C): it MUST target the IDENTICAL canonical sidecar resource that
// events.LockItemLogCrossProcess uses (see events.ItemLogLockPath,
// 167.017-T) so the two mutually exclude real writers (AssociateCommit,
// ArchiveItem, LinkCommit), while opening that sidecar in a
// directory-handle-relative, no-follow/reparse-safe way rather than by a
// freshly recomputed pathname.

func TestLockShipmentReconcileItemLog_MutuallyExcludesItemLogCrossProcess(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	itemID := "167.011-T-item"

	_, unlockReconcile, err := lockShipmentReconcileItemLog(context.Background(), ws, itemID)
	require.NoError(t, err)
	require.NotNil(t, unlockReconcile)

	logsDir := WorkspaceLogsRoot(ws.RootPath)
	locksRoot := WorkspaceLocksRoot(ws.RootPath)

	// A second acquirer using the EXISTING cross-process lock primitive
	// (already migrated to the stable identity by 167.017-T) must observe
	// the SAME resource as busy — proving these are the same underlying
	// lock, not two independent ones that provide no real mutual exclusion.
	_, _, lockErr := events.LockItemLogCrossProcess(context.Background(), locksRoot, logsDir, itemID)
	require.Error(t, lockErr, "the reconcile lock and events.LockItemLogCrossProcess must contend on the same resource")

	require.NoError(t, unlockReconcile())

	// Once released, the standard cross-process lock must succeed immediately.
	_, unlockCrossProcess, lockErr := events.LockItemLogCrossProcess(context.Background(), locksRoot, logsDir, itemID)
	require.NoError(t, lockErr)
	unlockCrossProcess()
}

func TestLockShipmentReconcileItemLog_ExcludesAcrossLogsDirSwap(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	itemID := "167.011-T-swap-item"
	locksRoot := WorkspaceLocksRoot(ws.RootPath)

	// events.LockItemLogCrossProcess acquires first, under one logs
	// directory.
	logsDirBefore := filepath.Join(t.TempDir(), "logs-before")
	_, unlockCrossProcess, err := events.LockItemLogCrossProcess(context.Background(), locksRoot, logsDirBefore, itemID)
	require.NoError(t, err)

	// The reconcile lock, resolved purely from the workspace (never from any
	// logs directory), must still observe the resource as busy even though
	// no explicit "logs directory" was ever passed to it.
	_, _, lockErr := lockShipmentReconcileItemLog(context.Background(), ws, itemID)
	require.Error(t, lockErr, "the reconcile lock must exclude a cross-process lock holder regardless of which logs directory that holder observed")

	unlockCrossProcess()

	_, unlockReconcile, err := lockShipmentReconcileItemLog(context.Background(), ws, itemID)
	require.NoError(t, err)
	_ = unlockReconcile()
}

func TestLockShipmentReconcileItemLog_ValidatesInputs(t *testing.T) {
	ws := setupShipmentWorkspace(t)

	_, _, err := lockShipmentReconcileItemLog(context.Background(), nil, "167.011-T")
	assert.Error(t, err, "a nil workspace must be refused")

	_, _, err = lockShipmentReconcileItemLog(context.Background(), ws, "")
	assert.Error(t, err, "an empty item id must be refused")
}

func TestLockShipmentReconcileItemLog_BoundedWaitOnContention(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	itemID := "167.011-T-bounded-wait"

	_, unlock, err := lockShipmentReconcileItemLog(context.Background(), ws, itemID)
	require.NoError(t, err)
	defer func() { _ = unlock() }()

	start := time.Now()
	_, _, lockErr := lockShipmentReconcileItemLog(context.Background(), ws, itemID)
	elapsed := time.Since(start)

	require.Error(t, lockErr, "a held lock must remain busy without stale reclamation")
	assert.Less(t, elapsed, 10*time.Second, "acquisition must fail within a bounded wait, not hang indefinitely")
}
