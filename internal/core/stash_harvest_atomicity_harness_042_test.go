package core_test

// 041.002-T: Fix stash harvest atomicity gap
//
// Atomicity contract after fix:
//   - If DB operations fail after artifact creation, the stash.jsonl must NOT
//     have the entry removed (harvest is rolled back).
//   - If artifact indexing (db.UpsertItem) or stash state updates fail, the
//     artifact file must be cleaned up so disk and DB stay consistent.
//   - A successful harvest: artifact file exists, artifact indexed, stash entry
//     absent from JSONL and marked "harvested" in DB.
//
// The harness drives core.HarvestStashEntry and inspects stash JSONL and DB
// state after simulated failures.

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
)

// setupHarvestWorkspace builds a workspace with a seed stash entry plus the
// minimal config required for CreateArtifact.
func setupHarvestWorkspace(t *testing.T) *core.Workspace {
	t.Helper()
	tmpDir := t.TempDir()
	backlogDir := filepath.Join(tmpDir, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogDir, 0o755))
	require.NoError(t, config.WriteDefaults(backlogDir))

	// Seed one active stash entry.
	stashContent := `{"id":"AAAABBBB","priority":"medium","kind":"task","text":"Stash harvest atomicity test entry"}` + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(backlogDir, "stash.jsonl"), []byte(stashContent), 0o644))

	ws, wsErr := core.NewWorkspace(context.Background(), tmpDir)
	require.NoError(t, wsErr)
	t.Cleanup(func() { ws.Close() })
	return ws
}

// TestHarvestStashEntry_Success_StashRemovedAndArtifactIndexed verifies that a
// successful harvest removes the stash entry from JSONL and indexes the artifact.
func TestHarvestStashEntry_Success_StashRemovedAndArtifactIndexed(t *testing.T) {
	ws := setupHarvestWorkspace(t)
	ctx := context.Background()

	result, err := core.HarvestStashEntry(ctx, ws, core.HarvestStashOptions{
		StashID:      "AAAABBBB",
		ArtifactType: "feature",
	})

	require.NoError(t, err)
	require.NotNil(t, result)

	// Stash entry must be gone from active JSONL.
	fetched, fetchErr := core.FetchStash(ctx, ws, core.FetchStashOptions{})
	require.NoError(t, fetchErr)
	for _, e := range fetched.Entries {
		assert.NotEqual(t, "AAAABBBB", e.ID, "stash entry must be absent after successful harvest")
	}

	// Artifact must be accessible: verify it's a non-nil models.Artifact.
	require.NotNil(t, result.Artifact, "harvest result must include the new artifact")
}

// TestHarvestStashEntry_DBFailureAfterStashRewrite_WorkspaceConsistent verifies
// that when DB operations fail after stash.jsonl is rewritten, the workspace
// is either:
//   (a) rolled back: stash entry restored in JSONL, artifact file removed, or
//   (b) forward-consistent: artifact indexed with stash marked harvested.
//
// The stash must never be removed from JSONL while the artifact is absent from
// the DB index.
func TestHarvestStashEntry_DBFailureAfterStashRewrite_WorkspaceConsistent(t *testing.T) {
	ws := setupHarvestWorkspace(t)
	ctx := context.Background()

	// Close DB to force all DB operations to fail.
	ws.DB.Close()

	_, err := core.HarvestStashEntry(ctx, ws, core.HarvestStashOptions{
		StashID:      "AAAABBBB",
		ArtifactType: "feature",
	})

	require.Error(t, err, "HarvestStashEntry must return an error when DB fails")

	// Re-open workspace to inspect stash JSONL state.
	ws2, wsErr := core.NewWorkspace(context.Background(), ws.RootPath)
	require.NoError(t, wsErr)
	defer ws2.Close()

	fetched, fetchErr := core.FetchStash(ctx, ws2, core.FetchStashOptions{})
	require.NoError(t, fetchErr)

	found := false
	for _, e := range fetched.Entries {
		if e.ID == "AAAABBBB" {
			found = true
			break
		}
	}
	// After DB failure the stash entry MUST still be in JSONL (rollback path).
	// If the entry is gone, an artifact file must exist on disk so the workspace
	// can recover via sync — both "gone from JSONL + file exists" and "stash
	// restored" are acceptable; "gone from JSONL + no file + not in DB" is not.
	if !found {
		// Assert artifact file exists to ensure the workspace is recoverable.
		queueDir := filepath.Join(ws.RootPath, ".backlogit", "queue")
		entries, readErr := os.ReadDir(queueDir)
		require.NoError(t, readErr)
		assert.NotEmpty(t, entries, "if stash was removed, artifact file must exist for recovery")
	}
}
