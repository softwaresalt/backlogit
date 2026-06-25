package core_test

// 066.002-T (Unit 2) harness: CreateArtifact pre-write canonical uniqueness
// guard. RED until CreateArtifact fails loud with ErrIDCollision when the
// resolved ID already exists as a canonical file.

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
	bldb "github.com/softwaresalt/backlogit/internal/db"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

func TestCreateArtifact_RefusesCanonicalIDCollision(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	// Create a feature and archive it: archive/<id>.md now exists on disk.
	feature, err := core.CreateArtifact(ctx, ws, "First feature", "feature")
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, feature))

	_, err = core.ArchiveItem(ctx, ws.DB, ws, feature.ID)
	require.NoError(t, err)

	// Simulate a stale index (the 066-F window): the allocator no longer sees the
	// archived ordinal, so it re-mints the same ID — which still exists on disk.
	_, err = ws.DB.ExecContext(ctx, "DELETE FROM items WHERE id = ?", feature.ID)
	require.NoError(t, err)

	_, err = core.CreateArtifact(ctx, ws, "Colliding feature", "feature")
	require.Error(t, err, "create must not silently reuse an ID that exists on the canonical filesystem")
	assert.True(t, errors.Is(err, blerrors.ErrIDCollision),
		"create must fail loud with ErrIDCollision; got: %v", err)
}

func TestCreateArtifact_NoCollisionPathUnchanged(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	f1, err := core.CreateArtifact(ctx, ws, "Feature one", "feature")
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, f1))

	// Normal sequential allocation must remain collision-free and advance.
	f2, err := core.CreateArtifact(ctx, ws, "Feature two", "feature")
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, f2))

	assert.NotEqual(t, f1.ID, f2.ID, "distinct sequential creates must produce distinct IDs")

	task, err := core.CreateArtifact(ctx, ws, "Child task", "task", core.WithParent(f1.ID))
	require.NoError(t, err, "creating a child under an existing parent must not be treated as a collision")
	assert.NotEmpty(t, task.ID)
}
