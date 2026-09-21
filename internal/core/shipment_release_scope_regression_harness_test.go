package core

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/models"
)

func TestUReleaseScopeRegression_ProjectionExcludesUnlistedBlockedDescendant(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	feature, err := CreateArtifact(ctx, ws, "Projection feature", "feature")
	require.NoError(t, err)
	listed, err := CreateArtifact(ctx, ws, "Projection listed member", "task", WithParent(feature.ID))
	require.NoError(t, err)
	blocked, err := CreateArtifact(ctx, ws, "Projection blocked non-member", "task", WithParent(feature.ID))
	require.NoError(t, err)
	forceFlatScopeStatus(t, ws, blocked.ID, models.StatusBlocked)
	shipment, err := CreateShipment(ctx, ws, "Projection shipment", []string{feature.ID, listed.ID})
	require.NoError(t, err)

	composition, err := SizeComposition(ctx, ws, shipment)
	require.NoError(t, err)
	memberIDs := make([]string, 0, len(composition.Members))
	for _, member := range composition.Members {
		memberIDs = append(memberIDs, member.ID)
	}
	assert.ElementsMatch(t, []string{listed.ID}, memberIDs,
		"SCOPE-RED-B: size_composition.members must project explicit task members only")
	assert.NotContains(t, memberIDs, blocked.ID,
		"SCOPE-RED-B: blocked status must not make an unlisted descendant a member")
}

func TestUReleaseScopeRegression_ProjectionExcludesUnlistedArchivedDescendant(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	feature, err := CreateArtifact(ctx, ws, "Archived projection feature", "feature")
	require.NoError(t, err)
	listed, err := CreateArtifact(ctx, ws, "Archived projection listed", "task", WithParent(feature.ID))
	require.NoError(t, err)
	archived, err := CreateArtifact(ctx, ws, "Archived projection non-member", "task", WithParent(feature.ID))
	require.NoError(t, err)
	forceFlatScopeStatus(t, ws, archived.ID, models.StatusArchived)
	shipment, err := CreateShipment(ctx, ws, "Archived projection shipment", []string{feature.ID, listed.ID})
	require.NoError(t, err)

	composition, err := SizeComposition(ctx, ws, shipment)
	require.NoError(t, err)
	memberIDs := make([]string, 0, len(composition.Members))
	for _, member := range composition.Members {
		memberIDs = append(memberIDs, member.ID)
	}
	assert.ElementsMatch(t, []string{listed.ID}, memberIDs,
		"SCOPE-RED-B: archived descendants are excluded by non-membership, not restoration")
	assert.NotContains(t, memberIDs, archived.ID)
}

func TestUReleaseScopeRegression_FeatureOnlyShipFreesActiveSlotWithoutTouchingDescendants(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	feature, err := CreateArtifact(ctx, ws, "Feature-only release", "feature")
	require.NoError(t, err)
	unlisted, err := CreateArtifact(ctx, ws, "Feature-only unlisted child", "task", WithParent(feature.ID))
	require.NoError(t, err)
	shipment, err := CreateShipment(ctx, ws, "Feature-only shipment", []string{feature.ID})
	require.NoError(t, err)
	_, err = ClaimShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)
	forceFlatScopeStatus(t, ws, unlisted.ID, models.StatusQueued)

	result, err := ShipShipment(ctx, ws, shipment.ID, nil)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotContains(t, result.ArchivedIDs, unlisted.ID,
		"SCOPE-RED-B: feature-only manifest must not archive an unlisted descendant")
	assert.Equal(t, models.StatusQueued, flatScopeStatus(t, ws, unlisted.ID),
		"SCOPE-RED-B: feature-only closure must leave unlisted descendants untouched")

	next, err := CreateShipment(ctx, ws, "Next active-slot shipment", nil)
	require.NoError(t, err)
	_, err = ClaimShipment(ctx, ws, next.ID)
	assert.NoError(t, err,
		"SCOPE-RED-B: feature-only closure must free the single active slot")
}
