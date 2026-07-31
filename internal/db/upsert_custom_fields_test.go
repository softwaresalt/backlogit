package db_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/models"
)

func TestUpsertItem_ProjectsModeledCustomFields(t *testing.T) {
	ctx := context.Background()
	database := setupProjectionDB(t)
	artifact := projectionArtifact("951.001-T", map[string]any{
		"complexity": "high",
		"size":       "M",
		"unmodeled":  "kept-in-json-only",
	})

	require.NoError(t, db.UpsertItem(ctx, database, artifact))

	row := database.QueryRow(`SELECT complexity, size, custom_fields FROM items WHERE id = ?`, artifact.ID)
	var complexity, size sql.NullString
	var customFields string
	require.NoError(t, row.Scan(&complexity, &size, &customFields))

	assert.Equal(t, "high", complexity.String)
	assert.True(t, complexity.Valid)
	assert.Equal(t, "M", size.String)
	assert.True(t, size.Valid)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal([]byte(customFields), &decoded))
	assert.Equal(t, "kept-in-json-only", decoded["unmodeled"])
}

func TestUpsertItem_DoesNotProjectComplexityForNonTasks(t *testing.T) {
	ctx := context.Background()
	database := setupProjectionDB(t)
	artifact := projectionArtifact("951-F", map[string]any{"complexity": "high"})
	artifact.ArtifactType = "feature"
	artifact.ParentID = ""
	artifact.Level = 1
	artifact.HierarchyPath = "951"

	require.NoError(t, db.UpsertItem(ctx, database, artifact))

	row := database.QueryRow(`SELECT complexity, custom_fields FROM items WHERE id = ?`, artifact.ID)
	var complexity sql.NullString
	var customFields string
	require.NoError(t, row.Scan(&complexity, &customFields))

	assert.False(t, complexity.Valid, "non-task complexity must stay out of the query projection")
	var decoded map[string]any
	require.NoError(t, json.Unmarshal([]byte(customFields), &decoded))
	assert.Equal(t, "high", decoded["complexity"], "custom field payload is preserved for compatibility")
}

func TestUpsertItemsTx_ProjectsModeledCustomFields(t *testing.T) {
	ctx := context.Background()
	database := setupProjectionDB(t)
	tx, err := database.BeginTx(ctx, nil)
	require.NoError(t, err)

	artifact := projectionArtifact("952.001-T", map[string]any{"complexity": "low"})
	require.NoError(t, db.UpsertItemsTx(ctx, tx, artifact))
	require.NoError(t, tx.Commit())

	row := database.QueryRow(`SELECT complexity FROM items WHERE id = ?`, artifact.ID)
	var complexity sql.NullString
	require.NoError(t, row.Scan(&complexity))
	assert.Equal(t, "low", complexity.String)
	assert.True(t, complexity.Valid)
}

func TestUpsertItem_ProjectsClearAsNull(t *testing.T) {
	ctx := context.Background()
	database := setupProjectionDB(t)
	artifact := projectionArtifact("953.001-T", map[string]any{"complexity": "high"})
	require.NoError(t, db.UpsertItem(ctx, database, artifact))

	artifact.CustomFields = map[string]any{}
	require.NoError(t, db.UpsertItem(ctx, database, artifact))

	row := database.QueryRow(`SELECT complexity FROM items WHERE id = ?`, artifact.ID)
	var complexity sql.NullString
	require.NoError(t, row.Scan(&complexity))
	assert.False(t, complexity.Valid)
}

func TestUpsertItem_RejectsMalformedProjectedColumnName(t *testing.T) {
	ctx := context.Background()
	database := setupProjectionDB(t)
	_, err := database.Exec(`ALTER TABLE items ADD COLUMN "bad-name" TEXT`)
	require.NoError(t, err)

	artifact := projectionArtifact("954.001-T", map[string]any{"bad-name": "value"})
	err = db.UpsertItem(ctx, database, artifact)
	require.Error(t, err)
	assert.ErrorContains(t, err, "invalid column name")
}

func setupProjectionDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, config.WriteDefaults(dir))
	headerDef, err := config.LoadHeaderDef(dir)
	require.NoError(t, err)

	dbPath := filepath.Join(t.TempDir(), "projection.db")
	database, err := db.Open(dbPath)
	require.NoError(t, err)
	require.NoError(t, db.EnsureSchemaWithExtensions(database, headerDef))
	t.Cleanup(func() { database.Close() })
	return database
}

func projectionArtifact(id string, customFields map[string]any) *models.Artifact {
	now := time.Date(2026, 7, 30, 20, 0, 0, 0, time.UTC)
	return &models.Artifact{
		ID:            id,
		Title:         "Projection task",
		Status:        models.StatusActive,
		ArtifactType:  "task",
		ParentID:      "951-F",
		Priority:      "medium",
		CustomFields:  customFields,
		CreatedAt:     now,
		UpdatedAt:     now,
		Level:         2,
		HierarchyPath: "951.001",
	}
}
