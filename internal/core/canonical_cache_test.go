package core

// 070.001-T harness: batch the canonical-uniqueness scan for bulk CreateArtifact
// callers. scanCanonicalArtifacts walks+parses every queue/archive .md on each
// CreateArtifact call, so bulk callers (migrate import loop, stash harvest) incur
// O(files) per create -> O(N^2). A CanonicalCache scans once per batch and is
// threaded into each create via WithCanonicalCache; single interactive creates
// keep scanning per call. RED until the cache short-circuits the per-call scan
// and bulk callers adopt it.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
)

// newCacheTestWorkspace builds a minimal real workspace (config + DB) for the
// canonical-cache batching tests.
func newCacheTestWorkspace(t *testing.T) *Workspace {
	t.Helper()
	tmp := t.TempDir()
	backlogDir := filepath.Join(tmp, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogDir, 0o755))
	require.NoError(t, config.WriteDefaults(backlogDir))
	ws, err := NewWorkspace(context.Background(), tmp)
	require.NoError(t, err)
	t.Cleanup(func() { ws.Close() })
	return ws
}

// installScanCounter swaps the package-level canonical-scan seam for a counting
// wrapper so a test can assert how many full filesystem scans a flow performs.
// The creates under test are sequential, so a plain counter is race-free.
func installScanCounter(t *testing.T) *int {
	t.Helper()
	count := 0
	prev := scanCanonicalArtifactsFn
	scanCanonicalArtifactsFn = func(ws *Workspace) (map[string][]artifactRef, error) {
		count++
		return prev(ws)
	}
	t.Cleanup(func() { scanCanonicalArtifactsFn = prev })
	return &count
}

// seedStashWorkspace builds a workspace whose stash JSONL holds the given
// low-priority task entries, ready for a batch harvest.
func seedStashWorkspace(t *testing.T, ids ...string) *Workspace {
	t.Helper()
	tmp := t.TempDir()
	backlogDir := filepath.Join(tmp, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogDir, 0o755))
	require.NoError(t, config.WriteDefaults(backlogDir))

	var sb string
	for _, id := range ids {
		sb += fmt.Sprintf(`{"id":%q,"priority":"low","kind":"task","text":"entry %s"}`+"\n", id, id)
	}
	require.NoError(t, os.WriteFile(filepath.Join(backlogDir, "stash.jsonl"), []byte(sb), 0o644))

	ws, err := NewWorkspace(context.Background(), tmp)
	require.NoError(t, err)
	t.Cleanup(func() { ws.Close() })
	return ws
}

// TestCreateArtifact_InteractiveScansPerCall locks in the unchanged single-create
// behavior: every interactive CreateArtifact call performs its own canonical scan.
func TestCreateArtifact_InteractiveScansPerCall(t *testing.T) {
	ws := newCacheTestWorkspace(t)
	ctx := context.Background()
	count := installScanCounter(t)

	const n = 4
	for i := 0; i < n; i++ {
		_, err := CreateArtifact(ctx, ws, fmt.Sprintf("Feature %d", i), "feature")
		require.NoError(t, err)
	}
	assert.Equal(t, n, *count, "interactive creates scan the canonical set once per call")
}

// TestCreateArtifact_BatchCacheScansOnce is the core acceptance test: a bulk
// batch that shares one CanonicalCache performs the canonical scan exactly once,
// not once per create, while still minting distinct IDs.
func TestCreateArtifact_BatchCacheScansOnce(t *testing.T) {
	ws := newCacheTestWorkspace(t)
	ctx := context.Background()
	count := installScanCounter(t)

	cache, err := NewCanonicalCache(ws)
	require.NoError(t, err)
	require.Equal(t, 1, *count, "constructing the cache performs exactly one scan")

	const n = 5
	ids := make(map[string]bool, n)
	for i := 0; i < n; i++ {
		a, err := CreateArtifact(ctx, ws, fmt.Sprintf("Batch feature %d", i), "feature", WithCanonicalCache(cache))
		require.NoError(t, err)
		assert.False(t, ids[a.ID], "batch must mint distinct IDs")
		ids[a.ID] = true
	}
	assert.Equal(t, 1, *count, "a bulk batch performs the canonical scan once for the whole batch, not once per create")
	assert.Len(t, ids, n, "every batch create produced a distinct artifact")
}

// TestCanonicalCache_RecordMakesIDVisible verifies the within-batch uniqueness
// guard: a freshly created ID recorded into the cache is visible to later creates
// in the same batch without re-scanning the filesystem.
func TestCanonicalCache_RecordMakesIDVisible(t *testing.T) {
	ws := newCacheTestWorkspace(t)
	cache, err := NewCanonicalCache(ws)
	require.NoError(t, err)

	require.Empty(t, cache.lookup("999-X"), "id absent before record")
	cache.record("999-X", filepath.Join("queue", "999-X.md"))
	assert.NotEmpty(t, cache.lookup("999-X"), "recorded ids must be visible to later batch creates")
}

// TestCanonicalCache_ZeroValueSeedsOnUse guards the exported-type footgun raised
// in PR review: a caller in another package can construct a zero-value
// &CanonicalCache{} (refs == nil) and pass it to CreateArtifact. Such a cache
// must NOT bypass the canonical uniqueness scan (which would allow duplicate IDs)
// nor panic recording into a nil map; it must lazily seed itself by scanning once
// on first use, then behave like a NewCanonicalCache-built cache for the batch.
func TestCanonicalCache_ZeroValueSeedsOnUse(t *testing.T) {
	ws := newCacheTestWorkspace(t)
	ctx := context.Background()
	count := installScanCounter(t)

	cache := &CanonicalCache{} // zero value: refs == nil (not built via NewCanonicalCache)

	const n = 3
	ids := make(map[string]bool, n)
	for i := 0; i < n; i++ {
		a, err := CreateArtifact(ctx, ws, fmt.Sprintf("Zero-cache feature %d", i), "feature", WithCanonicalCache(cache))
		require.NoError(t, err)
		assert.False(t, ids[a.ID], "batch must mint distinct IDs even with a zero-value cache")
		ids[a.ID] = true
	}
	assert.Equal(t, 1, *count, "a zero-value cache seeds itself with exactly one scan, never bypassing the uniqueness guard")
	assert.Len(t, ids, n, "every create produced a distinct artifact")
}

// TestHarvestStashByPriority_ScansCanonicalOnce proves the real bulk caller
// (priority harvest) scans the canonical set once for the whole batch.
func TestHarvestStashByPriority_ScansCanonicalOnce(t *testing.T) {
	ws := seedStashWorkspace(t, "AAAA1111", "BBBB2222", "CCCC3333")
	ctx := context.Background()
	count := installScanCounter(t)

	res, err := HarvestStashByPriority(ctx, ws, HarvestStashOptions{Priority: "low", ArtifactType: "feature"})
	require.NoError(t, err)
	require.Len(t, res.Results, 3, "all three low-priority entries harvested")

	assert.Equal(t, 1, *count, "batch harvest scans the canonical set once for the whole batch, not once per entry")
}
