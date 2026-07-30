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
	"github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/mdfront"
	"github.com/softwaresalt/backlogit/internal/models"
)

const goldenComplexityTaskFile = "---\n" +
	"artifact_type: task\n" +
	"created_at: 2026-07-30T20:00:00Z\n" +
	"id: 901.001-T\n" +
	"parent_id: 901-F\n" +
	"priority: medium\n" +
	"status: active\n" +
	"title: Complexity body task\n" +
	"updated_at: 2026-07-30T20:00:00Z\n" +
	"---\n" +
	"\n" +
	"# Body\n" +
	"\n" +
	"Body bytes must stay untouched.   \n" +
	"* one\n" +
	"* two\n"

func setupComplexityWorkspace(t *testing.T) (ws *core.Workspace, id, path string) {
	t.Helper()
	ctx := context.Background()
	root := t.TempDir()
	backlogitDir := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(filepath.Join(backlogitDir, "queue"), 0o755))
	require.NoError(t, config.WriteDefaults(backlogitDir))

	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ws.Close() })

	path = filepath.Join(backlogitDir, "queue", "901.001-T.md")
	require.NoError(t, os.WriteFile(path, []byte(goldenComplexityTaskFile), 0o644))

	art := &models.Artifact{
		ID:           "901.001-T",
		Title:        "Complexity body task",
		Status:       models.StatusActive,
		ArtifactType: "task",
		ParentID:     "901-F",
		Priority:     "medium",
	}
	require.NoError(t, db.UpsertItem(ctx, ws.DB, art))
	return ws, "901.001-T", path
}

func TestSetArtifactComplexity_PersistsAndPreservesBody(t *testing.T) {
	ctx := context.Background()
	ws, id, path := setupComplexityWorkspace(t)

	rawBefore, err := os.ReadFile(path)
	require.NoError(t, err)
	mdBefore, err := mdfront.Decode(rawBefore)
	require.NoError(t, err)

	before, err := db.GetItem(ctx, ws.DB, id)
	require.NoError(t, err)

	_, err = core.SetArtifactComplexity(ctx, ws, id, "high")
	require.NoError(t, err)

	rawAfter, err := os.ReadFile(path)
	require.NoError(t, err)
	mdAfter, err := mdfront.Decode(rawAfter)
	require.NoError(t, err)

	assert.Equal(t, mdBefore.Body, mdAfter.Body, "body must be preserved byte-for-byte")
	cf, ok := mdAfter.Frontmatter["custom_fields"].(map[string]any)
	require.True(t, ok, "custom_fields map must be present")
	assert.Equal(t, "high", cf["complexity"])

	after, err := db.GetItem(ctx, ws.DB, id)
	require.NoError(t, err)
	assert.Equal(t, before.Title, after.Title)
	assert.Equal(t, before.Status, after.Status)
	assert.Equal(t, before.Priority, after.Priority)
	assert.Equal(t, "high", after.CustomFields["complexity"])
}

func TestSetArtifactComplexity_RejectsInvalidValueBeforeWrite(t *testing.T) {
	ctx := context.Background()
	ws, id, path := setupComplexityWorkspace(t)

	rawBefore, err := os.ReadFile(path)
	require.NoError(t, err)

	_, err = core.SetArtifactComplexity(ctx, ws, id, "extreme")
	require.Error(t, err)

	rawAfter, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, rawBefore, rawAfter, "invalid complexity must not write")
}

func TestSetArtifactComplexity_EmptyClearsField(t *testing.T) {
	ctx := context.Background()
	ws, id, path := setupComplexityWorkspace(t)

	_, err := core.SetArtifactComplexity(ctx, ws, id, "low")
	require.NoError(t, err)

	_, err = core.SetArtifactComplexity(ctx, ws, id, "")
	require.NoError(t, err)

	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	md, err := mdfront.Decode(raw)
	require.NoError(t, err)
	cf, _ := md.Frontmatter["custom_fields"].(map[string]any)
	assert.NotContains(t, cf, "complexity")
}

func TestSetArtifactComplexity_GenericUpdatePreservesComplexity(t *testing.T) {
	ctx := context.Background()
	ws, id, _ := setupComplexityWorkspace(t)

	_, err := core.SetArtifactComplexity(ctx, ws, id, "high")
	require.NoError(t, err)

	updated, err := core.UpdateArtifact(ctx, ws, id, map[string]any{
		"custom_fields": map[string]any{
			"complexity":     "low",
			"harness_status": "passing",
		},
	})
	require.NoError(t, err)

	assert.Equal(t, "high", updated.CustomFields["complexity"])
	assert.Equal(t, "passing", updated.CustomFields["harness_status"])
}

func TestSetArtifactComplexity_GenericCreateRejectsComplexity(t *testing.T) {
	ctx := context.Background()
	ws, _, _ := setupComplexityWorkspace(t)

	_, err := core.CreateArtifact(ctx, ws, "Off seam complexity", "feature", core.WithFields(map[string]any{
		"complexity": "high",
	}))
	require.Error(t, err)
	assert.ErrorContains(t, err, "reserved")
}
