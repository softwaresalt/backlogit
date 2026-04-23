package core_test

// 041.001-T: Fix CLI delete file/index ordering
//
// Crash-safety contract:
//   - If os.Remove fails (file locked/missing), the DB entry must NOT be deleted.
//   - If the DB delete fails, the artifact file must still exist on disk.
//   - A successful delete leaves neither file nor DB entry.
//
// The harness tests core.DeleteArtifact, which will be wired into the CLI
// delete command once implemented.

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

// setupDeleteWorkspace builds a minimal workspace with one seeded task artifact.
func setupDeleteWorkspace(t *testing.T) (*core.Workspace, string) {
	t.Helper()
	tmpDir := t.TempDir()
	backlogDir := filepath.Join(tmpDir, ".backlogit")
	queueDir := filepath.Join(backlogDir, "queue")
	require.NoError(t, os.MkdirAll(queueDir, 0o755))

	dbPath := filepath.Join(backlogDir, "backlogit.db")
	database, err := db.Open(dbPath)
	require.NoError(t, err)
	require.NoError(t, db.EnsureSchema(database))
	t.Cleanup(func() { database.Close() })

	content := "---\nid: 099-T\ntitle: Delete harness task\nstatus: queued\nartifact_type: task\n---\nBody\n"
	filePath := filepath.Join(queueDir, "099-T.md")
	require.NoError(t, os.WriteFile(filePath, []byte(content), 0o644))
	require.NoError(t, db.UpsertItem(context.Background(), database, &models.Artifact{
		ID: "099-T", Title: "Delete harness task", Status: models.StatusQueued, ArtifactType: "task",
	}))

	ws := &core.Workspace{RootPath: tmpDir, DB: database}
	return ws, filePath
}

// TestDeleteArtifact_SuccessRemovesBothFileAndIndex verifies that a successful
// delete removes the artifact file and the DB record.
func TestDeleteArtifact_SuccessRemovesBothFileAndIndex(t *testing.T) {
	ws, filePath := setupDeleteWorkspace(t)
	ctx := context.Background()

	err := core.DeleteArtifact(ctx, ws, "099-T")

	require.NoError(t, err, "DeleteArtifact should succeed for an existing artifact")
	assert.NoFileExists(t, filePath, "artifact file must be removed after successful delete")

	_, dbErr := db.GetItem(ctx, ws.DB, "099-T")
	assert.Error(t, dbErr, "DB entry must be absent after successful delete")
}

// TestDeleteArtifact_DBFailure_FilePreserved verifies that when the DB delete
// fails (simulated by closing the DB before the call), the artifact file is
// preserved so the workspace remains consistent.
func TestDeleteArtifact_DBFailure_FilePreserved(t *testing.T) {
	ws, filePath := setupDeleteWorkspace(t)
	ctx := context.Background()

	// Close the DB to force all DB operations to fail.
	ws.DB.Close()

	err := core.DeleteArtifact(ctx, ws, "099-T")

	require.Error(t, err, "DeleteArtifact must return an error when DB is unavailable")
	assert.FileExists(t, filePath, "artifact file must still exist when DB delete fails")
}

// TestDeleteArtifact_NonExistentID_ReturnsError verifies that attempting to
// delete an unknown ID returns a descriptive error without panicking.
func TestDeleteArtifact_NonExistentID_ReturnsError(t *testing.T) {
	ws, _ := setupDeleteWorkspace(t)
	ctx := context.Background()

	err := core.DeleteArtifact(ctx, ws, "999-T")

	require.Error(t, err, "DeleteArtifact must error for unknown artifact IDs")
}
