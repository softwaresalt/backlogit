package core_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/models"
)

// TestArchiveItem_ReArchivePreservesReconciledArchivedStatus is the 167.021-T
// behavior harness (a): ArchiveItem currently stamps
// fm["archived_status"] = oldStatus unconditionally. For an already-archived
// item (status:archived) whose archived_status was set by a governed
// reconciliation (e.g. "shipped"), oldStatus == "archived" — so a naive
// re-archive would overwrite the reconciled value with the malformed
// self-provenance "archived". This test proves the guard: the existing
// non-empty archived_status MUST be preserved verbatim.
func TestArchiveItem_ReArchivePreservesReconciledArchivedStatus(t *testing.T) {
	ws := setupArchiveWorkspace(t)
	ctx := context.Background()

	archiveDir := filepath.Join(ws.RootPath, ".backlogit", "archive")
	archiveFilePath := filepath.Join(archiveDir, "070-S.md")
	content := "---\nid: 070-S\ntitle: Reconciled shipment\nstatus: archived\narchived_status: shipped\nartifact_type: shipment\n---\nBody\n"
	require.NoError(t, os.WriteFile(archiveFilePath, []byte(content), 0o644))
	require.NoError(t, db.UpsertItem(ctx, ws.DB, &models.Artifact{
		ID: "070-S", Title: "Reconciled shipment", Status: models.StatusArchived, ArtifactType: "shipment",
	}))

	record, err := core.ArchiveItem(ctx, ws.DB, ws, "070-S")
	require.NoError(t, err)

	raw, readErr := os.ReadFile(record.ArchivePath)
	require.NoError(t, readErr)
	fm, _, parseErr := models.ParseFrontmatter(string(raw))
	require.NoError(t, parseErr)
	assert.Equal(t, "shipped", fm["archived_status"],
		"re-archiving an already-reconciled item must PRESERVE archived_status, not overwrite it with 'archived'")
}

// TestArchiveItem_GenuineQueuedToArchiveStillStampsPreArchiveStatus is the
// 167.021-T control case (b): the genuine queued->archive transition
// (oldStatus != "archived") must still stamp archived_status = the
// pre-archive status, byte-for-byte unchanged from existing behavior.
func TestArchiveItem_GenuineQueuedToArchiveStillStampsPreArchiveStatus(t *testing.T) {
	ws := setupArchiveWorkspace(t)
	ctx := context.Background()

	record, err := core.ArchiveItem(ctx, ws.DB, ws, "001-T")
	require.NoError(t, err)

	raw, readErr := os.ReadFile(record.ArchivePath)
	require.NoError(t, readErr)
	fm, _, parseErr := models.ParseFrontmatter(string(raw))
	require.NoError(t, parseErr)
	assert.Equal(t, "done", fm["archived_status"],
		"a genuine queued->archive transition must still stamp archived_status = pre-archive status")
}

// TestArchiveItem_ReArchiveWithEmptyArchivedStatusStillStamps is a boundary
// case: an already-archived item with NO existing archived_status (empty)
// falls back to stamping oldStatus, so a malformed/legacy record is not left
// permanently empty.
func TestArchiveItem_ReArchiveWithEmptyArchivedStatusStillStamps(t *testing.T) {
	ws := setupArchiveWorkspace(t)
	ctx := context.Background()

	archiveDir := filepath.Join(ws.RootPath, ".backlogit", "archive")
	archiveFilePath := filepath.Join(archiveDir, "071-T.md")
	content := "---\nid: 071-T\ntitle: Legacy archived task\nstatus: archived\nartifact_type: task\n---\nBody\n"
	require.NoError(t, os.WriteFile(archiveFilePath, []byte(content), 0o644))
	require.NoError(t, db.UpsertItem(ctx, ws.DB, &models.Artifact{
		ID: "071-T", Title: "Legacy archived task", Status: models.StatusArchived, ArtifactType: "task",
	}))

	record, err := core.ArchiveItem(ctx, ws.DB, ws, "071-T")
	require.NoError(t, err)

	raw, readErr := os.ReadFile(record.ArchivePath)
	require.NoError(t, readErr)
	fm, _, parseErr := models.ParseFrontmatter(string(raw))
	require.NoError(t, parseErr)
	assert.Equal(t, "archived", fm["archived_status"],
		"a legacy record with no existing archived_status falls back to stamping oldStatus")
}
