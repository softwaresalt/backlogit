package core_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/models"
)

// 111.001-T / stash 50C90A1B guard: `backlogit update` on an already-archived
// item must not silently drop the archive-provenance keys (archived_from,
// archived_status). Those keys are written raw by ArchiveItem but the typed
// update round-trip (findArtifact -> ArtifactFromFrontmatter ->
// persistArtifact/WriteArtifactFile) formerly discarded them because
// models.Artifact carried no field for them, leaving the archive record
// non-invertible (UnarchiveItem hard-fails "missing archived_from").
//
// archivedProvenanceItemFile handcrafts an archived task whose frontmatter
// carries both provenance keys so the preserve/clear assertions are non-vacuous.
const archivedProvenanceItemFile = "---\n" +
	"archived_from: queue/910.002-T.md\n" +
	"archived_status: done\n" +
	"artifact_type: task\n" +
	"created_at: 2026-07-18T00:00:00.000Z\n" +
	"id: 910.002-T\n" +
	"parent_id: 910-F\n" +
	"priority: high\n" +
	"status: archived\n" +
	"title: Archived provenance task\n" +
	"updated_at: 2026-07-18T00:00:00.000Z\n" +
	"---\n" +
	"\n" +
	"# Heading\n" +
	"\n" +
	"Body paragraph.\n"

// TestUpdateArtifact_PreservesArchiveProvenanceOnArchivedItem proves that a
// commit association on an archived item (status unchanged, no relocation)
// preserves both archive-provenance keys through the update round-trip.
func TestUpdateArtifact_PreservesArchiveProvenanceOnArchivedItem(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	backlogitDir := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(filepath.Join(backlogitDir, "queue"), 0o755))
	require.NoError(t, config.WriteDefaults(backlogitDir))

	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ws.Close() })

	path := filepath.Join(backlogitDir, "queue", "910.002-T.md")
	require.NoError(t, os.WriteFile(path, []byte(archivedProvenanceItemFile), 0o644))

	// Precondition: the raw file carries both archive-provenance keys, so a
	// later absence is a genuine drop, not a setup omission.
	rawIn, err := os.ReadFile(path)
	require.NoError(t, err)
	fmIn, _, err := models.ParseFrontmatter(string(rawIn))
	require.NoError(t, err)
	require.Contains(t, fmIn, "archived_from", "fixture must carry archived_from")
	require.Contains(t, fmIn, "archived_status", "fixture must carry archived_status")

	// A commit association: status stays archived, so no relocation occurs and
	// the file is rewritten in place.
	updated, err := core.UpdateArtifact(ctx, ws, "910.002-T", map[string]any{"commit": "abc1234"})
	require.NoError(t, err)
	assert.Equal(t, "abc1234", updated.Commit)
	assert.Equal(t, models.StatusArchived, updated.Status)

	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	fmOut, _, err := models.ParseFrontmatter(string(raw))
	require.NoError(t, err)

	assert.Equal(t, "queue/910.002-T.md", fmOut["archived_from"],
		"archived_from must survive an update round-trip on an archived item")
	assert.Equal(t, "done", fmOut["archived_status"],
		"archived_status must survive an update round-trip on an archived item")
	assert.Equal(t, "archived", fmOut["status"], "status must remain archived")
	assert.Equal(t, "abc1234", fmOut["commit"], "commit must be applied")
}

// TestUpdateArtifact_RejectsUnarchiveViaStatusUpdate pins the assumption behind
// the status-gated writer: the typed update path can NEVER move an item out of
// archived status because the validate_status_transition hook rejects it.
// Unarchiving therefore only happens through UnarchiveItem (raw frontmatter
// path), which is why no clearStaleArchiveProvenance helper is needed on the
// update path. If this coupling is ever removed or the hook reconfigured, this
// guard fails loudly, flagging that the status-gate design must be revisited.
// The rejected attempt must also leave the archived file's provenance intact.
func TestUpdateArtifact_RejectsUnarchiveViaStatusUpdate(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	backlogitDir := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(filepath.Join(backlogitDir, "queue"), 0o755))
	require.NoError(t, config.WriteDefaults(backlogitDir))

	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ws.Close() })

	path := filepath.Join(backlogitDir, "queue", "910.002-T.md")
	require.NoError(t, os.WriteFile(path, []byte(archivedProvenanceItemFile), 0o644))

	_, err = core.UpdateArtifact(ctx, ws, "910.002-T", map[string]any{"status": "queued"})
	require.Error(t, err, "archived -> queued must be rejected by the transition hook")
	assert.Contains(t, err.Error(), "transition",
		"rejection must originate from the status-transition validator")

	// The rejected update must not have corrupted the archived record.
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	fmOut, _, err := models.ParseFrontmatter(string(raw))
	require.NoError(t, err)
	assert.Equal(t, "queue/910.002-T.md", fmOut["archived_from"], "provenance preserved after rejected update")
	assert.Equal(t, "done", fmOut["archived_status"], "provenance preserved after rejected update")
	assert.Equal(t, "archived", fmOut["status"], "status must remain archived after rejected update")
}

// TestWriteArtifactFile_ArchiveProvenanceIsStatusGated proves the writer
// contract that carries preservation AND the clear-on-unarchive semantics:
// archive-provenance keys are emitted only while the artifact is archived.
//
// The update path cannot move an item out of archived status (the
// validate_status_transition hook forbids archived -> queued), so unarchiving
// only happens through UnarchiveItem, which clears the keys via the raw
// frontmatter path. Gating the typed writer on archived status therefore keeps
// the invariant "archive provenance <=> archived status" enforced at the one
// seam the typed update path controls, without introducing an unreachable
// clear helper.
func TestWriteArtifactFile_ArchiveProvenanceIsStatusGated(t *testing.T) {
	base := models.Artifact{
		ID:             "910.003-T",
		Title:          "Writer contract task",
		ArtifactType:   "task",
		ParentID:       "910-F",
		ArchivedFrom:   "queue/910.003-T.md",
		ArchivedStatus: "done",
		CreatedAt:      models.NowUTC(),
		UpdatedAt:      models.NowUTC(),
	}

	t.Run("emits provenance when archived", func(t *testing.T) {
		artifact := base
		artifact.Status = models.StatusArchived
		path := filepath.Join(t.TempDir(), "910.003-T.md")
		require.NoError(t, core.WriteArtifactFile(&artifact, path))

		raw, err := os.ReadFile(path)
		require.NoError(t, err)
		fmOut, _, err := models.ParseFrontmatter(string(raw))
		require.NoError(t, err)
		assert.Equal(t, "queue/910.003-T.md", fmOut["archived_from"],
			"archived item must emit archived_from")
		assert.Equal(t, "done", fmOut["archived_status"],
			"archived item must emit archived_status")
	})

	t.Run("omits provenance when not archived", func(t *testing.T) {
		artifact := base
		artifact.Status = models.StatusQueued
		path := filepath.Join(t.TempDir(), "910.003-T.md")
		require.NoError(t, core.WriteArtifactFile(&artifact, path))

		raw, err := os.ReadFile(path)
		require.NoError(t, err)
		fmOut, _, err := models.ParseFrontmatter(string(raw))
		require.NoError(t, err)
		assert.NotContains(t, fmOut, "archived_from",
			"non-archived item must not emit stale archived_from")
		assert.NotContains(t, fmOut, "archived_status",
			"non-archived item must not emit stale archived_status")
		assert.Equal(t, "queued", fmOut["status"], "status must be persisted")
	})
}
