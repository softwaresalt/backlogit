package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
)

// helperWriteArtifact writes a minimal markdown artifact to dir/filename.
func helperWriteArtifact(t *testing.T, dir, filename, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, filename), []byte(content), 0o644))
}

// newDoctorTestWorkspace creates a Workspace with standard queue layout config.
// Config is set so artifactSearchDirs uses Config-based scanning (requires
// registry.yaml) — pass nil Config to use the fallback "scan all dirs" path.
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

	// Legacy bug at root level — type not even in queue layout but level 1.
	helperWriteArtifact(t, archiveDir, "001-B.md", `---
id: "001-B"
title: "Legacy root bug"
status: done
artifact_type: bug
level: 1
hierarchy_path: "001"
---
Legacy bug.
`)

	// Config is nil so artifactSearchDirs scans all non-hidden dirs.
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
