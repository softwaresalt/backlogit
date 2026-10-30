package mcp

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/db"
)

// TestU3_ComplexityProvenanceMCPAndCLIParity covers the MCP half of 207.003-T.
func TestU3_ComplexityProvenanceMCPAndCLIParity(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()
	feature, err := core.CreateArtifact(ctx, ws, "U3 feature", "feature")
	require.NoError(t, err)
	newTask := func(title string) string {
		task, err := core.CreateArtifact(ctx, ws, title, "task", core.WithParent(feature.ID))
		require.NoError(t, err)
		return task.ID
	}

	t.Run("explicit human source is rejected over MCP", func(t *testing.T) {
		id := newTask("U3 human")
		result, err := s.handleUpdateItem(ctx, contractRequest(map[string]any{
			"id": id, "complexity": "high", "complexity_source": "human", "complexity_ruleset_version": "ah-v1",
		}))
		require.NoError(t, err)
		assert.Equal(t, "validation_failed", contractErrorType(t, result))
	})

	t.Run("ruleset alone defaults source to agent", func(t *testing.T) {
		id := newTask("U3 default")
		result, err := s.handleUpdateItem(ctx, contractRequest(map[string]any{
			"id": id, "complexity": "high", "complexity_ruleset_version": "ah-v1",
		}))
		require.NoError(t, err)
		require.False(t, result.IsError, resultTextForHarness(t, result))
		cf := extractResultJSON(t, result)["custom_fields"].(map[string]any)
		assert.Equal(t, "agent", cf["complexity_source"])
		assert.Equal(t, "ah-v1", cf["complexity_ruleset_version"])
	})

	t.Run("source without ruleset and provenance without complexity are rejected", func(t *testing.T) {
		id := newTask("U3 incomplete")
		for _, args := range []map[string]any{
			{"id": id, "complexity": "high", "complexity_source": "agent"},
			{"id": id, "complexity_source": "agent", "complexity_ruleset_version": "ah-v1"},
			{"id": id, "complexity": "", "complexity_source": "agent", "complexity_ruleset_version": "ah-v1"},
			{"id": id, "complexity": "high", "complexity_ruleset_version": "ah-v1", "size": "M"},
		} {
			result, err := s.handleUpdateItem(ctx, contractRequest(args))
			require.NoError(t, err)
			assert.Equal(t, "validation_failed", contractErrorType(t, result), "%v", args)
		}
	})

	t.Run("value-only update clears stale provenance and route eligibility", func(t *testing.T) {
		id := newTask("U3 stale")
		_, err := core.SetArtifactSizeWithProvenance(ctx, ws, id, core.SizeMutation{
			Size: strp("M"), Source: strp("agent"), RulesetVersion: strp("1.0"), Actor: core.ActorContextAgent,
		})
		require.NoError(t, err)
		_, err = core.SetArtifactComplexityWithProvenance(ctx, ws, id, core.ComplexityMutation{
			Complexity: strp("high"), Source: strp("human"), RulesetVersion: strp("ah-v1"), Actor: core.ActorContextHuman,
		})
		require.NoError(t, err)
		got, err := db.GetItem(ctx, ws.DB, id)
		require.NoError(t, err)
		require.True(t, core.IsTaskRouteEligible(got))

		result, err := s.handleUpdateItem(ctx, contractRequest(map[string]any{"id": id, "complexity": "trivial"}))
		require.NoError(t, err)
		require.False(t, result.IsError, resultTextForHarness(t, result))
		cf := extractResultJSON(t, result)["custom_fields"].(map[string]any)
		assert.NotContains(t, cf, "complexity_source")
		assert.NotContains(t, cf, "complexity_ruleset_version")
		got, err = db.GetItem(ctx, ws.DB, id)
		require.NoError(t, err)
		assert.False(t, core.IsTaskRouteEligible(got))
	})

	t.Run("empty complexity clears all three fields", func(t *testing.T) {
		id := newTask("U3 clear")
		result, err := s.handleUpdateItem(ctx, contractRequest(map[string]any{
			"id": id, "complexity": "high", "complexity_ruleset_version": "ah-v1",
		}))
		require.NoError(t, err)
		require.False(t, result.IsError, resultTextForHarness(t, result))
		result, err = s.handleUpdateItem(ctx, contractRequest(map[string]any{"id": id, "complexity": ""}))
		require.NoError(t, err)
		require.False(t, result.IsError, resultTextForHarness(t, result))
		got, err := db.GetItem(ctx, ws.DB, id)
		require.NoError(t, err)
		assert.NotContains(t, got.CustomFields, "complexity")
		assert.NotContains(t, got.CustomFields, "complexity_source")
		assert.NotContains(t, got.CustomFields, "complexity_ruleset_version")
	})
}

func strp(s string) *string { return &s }
