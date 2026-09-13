package events

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// itemLogLockNamespaceDirName is the fixed, dotfile-adjacent leaf directory
// (under the caller-supplied locks root) holding ONLY item-log cross-process
// lock (C) sidecars — never real log content, and never the membership lock
// (A, "<shipmentID>") or artifact-mutation lock (B, "artifacts/<hex(id)>")
// leaves that share the same locks root in internal/core. A shipment item's
// itemID can equal its shipmentID, so without this distinct namespace a
// shared leaf would collapse the A/C distinction (167.017-T).
const itemLogLockNamespaceDirName = "itemlog"

// ItemLogLockIdentityVersion identifies the item-log lock (C) identity
// scheme currently in effect: a stable, handle-validated sidecar rooted at
// the caller's locks root rather than the (swappable) logs directory
// (167.017-T). It is durably recorded via the version marker so a FUTURE
// migration between two versions that both understand a lease protocol can
// use one; THIS migration (legacy recomputed-pathname -> stable
// handle-bound) cannot, because legacy binaries predate the marker and
// cannot be retrofitted to honor it. See
// docs/design-docs/2026-09-12-item-log-lock-identity-migration-runbook.md
// for the required stop-the-world operational precondition.
const ItemLogLockIdentityVersion = "stable-handle-bound-v1"

// itemLogLockVersionMarkerName is the durable marker file written into the
// item-log lock namespace recording ItemLogLockIdentityVersion.
const itemLogLockVersionMarkerName = ".identity-version"

// itemLogLockPathContained reports whether p resolves inside root using a
// lexical (not symlink-resolving) relative-path check. Callers are expected
// to have already EvalSymlinks'd both arguments so the comparison is over
// real paths.
func itemLogLockPathContained(root, p string) bool {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(p))
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// ItemLogLockPath resolves the STABLE, validated sidecar path for itemID's
// item-log cross-process lock (C). Unlike the legacy path derivation it
// supersedes (which recomputed a pathname inside the swappable logs
// directory), this path is rooted at locksRoot — a caller-supplied stable
// directory expected to be the SAME root as membership lock A and
// artifact-mutation lock B (internal/core's ".locks" directory) — so it
// remains identical across a logs-directory reconfiguration or replacement
// (167.017-T).
//
// The itemID is encoded into a single safe filename component via
// hex.EncodeToString, matching internal/core's artifactMutationLockPath
// convention: hex output contains only [0-9a-f], so it can never contain a
// path separator, ".." segment, or any other traversal-relevant byte, no
// matter what the caller-controlled itemID contains.
//
// SCOPE BOUND (PR #424 review, thread ft-pC; carried into 167.017-T): this
// validates and contains the "itemlog" leaf directory under locksRoot; it
// does NOT defend against an adversarial rename or symlink-swap of locksRoot
// itself. No lock in this codebase defends that today — membership lock A
// and artifact-mutation lock B (internal/core/shipment.go) open their own
// roots by path too — so this is a repository-wide threat-model boundary,
// not a gap specific to this lock.
func ItemLogLockPath(locksRoot, itemID string) (string, error) {
	if strings.TrimSpace(locksRoot) == "" {
		return "", fmt.Errorf("item log lock: locks root is required")
	}
	if itemID == "" {
		return "", fmt.Errorf("item log lock: item id is required")
	}

	absRoot, err := filepath.Abs(locksRoot)
	if err != nil {
		return "", fmt.Errorf("resolve item log locks root: %w", err)
	}
	absRoot = filepath.Clean(absRoot)
	if err := os.MkdirAll(absRoot, 0o755); err != nil {
		return "", fmt.Errorf("create item log locks root: %w", err)
	}
	realRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		return "", fmt.Errorf("resolve item log locks root symlinks: %w", err)
	}

	namespaceDir := filepath.Join(realRoot, itemLogLockNamespaceDirName)
	if existing, evalErr := filepath.EvalSymlinks(namespaceDir); evalErr == nil {
		if !itemLogLockPathContained(realRoot, existing) {
			return "", fmt.Errorf("item log lock namespace resolves outside locks root: %w", blerrors.ErrValidation)
		}
		namespaceDir = existing
	} else if !os.IsNotExist(evalErr) {
		return "", fmt.Errorf("resolve item log lock namespace: %w", evalErr)
	}
	if !itemLogLockPathContained(realRoot, namespaceDir) {
		return "", fmt.Errorf("item log lock namespace is outside locks root: %w", blerrors.ErrValidation)
	}
	if err := os.MkdirAll(namespaceDir, 0o755); err != nil {
		return "", fmt.Errorf("create item log lock namespace: %w", err)
	}
	realNamespaceDir, err := filepath.EvalSymlinks(namespaceDir)
	if err != nil {
		return "", fmt.Errorf("resolve item log lock namespace containment: %w", err)
	}
	if !itemLogLockPathContained(realRoot, realNamespaceDir) {
		return "", fmt.Errorf("item log lock namespace resolves outside locks root: %w", blerrors.ErrValidation)
	}

	writeItemLogLockVersionMarker(realNamespaceDir)

	encoded := hex.EncodeToString([]byte(itemID))
	lockPath := filepath.Join(realNamespaceDir, "."+encoded+".lock")
	if !itemLogLockPathContained(realNamespaceDir, lockPath) {
		return "", fmt.Errorf("item id %q resolves outside the item log lock namespace: %w", itemID, blerrors.ErrValidation)
	}
	return lockPath, nil
}

// writeItemLogLockVersionMarker durably records ItemLogLockIdentityVersion in
// the item-log lock namespace so tooling (and any future migration) can
// detect which identity scheme a workspace's item-log locks currently use.
// Best-effort: a write failure here must never block lock acquisition — the
// marker is informational/forward-compatibility metadata, not part of the
// mutual-exclusion mechanism itself.
//
// Copilot PR #440 review, finding 3: markerPath lives inside namespaceDir,
// which — despite namespaceDir itself already having been validated a few
// lines above in ItemLogLockPath — could in principle have had its
// ".identity-version" leaf independently swapped for a symlink pointing
// outside the workspace before this write runs (a distinct planted-file
// TOCTOU from the namespace-directory swap ItemLogLockPath already
// defends against). Since this marker is durability/migration metadata,
// not itself security-critical data, a full handle-relative rewrite is
// disproportionate; instead this applies the same lightweight
// Lstat-and-reject-ModeSymlink pattern this codebase already uses for
// other lower-criticality writes (e.g. checkpoint_readnofollow_windows.go's
// readFileNoFollow), refusing to blindly follow a symlink the way a bare
// os.WriteFile would.
func writeItemLogLockVersionMarker(namespaceDir string) {
	markerPath := filepath.Join(namespaceDir, itemLogLockVersionMarkerName)
	if info, err := os.Lstat(markerPath); err == nil && info.Mode()&os.ModeSymlink != 0 {
		// A symlink already sits at the marker path: refuse to follow it.
		// Best-effort semantics mean this is silently skipped, exactly like
		// any other write failure here.
		return
	}
	_ = os.WriteFile(markerPath, []byte(ItemLogLockIdentityVersion+"\n"), 0o644)
}
