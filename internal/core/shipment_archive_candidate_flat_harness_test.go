package core

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/models"
)

func TestUArchiveCandidateFlat_UnlistedTerminalDescendantIsUntouched(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	feature, err := CreateArtifact(ctx, ws, "Archive candidate feature", "feature")
	require.NoError(t, err)
	unlisted, err := CreateArtifact(ctx, ws, "Unlisted done descendant", "task", WithParent(feature.ID))
	require.NoError(t, err)
	forceFlatScopeStatus(t, ws, unlisted.ID, models.StatusDone)
	shipment, err := CreateShipment(ctx, ws, "Archive candidate shipment", []string{feature.ID})
	require.NoError(t, err)
	_, err = ClaimShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)
	forceFlatScopeStatus(t, ws, unlisted.ID, models.StatusDone)

	result, err := ShipShipment(ctx, ws, shipment.ID, nil)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotContains(t, result.ArchivedIDs, unlisted.ID,
		"SCOPE-RED-D: unlisted terminal-but-not-archived descendant must not be an archive candidate")
	assert.Equal(t, models.StatusDone, flatScopeStatus(t, ws, unlisted.ID),
		"SCOPE-RED-D: unlisted terminal descendant must remain untouched")
}

func TestUArchiveCandidateFlat_UnlistedLinkedDeliberationIsUntouched(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	deliberation, err := CreateArtifact(ctx, ws, "Unlisted linked deliberation", "deliberation")
	require.NoError(t, err)
	feature, err := CreateArtifact(
		ctx,
		ws,
		"Linked deliberation feature",
		"feature",
		WithDescription("Origin: "+deliberation.ID),
	)
	require.NoError(t, err)
	shipment, err := CreateShipment(ctx, ws, "Linked deliberation shipment", []string{feature.ID})
	require.NoError(t, err)
	_, err = ClaimShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)

	result, err := ShipShipment(ctx, ws, shipment.ID, nil)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotContains(t, result.ArchivedIDs, deliberation.ID,
		"SCOPE-RED-D: unlisted linked deliberation must not be an archive candidate")
	assert.NotEqual(t, models.StatusArchived, flatScopeStatus(t, ws, deliberation.ID),
		"SCOPE-RED-D: unlisted linked deliberation must remain untouched")
}

func TestUArchiveCandidateFlat_ArchivedDescendantIsNeverRestored(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	feature, err := CreateArtifact(ctx, ws, "Archived control feature", "feature")
	require.NoError(t, err)
	archived, err := CreateArtifact(ctx, ws, "Archived control descendant", "task", WithParent(feature.ID))
	require.NoError(t, err)
	forceFlatScopeStatus(t, ws, archived.ID, models.StatusArchived)
	shipment, err := CreateShipment(ctx, ws, "Archived control shipment", []string{feature.ID})
	require.NoError(t, err)
	_, err = ClaimShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)
	forceFlatScopeStatus(t, ws, archived.ID, models.StatusArchived)

	result, err := ShipShipment(ctx, ws, shipment.ID, nil)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotContains(t, result.ArchivedIDs, archived.ID,
		"SCOPE-RED-D: already-archived descendant must not be archived again")
	assert.Equal(t, models.StatusArchived, flatScopeStatus(t, ws, archived.ID),
		"SCOPE-RED-D: archived descendant must never be restored to satisfy scope")
}
