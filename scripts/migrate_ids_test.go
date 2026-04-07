package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/models"
)

func TestMigrateArtifactID_IdempotentForLegacyAndCurrentFormats(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		artifactID   string
		artifactType string
		suffix       string
		want         string
	}{
		{
			name:         "legacy root feature",
			artifactID:   "F016",
			artifactType: "feature",
			suffix:       "-F",
			want:         "016-F",
		},
		{
			name:         "legacy child task",
			artifactID:   "F016.T002",
			artifactType: "task",
			suffix:       "-T",
			want:         "016.002-T",
		},
		{
			name:         "already migrated review",
			artifactID:   "013.001-R",
			artifactType: "review",
			suffix:       "-R",
			want:         "013.001-R",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := migrateArtifactID(tt.artifactID, tt.suffix)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRunMigration_RewritesArtifactsLogsAndArchivedFrom(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	workspaceDir := filepath.Join(root, ".backlogit")
	queueDir := filepath.Join(workspaceDir, "queue")
	archiveDir := filepath.Join(workspaceDir, "archive")
	logsDir := filepath.Join(root, "logs")

	require.NoError(t, os.MkdirAll(queueDir, 0o755))
	require.NoError(t, os.MkdirAll(archiveDir, 0o755))
	require.NoError(t, os.MkdirAll(logsDir, 0o755))

	writeWorkspaceConfig(t, workspaceDir)
	writeArtifact(t, filepath.Join(queueDir, "F016.md"), map[string]any{
		"id":            "F016",
		"title":         "Feature sixteen",
		"status":        "queued",
		"artifact_type": "feature",
	})
	writeArtifact(t, filepath.Join(queueDir, "F016.T001.md"), map[string]any{
		"id":            "F016.T001",
		"title":         "Task one",
		"status":        "queued",
		"artifact_type": "task",
		"parent_id":     "F016",
		"dependencies":  []string{"F016.T002"},
	})
	writeArtifact(t, filepath.Join(queueDir, "F016.T002.md"), map[string]any{
		"id":            "F016.T002",
		"title":         "Task two",
		"status":        "queued",
		"artifact_type": "task",
		"parent_id":     "F016",
	})
	writeArtifact(t, filepath.Join(archiveDir, "F013.R001-branch-review.md"), map[string]any{
		"id":            "F013.R001",
		"title":         "Branch review",
		"status":        "archived",
		"artifact_type": "review",
		"parent_id":     "F013",
		"archived_from": filepath.Join(".backlogit", "queue", "F013.R001-branch-review.md"),
	})
	require.NoError(t, os.WriteFile(
		filepath.Join(logsDir, "F016.T001.jsonl"),
		[]byte("{\"timestamp\":\"2026-04-07T00:00:00Z\",\"actor\":\"tester\",\"item_id\":\"F016.T001\",\"event_type\":\"comment\",\"delta\":{\"message\":\"migrate me\"}}\n"),
		0o644,
	))

	require.NoError(t, runMigration(root))

	assert.FileExists(t, filepath.Join(queueDir, "016-F.md"))
	assert.FileExists(t, filepath.Join(queueDir, "016.001-T.md"))
	assert.FileExists(t, filepath.Join(queueDir, "016.002-T.md"))
	assert.FileExists(t, filepath.Join(archiveDir, "013.001-R-branch-review.md"))
	assert.NoFileExists(t, filepath.Join(queueDir, "F016.md"))
	assert.NoFileExists(t, filepath.Join(queueDir, "F016.T001.md"))
	assert.NoFileExists(t, filepath.Join(logsDir, "F016.T001.jsonl"))
	assert.FileExists(t, filepath.Join(logsDir, "016.001-T.jsonl"))

	taskContent, err := os.ReadFile(filepath.Join(queueDir, "016.001-T.md"))
	require.NoError(t, err)
	taskFM, _, err := models.ParseFrontmatter(string(taskContent))
	require.NoError(t, err)
	assert.Equal(t, "016.001-T", taskFM["id"])
	assert.Equal(t, "016-F", taskFM["parent_id"])
	assert.Equal(t, []any{"016.002-T"}, taskFM["dependencies"])

	reviewContent, err := os.ReadFile(filepath.Join(archiveDir, "013.001-R-branch-review.md"))
	require.NoError(t, err)
	reviewFM, _, err := models.ParseFrontmatter(string(reviewContent))
	require.NoError(t, err)
	assert.Equal(
		t,
		filepath.Join(".backlogit", "queue", "013.001-R-branch-review.md"),
		reviewFM["archived_from"],
	)

	logContent, err := os.ReadFile(filepath.Join(logsDir, "016.001-T.jsonl"))
	require.NoError(t, err)
	assert.Contains(t, string(logContent), "\"item_id\":\"016.001-T\"")

	require.NoError(t, runMigration(root))
	assert.FileExists(t, filepath.Join(queueDir, "016.001-T.md"))
	assert.FileExists(t, filepath.Join(logsDir, "016.001-T.jsonl"))
}

func writeWorkspaceConfig(t *testing.T, workspaceDir string) {
	t.Helper()

	configYAML := `artifact_types:
  feature:
    prefix: "F"
    suffix: "-F"
    name_format: "{NNN}{suffix}"
  task:
    prefix: "T"
    suffix: "-T"
    name_format: "{NNN}{suffix}"
  subtask:
    prefix: "ST"
    suffix: "-ST"
    name_format: "{NNN}{suffix}"
  review:
    prefix: "R"
    suffix: "-R"
    name_format: "{NNN}{suffix}"
    file_name_format: "{id}-{title_slug}"
  deliberation:
    prefix: "DL"
    suffix: "-DL"
    name_format: "{NNN}{suffix}"
  shipment:
    prefix: "S"
    suffix: "-S"
    name_format: "{NNN}{suffix}"
max_slug_length: 60
bug_level: 3
queue_layout:
  root_dir: queue
  levels:
    - level: 1
      types: [feature, deliberation, shipment]
    - level: 2
      types: [task, review]
    - level: 3
      types: [subtask]
`
	require.NoError(t, os.WriteFile(filepath.Join(workspaceDir, "config.yaml"), []byte(configYAML), 0o644))
}

func writeArtifact(t *testing.T, path string, fm map[string]any) {
	t.Helper()

	require.NoError(t, os.WriteFile(path, []byte(models.SerializeFrontmatter(fm, "body")), 0o644))
}
