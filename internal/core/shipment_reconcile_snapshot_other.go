//go:build !windows && !(linux || darwin || freebsd || netbsd || openbsd || dragonfly)

package core

import (
	"fmt"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// readShipmentReconcileArchiveSnapshotFile is the fallback for a truly
// generic non-Unix, non-Windows platform (e.g. js/wasm, plan9, solaris):
// this codebase has no reparse-point/no-follow primitive available on such
// platforms, and every sibling platform-split file in this family
// (shipment_reconcile_fs_other.go, shipment_reconcile_append_other.go,
// shipment_reconcile_evidence_other.go, shipment_reconcile_lock_other.go)
// already treats this same tier as genuinely unsupported rather than
// attempting a best-effort mitigation. Windows previously shared this
// fallback too (the pre-167.016-T-windows-hardening state: os.Lstat
// symlink/type check, then a separate os.ReadFile — two pathname
// operations, hence a check/use TOCTOU race), which is why Windows now has
// its own dedicated, handle-verified implementation in
// shipment_reconcile_snapshot_windows.go, matching every sibling file's own
// windows/other split (Copilot PR #440 review, finding 1).
//
// Residual risk, documented explicitly per this codebase's established
// convention: this fallback is reached ONLY on a platform this codebase
// does not otherwise support for the reconcile write path either (the
// sibling writer/lock/evidence primitives above already refuse to run at
// all there), so refusing outright here is consistent rather than
// introducing a weaker, unexercised mitigation for a tier nothing else in
// this family actually operates on.
func readShipmentReconcileArchiveSnapshotFile(string, string) ([]byte, error) {
	return nil, fmt.Errorf("shipment reconcile archive snapshot read unsupported on this platform: %w", blerrors.ErrValidation)
}
