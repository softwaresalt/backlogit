package core

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/models"
)

func TestUReleaseScopeFlat_TaskOnlyManifestDoesNotExpandDescendants(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	feature, err := CreateArtifact(ctx, ws, "Flat task-only feature", "feature")
	require.NoError(t, err)
	task, err := CreateArtifact(ctx, ws, "Flat task-only member", "task", WithParent(feature.ID))
	require.NoError(t, err)
	subtask, err := CreateArtifact(ctx, ws, "Unlisted task descendant", "subtask", WithParent(task.ID))
	require.NoError(t, err)

	scope, err := releaseScopeItemIDs(ctx, ws, []string{task.ID})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{task.ID}, scope,
		"SCOPE-RED-A: task-only release scope must equal the explicit manifest")
	assert.NotContains(t, scope, subtask.ID,
		"SCOPE-RED-A: an unlisted descendant must not enter release scope")
}

func TestUReleaseScopeFlat_ListedFeatureDoesNotExpandBlockedSibling(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	feature, err := CreateArtifact(ctx, ws, "Flat feature member", "feature")
	require.NoError(t, err)
	listed, err := CreateArtifact(ctx, ws, "Listed feature child", "task", WithParent(feature.ID))
	require.NoError(t, err)
	blockedSibling, err := CreateArtifact(ctx, ws, "Unlisted blocked sibling", "task", WithParent(feature.ID))
	require.NoError(t, err)
	forceFlatScopeStatus(t, ws, blockedSibling.ID, models.StatusBlocked)

	scope, err := releaseScopeItemIDs(ctx, ws, []string{feature.ID, listed.ID})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{feature.ID, listed.ID}, scope,
		"SCOPE-RED-A: feature-plus-child scope must set-equal the flat manifest")
	assert.NotContains(t, scope, blockedSibling.ID,
		"SCOPE-RED-A: blocked status does not create implicit membership")
}

func TestUReleaseScopeFlat_ExplicitDescendantInclusionDoesNotExpandSiblings(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	feature, err := CreateArtifact(ctx, ws, "Explicit descendant feature", "feature")
	require.NoError(t, err)
	explicit, err := CreateArtifact(ctx, ws, "Explicit descendant", "task", WithParent(feature.ID))
	require.NoError(t, err)
	unlisted, err := CreateArtifact(ctx, ws, "Unlisted descendant", "task", WithParent(feature.ID))
	require.NoError(t, err)

	scope, err := releaseScopeItemIDs(ctx, ws, []string{feature.ID, explicit.ID})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{feature.ID, explicit.ID}, scope,
		"SCOPE-RED-A: explicit listing controls inclusion and must not expand siblings")
	assert.Contains(t, scope, explicit.ID)
	assert.NotContains(t, scope, unlisted.ID)
}
