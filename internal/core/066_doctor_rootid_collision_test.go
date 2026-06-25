package core_test

// 066.001-T (Unit 1) harness: Doctor surfaces a distinguishable root-ID
// collision when a level-1 ID exists in both queue and archive.
// RED until Doctor emits FindingRootIDCollision.

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
	bldb "github.com/softwaresalt/backlogit/internal/db"
)

func TestDoctor_DetectsRootIDCollisionAcrossQueueArchive(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	// Create a feature and archive it -> archive/<id>.md
	feature, err := core.CreateArtifact(ctx, ws, "Original archived feature", "feature")
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, feature))

	_, err = core.ArchiveItem(ctx, ws.DB, ws, feature.ID)
	require.NoError(t, err)

	// Drop a DIFFERENT level-1 feature into the queue that shares the same root
	// ID (the 066-F bug): two distinct items, one in queue, one in archive.
	queueDir := filepath.Join(core.WorkspaceStorageRoot(ws.RootPath), "queue")
	require.NoError(t, os.MkdirAll(queueDir, 0o755))
	colliding := "---\n" +
		"id: \"" + feature.ID + "\"\n" +
		"title: \"Different queued feature sharing the root id\"\n" +
		"status: active\n" +
		"artifact_type: feature\n" +
		"level: 1\n" +
		"---\nDifferent body.\n"
	require.NoError(t, os.WriteFile(filepath.Join(queueDir, feature.ID+".md"), []byte(colliding), 0o644))

	report, err := core.Doctor(ctx, ws, allDoctorChecks)
	require.NoError(t, err)
	require.NotNil(t, report)

	var rootCollision, duplicate bool
	for _, f := range report.Findings {
		if f.ArtifactID != feature.ID {
			continue
		}
		switch f.Type {
		case core.FindingRootIDCollision:
			rootCollision = true
		case core.FindingDuplicateID:
			duplicate = true
		}
	}
	assert.True(t, rootCollision, "Doctor must report a distinguishable root-ID collision for a level-1 id present in queue+archive")
	assert.True(t, duplicate, "existing FindingDuplicateID behavior must be preserved (additive signal)")
}

func TestDoctor_Level2DuplicateIsNotRootCollision(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	feature, err := core.CreateArtifact(ctx, ws, "Parent feature", "feature")
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, feature))

	task, err := core.CreateArtifact(ctx, ws, "Child task", "task", core.WithParent(feature.ID))
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, task))

	record, err := core.ArchiveItem(ctx, ws.DB, ws, task.ID)
	require.NoError(t, err)

	// Write the archived task back to the queue to create a level-2 duplicate.
	archiveContent, err := os.ReadFile(record.ArchivePath)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(record.OriginalPath, archiveContent, 0o644))

	report, err := core.Doctor(ctx, ws, allDoctorChecks)
	require.NoError(t, err)

	for _, f := range report.Findings {
		if f.ArtifactID == task.ID && f.Type == core.FindingRootIDCollision {
			t.Errorf("level-2 task duplicate must NOT be reported as a root-ID collision")
		}
	}
}
