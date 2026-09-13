package core

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
)

// shipmentReconcileLockWait bounds how long lockShipmentReconcileItemLogImpl
// retries a contended acquisition before failing with ErrShipmentReconcileLockBusy.
// It matches events.itemLogLockWait so both acquirers of the shared sidecar
// present a consistent contention window to callers.
const shipmentReconcileLockWait = 3 * time.Second

// ErrShipmentReconcileLockBusy signals that the item-log lock (C) sidecar is
// already held — either by events.LockItemLogCrossProcess (an existing
// writer: AssociateCommit, ArchiveItem, LinkCommit) or by another reconcile
// acquirer — after the bounded retry window elapsed. A static sentinel, so
// errors.New is used rather than fmt.Errorf (no formatting verbs are ever
// supplied here), matching internal/errors/errors.go's own convention for
// every other static sentinel in this codebase.
var ErrShipmentReconcileLockBusy = errors.New("shipment reconcile item log lock is busy")

// lockShipmentReconcileItemLogImpl is the real implementation behind the
// gated lockShipmentReconcileItemLog declaration (167.003-T panic body,
// 167.011-T behavior).
//
// It resolves the IDENTICAL stable sidecar path events.ItemLogLockPath
// returns for the same (locksRoot, itemID) pair — the same resource
// events.LockItemLogCrossProcess locks (167.017-T) — so this primitive and
// that one contend on the same underlying OS advisory lock (same path, same
// inode) regardless of which one opened the handle. It then opens that
// sidecar directory-handle-relative: the "itemlog" namespace directory is
// opened and validated as a real, non-reparse directory FIRST, and the
// sidecar FILE is opened relative to that verified directory handle (POSIX
// openat on Unix; a hardened, reparse-safe directory-validated open on
// Windows, where a true handle-relative create requires NT-native APIs this
// codebase does not otherwise depend on — see
// shipment_reconcile_lock_windows.go for the documented parity note). This
// is intentionally a STRICTER open discipline than
// events.LockItemLogCrossProcess's own sidecar open uses; both still target
// the same path, so the shared OS-level lock still mutually excludes
// regardless of which side's open path is stricter.
//
// Do NOT call events.LockItemLogCrossProcess from here: it re-derives and
// re-opens the sidecar purely from (logsDir, itemID) and does not bind to a
// verified directory handle at all (167.011-T scope note).
func lockShipmentReconcileItemLogImpl(ctx context.Context, ws *Workspace, itemID string) (context.Context, func() error, error) {
	if ws == nil {
		return ctx, nil, fmt.Errorf("lock shipment reconcile item log: workspace is required: %w", blerrors.ErrValidation)
	}
	if itemID == "" {
		return ctx, nil, fmt.Errorf("lock shipment reconcile item log: item id is required: %w", blerrors.ErrValidation)
	}

	lockPath, err := events.ItemLogLockPath(WorkspaceLocksRoot(ws.RootPath), itemID)
	if err != nil {
		return ctx, nil, fmt.Errorf("resolve shipment reconcile item log lock: %w", err)
	}
	namespaceDir := filepath.Dir(lockPath)
	fileName := filepath.Base(lockPath)

	deadline := time.Now().Add(shipmentReconcileLockWait)
	backoff := 20 * time.Millisecond
	for {
		file, busy, openErr := openShipmentReconcileLockHandleRelative(namespaceDir, fileName)
		if openErr != nil {
			return ctx, nil, openErr
		}
		if !busy {
			var once sync.Once
			unlock := func() error {
				var closeErr error
				once.Do(func() {
					closeErr = file.Close()
				})
				return closeErr
			}
			return ctx, unlock, nil
		}
		if !time.Now().Before(deadline) {
			return ctx, nil, fmt.Errorf("%s: %w", lockPath, ErrShipmentReconcileLockBusy)
		}
		select {
		case <-ctx.Done():
			return ctx, nil, ctx.Err()
		case <-time.After(backoff):
		}
		if backoff < 250*time.Millisecond {
			backoff *= 2
		}
	}
}
