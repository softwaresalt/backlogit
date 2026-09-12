package events

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestItemLogLockPath_StableAcrossLogsDirSwap is the core 167.017-T contract:
// the lock path for a given itemID must be IDENTICAL regardless of which logs
// directory is currently configured, because it is derived from a locksRoot
// argument that is independent of any logs directory.
func TestItemLogLockPath_StableAcrossLogsDirSwap(t *testing.T) {
	locksRoot := t.TempDir()

	pathBeforeSwap, err := ItemLogLockPath(locksRoot, "167.017-T")
	require.NoError(t, err)

	// Simulate a logs-directory reconfiguration/replacement: this has no
	// bearing on locksRoot at all, so the lock path must be unaffected.
	_ = t.TempDir() // a "new" logs directory that is never referenced below

	pathAfterSwap, err := ItemLogLockPath(locksRoot, "167.017-T")
	require.NoError(t, err)

	assert.Equal(t, pathBeforeSwap, pathAfterSwap,
		"the item-log lock path must be stable across a logs-directory swap")
}

// TestLockItemLogCrossProcess_MutualExclusionSurvivesLogsDirSwap proves the
// actual mutual-exclusion guarantee: two lockers for the SAME item, using
// DIFFERENT (swapped) logs directories but the SAME locksRoot, must still
// contend on the same OS-level sidecar and therefore mutually exclude.
// Before 167.017-T this failed because the sidecar path was derived from the
// (here, deliberately different) logs directory.
func TestLockItemLogCrossProcess_MutualExclusionSurvivesLogsDirSwap(t *testing.T) {
	locksRoot := t.TempDir()
	logsDirBefore := filepath.Join(t.TempDir(), "logs-before")
	logsDirAfter := filepath.Join(t.TempDir(), "logs-after")
	itemID := "167.017-T"

	// First locker opens under the pre-swap logs directory.
	_, unlockFirst, err := LockItemLogCrossProcess(context.Background(), locksRoot, logsDirBefore, itemID)
	require.NoError(t, err)
	t.Cleanup(func() {
		if unlockFirst != nil {
			unlockFirst()
		}
	})

	// Second locker, in a fresh (uncancelled) context so it does not inherit
	// the first locker's in-process ownership markers, opens under the
	// post-swap logs directory. It must be refused: the underlying sidecar
	// (rooted at the stable locksRoot) is the SAME resource.
	_, _, err = LockItemLogCrossProcess(context.Background(), locksRoot, logsDirAfter, itemID)
	require.Error(t, err, "a lock acquired before a logs-dir swap must still exclude a locker acquiring after the swap")

	unlockFirst()
	unlockFirst = nil

	// Once released, the post-swap locker must succeed — proving this is
	// real contention on a shared resource, not a permanent failure.
	_, unlockSecond, err := LockItemLogCrossProcess(context.Background(), locksRoot, logsDirAfter, itemID)
	require.NoError(t, err)
	unlockSecond()
}

// TestItemLogLockPath_EncodesItemIDSafely proves that an itemID containing
// path-separator-like or traversal-relevant characters cannot escape the
// item-log lock namespace: the encoded filename component must never embed a
// raw separator, and the resolved lock path must remain contained.
func TestItemLogLockPath_EncodesItemIDSafely(t *testing.T) {
	locksRoot := t.TempDir()
	adversarialIDs := []string{
		"../../escape",
		"a/b/c",
		"..",
		`a\b`,
	}
	for _, id := range adversarialIDs {
		lockPath, err := ItemLogLockPath(locksRoot, id)
		require.NoError(t, err, "id %q must resolve to a safe, contained path", id)

		// Compare against the SAME symlink-resolved root the production code
		// used, not the raw locksRoot string: on some platforms t.TempDir()
		// itself resolves through a symlink (e.g. macOS /var -> /private/var),
		// so a naive filepath.Join(locksRoot, ...) comparison would report a
		// false escape that has nothing to do with itemID encoding.
		realRoot, evalErr := filepath.EvalSymlinks(locksRoot)
		require.NoError(t, evalErr)
		namespaceDir := filepath.Join(realRoot, itemLogLockNamespaceDirName)
		rel, relErr := filepath.Rel(namespaceDir, lockPath)
		require.NoError(t, relErr)
		assert.False(t, strings.Contains(rel, ".."),
			"id %q must not escape the item log lock namespace, got relative path %q", id, rel)
		assert.False(t, strings.ContainsAny(filepath.Base(lockPath), `/\`),
			"the encoded lock filename must not contain a path separator for id %q", id)
	}
}

// TestItemLogLockPath_DistinctFromMembershipLockNamespace proves the C
// namespace ("itemlog/<encoded>") can never alias the membership lock A
// namespace ("<shipmentID>" directly under locksRoot), even when itemID
// equals a shipmentID (the exact aliasing scenario 167.017-T calls out).
func TestItemLogLockPath_DistinctFromMembershipLockNamespace(t *testing.T) {
	locksRoot := t.TempDir()
	sharedID := "148-S"

	itemLogPath, err := ItemLogLockPath(locksRoot, sharedID)
	require.NoError(t, err)

	realRoot, evalErr := filepath.EvalSymlinks(locksRoot)
	require.NoError(t, evalErr)

	// Lock A's convention (internal/core/shipment.go lockShipmentMembership):
	// stableKey := filepath.Join(realLocksDir, shipmentID) — i.e. directly
	// under locksRoot, no "itemlog" namespace segment.
	membershipLockPath := filepath.Join(realRoot, sharedID)

	assert.NotEqual(t, membershipLockPath, itemLogPath,
		"the item-log lock (C) path must never alias the membership lock (A) path for the same ID")
	assert.True(t, strings.HasPrefix(itemLogPath, filepath.Join(realRoot, itemLogLockNamespaceDirName)+string(filepath.Separator)),
		"the item-log lock path must live under the distinct itemlog namespace")
}

// TestItemLogLockPath_WritesVersionMarker proves the durable version marker
// (167.017-T rolling-upgrade/migration-safety requirement) is written into
// the item-log lock namespace so a future migration can detect the identity
// scheme in effect.
func TestItemLogLockPath_WritesVersionMarker(t *testing.T) {
	locksRoot := t.TempDir()

	_, err := ItemLogLockPath(locksRoot, "167.017-T")
	require.NoError(t, err)

	markerPath := filepath.Join(locksRoot, itemLogLockNamespaceDirName, itemLogLockVersionMarkerName)
	data, readErr := os.ReadFile(markerPath)
	require.NoError(t, readErr, "the identity version marker must be written on lock-path resolution")
	assert.Equal(t, ItemLogLockIdentityVersion+"\n", string(data))
}
