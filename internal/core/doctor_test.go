package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/models"
)

// helperWriteArtifact writes a minimal markdown artifact to dir/filename.
func helperWriteArtifact(t *testing.T, dir, filename, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, filename), []byte(content), 0o644))
}

// newDoctorTestWorkspace creates a Workspace with standard queue layout config.
// When withConfig is true, it sets Workspace.Config so tests exercise the
// config-driven artifact search path. Pass nil Config to use the fallback
// "scan all dirs" path.
func newDoctorTestWorkspace(t *testing.T, rootPath string, withConfig bool) *Workspace {
	t.Helper()
	ws := &Workspace{RootPath: rootPath}
	if withConfig {
		ws.Config = &config.WorkspaceConfig{
			ArtifactTypes: map[string]*config.ArtifactTypeConfig{
				"feature": {Prefix: "F", NameFormat: "{id}-{prefix}"},
				"task":    {Prefix: "T", NameFormat: "{id}-{prefix}"},
				"subtask": {Prefix: "ST", NameFormat: "{id}-{prefix}"},
			},
			MaxSlugLength: 60,
			QueueLayout: &config.QueueLayoutConfig{
				RootDir: "queue",
				Levels: []config.HierarchyLevel{
					{Level: 1, Types: []string{"feature", "deliberation", "shipment"}},
					{Level: 2, Types: []string{"task", "review"}},
					{Level: 3, Types: []string{"subtask"}},
				},
			},
		}
	}
	return ws
}

func TestDoctor_LegacyRootTaskNotFlaggedAsOrphan(t *testing.T) {
	tmp := t.TempDir()
	wsRoot := filepath.Join(tmp, ".backlogit")
	archiveDir := filepath.Join(wsRoot, "archive")
	require.NoError(t, os.MkdirAll(filepath.Join(wsRoot, "queue"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(wsRoot, "logs"), 0o755))

	// Legacy task at root level (level 1) with no parent — should NOT be flagged.
	helperWriteArtifact(t, archiveDir, "001-T.md", `---
id: "001-T"
title: "Legacy root task"
status: done
artifact_type: task
level: 1
hierarchy_path: "001"
---
Legacy task.
`)

	// Legacy bug at root level — type not in queue layout, no level field.
	helperWriteArtifact(t, archiveDir, "001-B.md", `---
id: "001-B"
title: "Legacy root bug"
status: done
artifact_type: bug
---
Legacy bug.
`)

	// Use a configured workspace and verify these legacy root-level artifacts are not flagged as orphans.
	ws := newDoctorTestWorkspace(t, tmp, true)

	report, err := Doctor(context.Background(), ws, &DoctorOptions{CheckOrphans: true})
	require.NoError(t, err)

	for _, f := range report.Findings {
		if f.Type == FindingOrphanedArtifact {
			t.Errorf("unexpected orphan finding: %s (%s)", f.ArtifactID, f.Description)
		}
	}
}

func TestDoctor_RealOrphanedChildTaskStillFlagged(t *testing.T) {
	tmp := t.TempDir()
	wsRoot := filepath.Join(tmp, ".backlogit")
	queueDir := filepath.Join(wsRoot, "queue")
	require.NoError(t, os.MkdirAll(filepath.Join(wsRoot, "logs"), 0o755))

	// Genuine orphaned child task at level 2 — SHOULD be flagged.
	helperWriteArtifact(t, queueDir, "099.001-T.md", `---
id: "099.001-T"
title: "Orphaned child task"
status: active
artifact_type: task
level: 2
hierarchy_path: "099/099.001"
---
This task has no parent.
`)

	ws := newDoctorTestWorkspace(t, tmp, true)

	report, err := Doctor(context.Background(), ws, &DoctorOptions{CheckOrphans: true})
	require.NoError(t, err)

	var orphanIDs []string
	for _, f := range report.Findings {
		if f.Type == FindingOrphanedArtifact {
			orphanIDs = append(orphanIDs, f.ArtifactID)
		}
	}
	assert.Contains(t, orphanIDs, "099.001-T", "real orphaned child task should be flagged")
}

func TestDoctor_ReturnedToBacklogEventStillIgnored(t *testing.T) {
	tmp := t.TempDir()
	wsRoot := filepath.Join(tmp, ".backlogit")
	queueDir := filepath.Join(wsRoot, "queue")
	logsDir := filepath.Join(wsRoot, "logs")
	require.NoError(t, os.MkdirAll(logsDir, 0o755))

	// Task at level 2 with no parent but has returned_to_backlog event.
	helperWriteArtifact(t, queueDir, "050.001-T.md", `---
id: "050.001-T"
title: "Returned task"
status: active
artifact_type: task
level: 2
hierarchy_path: "050/050.001"
---
Returned to backlog.
`)

	logContent := `{"event_type":"returned_to_backlog","timestamp":"2026-04-20T00:00:00Z"}` + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(logsDir, "050.001-T.jsonl"), []byte(logContent), 0o644))

	ws := newDoctorTestWorkspace(t, tmp, true)

	report, err := Doctor(context.Background(), ws, &DoctorOptions{CheckOrphans: true})
	require.NoError(t, err)

	for _, f := range report.Findings {
		if f.Type == FindingOrphanedArtifact && f.ArtifactID == "050.001-T" {
			t.Error("returned_to_backlog task should not be flagged as orphan")
		}
	}
}

