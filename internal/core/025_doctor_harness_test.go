package core_test

// 025.016-T (Unit 4): Doctor workspace integrity diagnostics.
// All tests in this file are RED until Doctor is implemented (it panics).
//
// Tests:
//   - TestDoctor_DetectsOrphanedTask          — RED: Doctor panics
//   - TestDoctor_IgnoresIntentionalOrphans    — RED: Doctor panics
//   - TestDoctor_DetectsDuplicateAcrossQueueArchive — RED: Doctor panics
//   - TestDoctor_CleanWorkspaceNoFindings     — RED: Doctor panics
//   - TestDoctor_NilLayoutDoesNotPanic        — RED: Doctor panics

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/core"
	bldb "github.com/backlogit/backlogit/internal/db"
)

var allDoctorChecks = &core.DoctorOptions{
	CheckOrphans:    true,
	CheckDuplicates: true,
}

// TestDoctor_DetectsOrphanedTask verifies that a level-2 task with no parent_id
// and no returned_to_backlog event is reported as FindingOrphanedArtifact.
func TestDoctor_DetectsOrphanedTask(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	// Create a feature, then create a task under it, then manually remove the
	// parent_id from the task file to simulate corruption/orphaning.
	feature, err := core.CreateArtifact(ctx, ws, "Orphan parent feature", "feature")
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, feature))

	task, err := core.CreateArtifact(ctx, ws, "Orphaned task", "task", core.WithParent(feature.ID))
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, task))

	// Strip parent_id from the task's Markdown file to simulate corruption.
	taskPath, err := core.FindArtifactPath(ctx, ws, task.ID)
	require.NoError(t, err)
	content, err := os.ReadFile(taskPath)
	require.NoError(t, err)
	stripped := removeFrontmatterField(content, "parent_id")
	require.NoError(t, os.WriteFile(taskPath, stripped, 0o644))

	// Act
	report, err := core.Doctor(ctx, ws, allDoctorChecks)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, report)
	assert.NotEmpty(t, report.Findings, "Doctor must report the orphaned task")
	assert.Equal(t, core.FindingOrphanedArtifact, report.Findings[0].Type)
	assert.Equal(t, task.ID, report.Findings[0].ArtifactID)
}

// TestDoctor_IgnoresIntentionalOrphans verifies that a task returned from a shipment
// (parent_id cleared by ShipShipment via returnUnreleasedFeatureItems) is NOT
// reported as an orphan. The presence of a returned_to_backlog event log entry
// distinguishes intentional from accidental orphans.
func TestDoctor_IgnoresIntentionalOrphans(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	// Create feature and two tasks: only one will be in the release scope.
	feature, err := core.CreateArtifact(ctx, ws, "Intentional orphan feature", "feature")
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, feature))

	releasedTask, err := core.CreateArtifact(ctx, ws, "Released task", "task", core.WithParent(feature.ID))
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, releasedTask))

	futureTask, err := core.CreateArtifact(ctx, ws, "Future task (will be returned)", "task", core.WithParent(feature.ID))
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, futureTask))

	// Ship only releasedTask — futureTask gets returned with parent_id cleared.
	shipment, err := core.CreateShipment(ctx, ws, "Partial release", []string{releasedTask.ID})
	require.NoError(t, err)
	_, err = core.ClaimShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)
	_, err = core.ShipShipment(ctx, ws, shipment.ID, nil)
	require.NoError(t, err)

	// Act
	report, err := core.Doctor(ctx, ws, allDoctorChecks)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, report)
	for _, f := range report.Findings {
		assert.NotEqual(t, futureTask.ID, f.ArtifactID,
			"returned task with returned_to_backlog event must not be flagged as an orphan")
	}
}

// TestDoctor_DetectsDuplicateAcrossQueueArchive verifies that an artifact ID
// appearing in both the queue and archive directories is reported as FindingDuplicateID.
func TestDoctor_DetectsDuplicateAcrossQueueArchive(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	// Create and archive a task.
	feature, err := core.CreateArtifact(ctx, ws, "Duplicate test feature", "feature")
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, feature))

	task, err := core.CreateArtifact(ctx, ws, "Task to duplicate", "task", core.WithParent(feature.ID))
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, task))

	record, err := core.ArchiveItem(ctx, ws.DB, ws, task.ID)
	require.NoError(t, err)

	// Manually write the task file back to queue (simulating stale state).
	queuePath := record.OriginalPath
	archivePath := record.ArchivePath
	archiveContent, err := os.ReadFile(archivePath)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(queuePath, archiveContent, 0o644))

	// Act
	report, err := core.Doctor(ctx, ws, allDoctorChecks)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, report)
	assert.NotEmpty(t, report.Findings, "Doctor must detect the duplicate ID")
	found := false
	for _, f := range report.Findings {
		if f.Type == core.FindingDuplicateID && f.ArtifactID == task.ID {
			found = true
			break
		}
	}
	assert.True(t, found, "Doctor must report FindingDuplicateID for the duplicated artifact")
}

// TestDoctor_CleanWorkspaceNoFindings verifies that a workspace with no structural
// issues produces a DoctorReport with an empty Findings slice.
func TestDoctor_CleanWorkspaceNoFindings(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	// Create a feature+task with parent to produce a clean state.
	feature, err := core.CreateArtifact(ctx, ws, "Clean feature", "feature")
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, feature))

	task, err := core.CreateArtifact(ctx, ws, "Clean task", "task", core.WithParent(feature.ID))
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, task))

	// Act
	report, err := core.Doctor(ctx, ws, allDoctorChecks)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, report)
	assert.Empty(t, report.Findings, "a clean workspace must yield zero findings")
}

// TestDoctor_NilLayoutDoesNotPanic verifies that Doctor does not panic when the
// workspace has a nil queue layout configuration.
func TestDoctor_NilLayoutDoesNotPanic(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	assert.NotPanics(t, func() {
		_, _ = core.Doctor(ctx, ws, allDoctorChecks)
	}, "Doctor must not panic even when run on a workspace with nil layout")

	// Verify Doctor actually produces a usable result (not just avoids panicking).
	// RED until Doctor is fully implemented.
	report, err := core.Doctor(ctx, ws, allDoctorChecks)
	require.NoError(t, err, "Doctor must return a nil error on a valid workspace")
	require.NotNil(t, report, "Doctor must return a non-nil DoctorReport")
}

// removeFrontmatterField removes a key: value line from YAML frontmatter bytes.
// This is a test helper only — do not use in production code.
func removeFrontmatterField(content []byte, key string) []byte {
	lines := splitLines(string(content))
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		prefix := key + ":"
		if len(line) >= len(prefix) && line[:len(prefix)] == prefix {
			continue
		}
		result = append(result, line)
	}
	return []byte(joinLines(result))
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i+1])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func joinLines(lines []string) string {
	result := ""
	for _, l := range lines {
		result += l
	}
	return result
}

// findFileWithPrefix searches dir for any file whose base name starts with prefix.
//
//nolint:unused // used in pending TestDoctor tests
func findFileWithPrefix(t *testing.T, dir, prefix string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, e := range entries {
		if !e.IsDir() && len(e.Name()) >= len(prefix) && e.Name()[:len(prefix)] == prefix {
			return filepath.Join(dir, e.Name())
		}
	}
	return ""
}
