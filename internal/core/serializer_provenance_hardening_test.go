package core_test

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/models"
)

// TestCreateArtifact_RejectsArchivedInitialStatus guards stash 7EEADCD3:
// CreateArtifact must reject "archived" as an initial status. A freshly created
// item has no archive provenance, and the create serializer emits none, so an
// item born archived is non-invertible (UnarchiveItem cannot restore it). The
// supported path is create-then-ArchiveItem.
func TestCreateArtifact_RejectsArchivedInitialStatus(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	_, err := core.CreateArtifact(ctx, ws, "Born archived", "feature", core.WithStatus("archived"))
	require.Error(t, err, "creating an artifact with initial status 'archived' must be rejected")
	assert.Contains(t, err.Error(), "archived", "error must name the rejected status")

	// The create must not have left a stray file behind.
	_, findErr := core.FindArtifactPath(ctx, ws, "001-F")
	assert.Error(t, findErr, "no artifact file should have been written")
}

// TestCreateArtifact_AllowsNonArchivedStatuses proves the rejection is narrow:
// other explicit statuses still create normally.
func TestCreateArtifact_AllowsNonArchivedStatuses(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	a, err := core.CreateArtifact(ctx, ws, "Active item", "feature", core.WithStatus("active"))
	require.NoError(t, err)
	assert.Equal(t, models.StatusActive, a.Status)
}

// TestMoveInQueue_PreservesArchiveProvenance guards stash 80DD65C4: reordering
// an archived item in a queue view whose status filter includes "archived" must
// not drop archive provenance. MoveInQueue reorders DB-sourced artifacts (from
// QueryQueue) which do not carry archived_from/archived_status; persisting them
// directly rewrites the Markdown with empty provenance. The fix reloads each
// artifact from Markdown before persisting the new position.
func TestMoveInQueue_PreservesArchiveProvenance(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	first, err := core.CreateArtifact(ctx, ws, "First archived feature", "feature", core.WithPriority("high"))
	require.NoError(t, err)
	second, err := core.CreateArtifact(ctx, ws, "Second archived feature", "feature", core.WithPriority("low"))
	require.NoError(t, err)

	// Archive both so each carries archive provenance in Markdown.
	_, err = core.ArchiveItem(ctx, ws.DB, ws, first.ID)
	require.NoError(t, err)
	_, err = core.ArchiveItem(ctx, ws.DB, ws, second.ID)
	require.NoError(t, err)

	// Rehydrate so the DB reflects archived status. The DB codec does not carry
	// archive provenance, which is exactly the divergence this test exercises.
	_, err = db.Rehydrate(ctx, core.WorkspaceStorageRoot(ws.RootPath), ws.DB)
	require.NoError(t, err)

	filter := &core.QueueFilter{Statuses: []string{"archived"}}
	view, err := core.QueryQueue(ctx, ws.DB, filter)
	require.NoError(t, err)
	require.Len(t, view.Items, 2, "both archived items must be visible in the archived queue view")

	// Precondition: both files carry provenance before the move.
	for _, id := range []string{first.ID, second.ID} {
		assertProvenancePresent(t, ctx, ws, id)
	}

	// Reorder: move the second item to position 1. Both items change position
	// and are rewritten.
	require.NoError(t, core.MoveInQueue(ctx, ws, second.ID, 1, filter))

	// Postcondition: provenance survives the reorder rewrite for every item.
	for _, id := range []string{first.ID, second.ID} {
		assertProvenancePresent(t, ctx, ws, id)
	}
}

// TestUpdateArtifact_RejectsTransitionToArchived guards the write-path completion
// of the archive-provenance invariant surfaced by adversarial review of PR for
// 112-F. The default transition matrix allows done -> archived, but the generic
// update path (CLI move, MCP move_item, UpdateArtifact) never stamps
// archived_from/archived_status the way ArchiveItem does. Permitting the
// transition would write status: archived with empty provenance — a
// non-invertible artifact UnarchiveItem cannot restore. The supported path is
// ArchiveItem; a generic update into archived must be rejected.
func TestUpdateArtifact_RejectsTransitionToArchived(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	feature, err := core.CreateArtifact(ctx, ws, "Feature to archive via update", "feature")
	require.NoError(t, err)

	_, err = core.UpdateArtifact(ctx, ws, feature.ID, map[string]any{"status": "active"})
	require.NoError(t, err)
	_, err = core.UpdateArtifact(ctx, ws, feature.ID, map[string]any{"status": "done"})
	require.NoError(t, err)

	// done -> archived is allowed by the transition matrix, but the generic
	// update path must refuse it because it cannot preserve provenance.
	_, err = core.UpdateArtifact(ctx, ws, feature.ID, map[string]any{"status": "archived"})
	require.Error(t, err, "transition to archived via generic update must be rejected")
	assert.Contains(t, err.Error(), "archive", "error must reference the archive operation")

	// The refused update must not have produced a non-invertible archived file.
	path, findErr := core.FindArtifactPath(ctx, ws, feature.ID)
	require.NoError(t, findErr)
	raw, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	fm, _, parseErr := models.ParseFrontmatter(string(raw))
	require.NoError(t, parseErr)
	assert.Equal(t, "done", fm["status"], "status must remain done after the rejected archive update")
	assert.NotContains(t, fm, "archived_from", "no empty archive provenance may be written")
}

// TestBulkUpdateStatus_RejectsArchivedTarget proves the bulk write path shares
// the same guard: archiving must go through ArchiveItem so provenance is stamped.
// BulkUpdateStatus fires no transition hook and stamps no provenance, so it must
// refuse an archived target status outright rather than write non-invertible
// artifacts.
func TestBulkUpdateStatus_RejectsArchivedTarget(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	feature, err := core.CreateArtifact(ctx, ws, "Bulk archive parent", "feature")
	require.NoError(t, err)
	task, err := core.CreateArtifact(ctx, ws, "Bulk archive task", "task", core.WithParent(feature.ID))
	require.NoError(t, err)

	_, err = core.BulkUpdateStatus(ctx, ws.DB, ws, []string{task.ID}, "archived")
	require.Error(t, err, "bulk update to archived must be rejected")
	assert.Contains(t, err.Error(), "archive", "error must reference the archive operation")

	// The task must be untouched (still queued, no provenance written).
	path, findErr := core.FindArtifactPath(ctx, ws, task.ID)
	require.NoError(t, findErr)
	raw, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	fm, _, parseErr := models.ParseFrontmatter(string(raw))
	require.NoError(t, parseErr)
	assert.Equal(t, "queued", fm["status"], "status must be unchanged after the rejected bulk archive")
	assert.NotContains(t, fm, "archived_from", "no empty archive provenance may be written")
}

func assertProvenancePresent(t *testing.T, ctx context.Context, ws *core.Workspace, id string) {
	t.Helper()
	path, err := core.FindArtifactPath(ctx, ws, id)
	require.NoError(t, err)
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	fm, _, err := models.ParseFrontmatter(string(raw))
	require.NoError(t, err)
	assert.Equal(t, "archived", fm["status"], "%s must remain archived", id)
	assert.Contains(t, fm, "archived_from", "%s must retain archived_from through queue reorder", id)
	assert.Contains(t, fm, "archived_status", "%s must retain archived_status through queue reorder", id)
}