func TestDoctor_FixOrphansArchivesOrphanedTask(t *testing.T) {
	tmp := t.TempDir()
	wsRoot := filepath.Join(tmp, ".backlogit")
	queueDir := filepath.Join(wsRoot, "queue")
	require.NoError(t, os.MkdirAll(filepath.Join(wsRoot, "logs"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(wsRoot, "archive"), 0o755))

	// Genuine orphaned child task at level 2 — should be archived.
	helperWriteArtifact(t, queueDir, "099.001-T.md", `---
id: "099.001-T"
title: "Orphaned child task"
status: active
artifact_type: task
level: 2
hierarchy_path: "099/099.001"
---
This task has no parent.
`)

	ws := newDoctorTestWorkspace(t, tmp, true)

	dbPath := filepath.Join(wsRoot, "backlogit.db")
	database, err := db.Open(dbPath)
	require.NoError(t, err)
	require.NoError(t, db.EnsureSchema(database))
	t.Cleanup(func() { database.Close() })
	ws.DB = database

	// Index the artifact.
	a := &models.Artifact{
		ID: "099.001-T", Title: "Orphaned child task", Status: models.StatusActive,
		ArtifactType: "task", Level: 2,
	}
	require.NoError(t, db.UpsertItem(context.Background(), database, a))

	report, err := Doctor(context.Background(), ws, &DoctorOptions{
		CheckOrphans: true,
		FixOrphans:   true,
	})
	require.NoError(t, err)

	// Should have the finding AND a fix action.
	assert.Len(t, report.Findings, 1)
	assert.Len(t, report.FixActions, 1)
	assert.Equal(t, "099.001-T", report.FixActions[0].ArtifactID)
	assert.Equal(t, FixArchived, report.FixActions[0].Type)

	// File should be in archive/, not queue/.
	assert.NoFileExists(t, filepath.Join(queueDir, "099.001-T.md"))
	assert.FileExists(t, filepath.Join(wsRoot, "archive", "099.001-T.md"))
}

func TestDoctor_FixOrphansSkipsReturnedToBacklog(t *testing.T) {
	tmp := t.TempDir()
	wsRoot := filepath.Join(tmp, ".backlogit")
	queueDir := filepath.Join(wsRoot, "queue")
	logsDir := filepath.Join(wsRoot, "logs")
	require.NoError(t, os.MkdirAll(logsDir, 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(wsRoot, "archive"), 0o755))

	// Task with returned_to_backlog event — should NOT be fixed.
	helperWriteArtifact(t, queueDir, "050.001-T.md", `---
id: "050.001-T"
title: "Returned task"
status: active
artifact_type: task
level: 2
hierarchy_path: "050/050.001"
---
Returned to backlog.
`)
	logContent := `{"event_type":"returned_to_backlog","timestamp":"2026-04-20T00:00:00Z"}` + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(logsDir, "050.001-T.jsonl"), []byte(logContent), 0o644))

	ws := newDoctorTestWorkspace(t, tmp, true)

	dbPath := filepath.Join(wsRoot, "backlogit.db")
	database, err := db.Open(dbPath)
	require.NoError(t, err)
	require.NoError(t, db.EnsureSchema(database))
	t.Cleanup(func() { database.Close() })
	ws.DB = database

	report, err := Doctor(context.Background(), ws, &DoctorOptions{
		CheckOrphans: true,
		FixOrphans:   true,
	})
	require.NoError(t, err)

	assert.Empty(t, report.Findings, "returned task should not appear as orphan")
	assert.Empty(t, report.FixActions, "no fix actions for returned task")
	assert.FileExists(t, filepath.Join(queueDir, "050.001-T.md"), "file should remain in queue")
}

func TestDoctor_ReportOnlyByDefault(t *testing.T) {
	tmp := t.TempDir()
	wsRoot := filepath.Join(tmp, ".backlogit")
	queueDir := filepath.Join(wsRoot, "queue")
	require.NoError(t, os.MkdirAll(filepath.Join(wsRoot, "logs"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(wsRoot, "archive"), 0o755))

	helperWriteArtifact(t, queueDir, "099.001-T.md", `---
id: "099.001-T"
title: "Orphaned child task"
status: active
artifact_type: task
level: 2
hierarchy_path: "099/099.001"
---
This task has no parent.
`)

	ws := newDoctorTestWorkspace(t, tmp, true)

	report, err := Doctor(context.Background(), ws, &DoctorOptions{
		CheckOrphans: true,
	})
	require.NoError(t, err)

	assert.Len(t, report.Findings, 1, "orphan should be reported")
	assert.Empty(t, report.FixActions, "no fix actions when fix-orphans is false")
	assert.FileExists(t, filepath.Join(queueDir, "099.001-T.md"), "file should remain in queue")
}
