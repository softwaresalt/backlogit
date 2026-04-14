package core_test

// 025.013-T (Unit 2): Enforce WIT hierarchy constraints at creation time.
//
// Red tests (fail until Unit 2 is implemented):
//   - TestCreateArtifact_RejectsLevel2WithoutParent (no hierarchy check in CreateArtifact yet)
//   - TestLevelForType_NilLayout (panics on nil layout)
//
// Green tests (pass with current code):
//   - TestCreateArtifact_AcceptsLevel1WithoutParent
//   - TestCreateArtifact_AcceptsLevel2WithParent

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
	corerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// TestCreateArtifact_RejectsLevel2WithoutParent asserts that creating a task
// (hierarchy level 2) without a parent_id returns an ErrValidation-wrapped error.
// RED until Unit 2 adds the level check to validateArtifactParent / CreateArtifact.
func TestCreateArtifact_RejectsLevel2WithoutParent(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	_, err := core.CreateArtifact(ctx, ws, "Orphan task", "task") // no WithParent

	require.Error(t, err, "creating a level-2 task without parent_id must return an error")
	assert.ErrorIs(t, err, corerrors.ErrValidation, "error must wrap ErrValidation")
	assert.Contains(t, err.Error(), "requires parent_id", "error message must mention parent_id requirement")
}

// TestCreateArtifact_AcceptsLevel1WithoutParent asserts that creating a feature
// (hierarchy level 1) without a parent_id succeeds.
// GREEN with current code.
func TestCreateArtifact_AcceptsLevel1WithoutParent(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	artifact, err := core.CreateArtifact(ctx, ws, "Top-level feature", "feature")

	require.NoError(t, err, "creating a level-1 feature without parent_id must succeed")
	require.NotNil(t, artifact)
	assert.Equal(t, "feature", artifact.ArtifactType)
	assert.Empty(t, artifact.ParentID)
}

// TestCreateArtifact_AcceptsLevel2WithParent asserts that creating a task
// (hierarchy level 2) with a valid parent_id succeeds.
// GREEN with current code.
func TestCreateArtifact_AcceptsLevel2WithParent(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	feature, err := core.CreateArtifact(ctx, ws, "Parent feature", "feature")
	require.NoError(t, err)

	task, err := core.CreateArtifact(ctx, ws, "Child task", "task", core.WithParent(feature.ID))

	require.NoError(t, err, "creating a level-2 task with a valid parent_id must succeed")
	require.NotNil(t, task)
	assert.Equal(t, feature.ID, task.ParentID)
}

// TestLevelForType_NilLayout asserts that LevelForType returns (0, error) when
// the layout parameter is nil instead of panicking.
// RED until Unit 2 hardens LevelForType to handle nil safely.
func TestLevelForType_NilLayout(t *testing.T) {
	var result int
	var resultErr error

	assert.NotPanics(t, func() {
		result, resultErr = core.LevelForType((*config.QueueLayoutConfig)(nil), "task")
	}, "LevelForType with nil layout must not panic")

	assert.Equal(t, 0, result)
	assert.Error(t, resultErr, "LevelForType with nil layout must return a non-nil error")
}
