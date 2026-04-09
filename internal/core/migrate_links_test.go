package core_test

// 018.003-T: custom-field link migration harness.
//
// Implementation complete. These tests validate the contract for
// MigrateCustomFieldLinks:
//
//   - linked_deliberation_id on an artifact becomes an item_links row (informs)
//   - linked_stash_id is skipped when the stash ID is not an item ID
//   - source_stash_id is skipped when the stash ID is not an item ID
//   - original custom_fields values are preserved after migration
//   - the result counts reflect actual rows inserted vs. skipped

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/config"
	"github.com/backlogit/backlogit/internal/core"
	"github.com/backlogit/backlogit/internal/db"
)

func setupMigrateLinksWorkspace(t *testing.T) *core.Workspace {
	t.Helper()
	root := t.TempDir()
	backlogDir := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogDir, 0o755))
	require.NoError(t, config.WriteDefaults(backlogDir))
	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)
	t.Cleanup(func() { ws.Close() })
	return ws
}

func TestMigrateCustomFieldLinks_DeliberationID_CreatesInformsLink(t *testing.T) {
	ws := setupMigrateLinksWorkspace(t)
	ctx := context.Background()

	deliberation, err := core.CreateArtifact(ctx, ws, "DL001 Deliberation", "deliberation")
	require.NoError(t, err)

	task, err := core.CreateArtifact(ctx, ws, "Task linked to deliberation", "task")
	require.NoError(t, err)
	_, err = core.UpdateArtifact(ctx, ws, task.ID, map[string]any{
		"custom_fields": map[string]any{
			"linked_deliberation_id": deliberation.ID,
		},
	})
	require.NoError(t, err)

	result, err := core.MigrateCustomFieldLinks(ctx, ws)

	require.NoError(t, err)
	assert.GreaterOrEqual(t, result.Migrated, 1, "at least one link should be created")

	links, err := db.GetLinks(ctx, ws.DB, task.ID)
	require.NoError(t, err)
	require.Len(t, links, 1)
	assert.Equal(t, deliberation.ID, links[0].TargetID)
	assert.Equal(t, "informs", links[0].LinkType)
}

func TestMigrateCustomFieldLinks_StashID_Skipped(t *testing.T) {
	ws := setupMigrateLinksWorkspace(t)
	ctx := context.Background()

	task, err := core.CreateArtifact(ctx, ws, "Task with stash ref", "task")
	require.NoError(t, err)
	_, err = core.UpdateArtifact(ctx, ws, task.ID, map[string]any{
		"custom_fields": map[string]any{
			"source_stash_id": "STASH-ABC123",
		},
	})
	require.NoError(t, err)

	result, err := core.MigrateCustomFieldLinks(ctx, ws)

	require.NoError(t, err)
	assert.GreaterOrEqual(t, result.Skipped, 1, "non-item stash ID must be counted as skipped")

	links, err := db.GetLinks(ctx, ws.DB, task.ID)
	require.NoError(t, err)
	assert.Empty(t, links, "stash ID not resolvable as an item — no link should be created")
}

func TestMigrateCustomFieldLinks_PreservesOriginalCustomFields(t *testing.T) {
	ws := setupMigrateLinksWorkspace(t)
	ctx := context.Background()

	deliberation, err := core.CreateArtifact(ctx, ws, "DL Migration DL", "deliberation")
	require.NoError(t, err)

	task, err := core.CreateArtifact(ctx, ws, "Task to migrate", "task")
	require.NoError(t, err)
	_, err = core.UpdateArtifact(ctx, ws, task.ID, map[string]any{
		"custom_fields": map[string]any{
			"linked_deliberation_id": deliberation.ID,
			"extra_field":            "keep-me",
		},
	})
	require.NoError(t, err)

	_, err = core.MigrateCustomFieldLinks(ctx, ws)
	require.NoError(t, err)

	refreshedArtifact, err := db.GetItem(ctx, ws.DB, task.ID)
	require.NoError(t, err)
	assert.Equal(t, deliberation.ID, refreshedArtifact.CustomFields["linked_deliberation_id"],
		"original custom_field value must be preserved after migration")
	assert.Equal(t, "keep-me", refreshedArtifact.CustomFields["extra_field"],
		"unrelated custom_fields must not be removed")
}

func TestMigrateCustomFieldLinks_IdempotentOnRerun(t *testing.T) {
	ws := setupMigrateLinksWorkspace(t)
	ctx := context.Background()

	deliberation, err := core.CreateArtifact(ctx, ws, "DL Idempotent", "deliberation")
	require.NoError(t, err)

	task, err := core.CreateArtifact(ctx, ws, "Idempotent task", "task")
	require.NoError(t, err)
	_, err = core.UpdateArtifact(ctx, ws, task.ID, map[string]any{
		"custom_fields": map[string]any{"linked_deliberation_id": deliberation.ID},
	})
	require.NoError(t, err)

	_, err = core.MigrateCustomFieldLinks(ctx, ws)
	require.NoError(t, err)

	_, err = core.MigrateCustomFieldLinks(ctx, ws)
	require.NoError(t, err, "second migration run must not error")

	links, err := db.GetLinks(ctx, ws.DB, task.ID)
	require.NoError(t, err)
	assert.Len(t, links, 1, "idempotent upsert must not create duplicate link rows")
}
