package core_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/models"
)

// TestU2_CoreIsTaskRouteEligibleAfterValueOnlyComplexityUpdate proves the
// route-eligibility predicate drops after a value-only complexity update
// clears provenance on a previously provenance-complete task (207.002-T).
func TestU2_CoreIsTaskRouteEligibleAfterValueOnlyComplexityUpdate(t *testing.T) {
	ctx := context.Background()
	ws, id, _ := setupComplexityWorkspace(t)
	_, err := core.SetArtifactSizeWithProvenance(ctx, ws, id, core.SizeMutation{
		Size: strPtr("M"), Source: strPtr("agent"), RulesetVersion: strPtr("1.0"), Actor: core.ActorContextAgent,
	})
	require.NoError(t, err)
	setProvenanceComplete(t, ws, id)

	art, err := core.SetArtifactComplexityWithProvenance(ctx, ws, id, core.ComplexityMutation{
		Complexity: strPtr("high"), Source: strPtr("human"), RulesetVersion: strPtr("ah-v1"), Actor: core.ActorContextHuman,
	})
	require.NoError(t, err)
	assert.True(t, core.IsTaskRouteEligible(art))

	art, err = core.SetArtifactComplexityWithProvenance(ctx, ws, id, core.ComplexityMutation{
		Complexity: strPtr("trivial"), Actor: core.ActorContextHuman,
	})
	require.NoError(t, err)
	assert.False(t, core.IsTaskRouteEligible(art))
	assert.False(t, core.IsTaskRouteEligible((*models.Artifact)(nil)))
}
