package core_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/db"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/mdfront"
)

func strPtr(s string) *string { return &s }

func readCustomFields(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	md, err := mdfront.Decode(raw)
	require.NoError(t, err)
	cf, _ := md.Frontmatter["custom_fields"].(map[string]any)
	return cf
}

func complexityEventCount(t *testing.T, ws *core.Workspace, id string) int {
	t.Helper()
	raw, err := os.ReadFile(events.LogPathForItem(core.WorkspaceLogsRoot(ws.RootPath), id))
	if os.IsNotExist(err) {
		return 0
	}
	require.NoError(t, err)
	n := 0
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if strings.Contains(line, events.EventEstimateHistory) && strings.Contains(line, `"dimension":"complexity"`) {
			n++
		}
	}
	return n
}

func setProvenanceComplete(t *testing.T, ws *core.Workspace, id string) {
	t.Helper()
	_, err := core.SetArtifactComplexityWithProvenance(context.Background(), ws, id, core.ComplexityMutation{
		Complexity:     strPtr("high"),
		Source:         strPtr("human"),
		RulesetVersion: strPtr("ah-v1"),
		Actor:          core.ActorContextHuman,
	})
	require.NoError(t, err)
}

// TestU1_ComplexityProvenanceSeam exercises the complexity provenance seam (207.001-T).
func TestU1_ComplexityProvenanceSeam(t *testing.T) {
	ctx := context.Background()

	t.Run("header-def declares task-only optional provenance fields", func(t *testing.T) {
		ws, _, _ := setupComplexityWorkspace(t)
		fields := ws.HeaderDef.Types["task"].Fields
		src := fields["complexity_source"]
		require.NotNil(t, src)
		assert.Equal(t, "enum", src.Type)
		assert.Equal(t, []string{"human", "agent", "derived"}, src.Values)
		assert.True(t, src.Optional)
		rv := fields["complexity_ruleset_version"]
		require.NotNil(t, rv)
		assert.Equal(t, "string", rv.Type)
		assert.True(t, rv.Optional)
		assert.NotContains(t, ws.HeaderDef.Types["feature"].Fields, "complexity_source")
	})

	t.Run("persists provenance and appends complexity audit event", func(t *testing.T) {
		ws, id, path := setupComplexityWorkspace(t)
		setProvenanceComplete(t, ws, id)
		cf := readCustomFields(t, path)
		assert.Equal(t, "high", cf["complexity"])
		assert.Equal(t, "human", cf["complexity_source"])
		assert.Equal(t, "ah-v1", cf["complexity_ruleset_version"])
		assert.Equal(t, 1, complexityEventCount(t, ws, id))
		got, err := db.GetItem(ctx, ws.DB, id)
		require.NoError(t, err)
		assert.Equal(t, "human", got.CustomFields["complexity_source"])
	})

	t.Run("value-only update deletes stale provenance", func(t *testing.T) {
		ws, id, path := setupComplexityWorkspace(t)
		setProvenanceComplete(t, ws, id)
		art, err := core.SetArtifactComplexityWithProvenance(ctx, ws, id, core.ComplexityMutation{
			Complexity: strPtr("trivial"), Actor: core.ActorContextHuman,
		})
		require.NoError(t, err)
		cf := readCustomFields(t, path)
		assert.Equal(t, "trivial", cf["complexity"])
		assert.NotContains(t, cf, "complexity_source")
		assert.NotContains(t, cf, "complexity_ruleset_version")
		assert.NotContains(t, art.CustomFields, "complexity_source")
		assert.NotContains(t, art.CustomFields, "complexity_ruleset_version")
		got, err := db.GetItem(ctx, ws.DB, id)
		require.NoError(t, err)
		assert.NotContains(t, got.CustomFields, "complexity_source")
		assert.NotContains(t, got.CustomFields, "complexity_ruleset_version")

		// Legacy wrapper behaves identically.
		setProvenanceComplete(t, ws, id)
		_, err = core.SetArtifactComplexity(ctx, ws, id, "low")
		require.NoError(t, err)
		cf = readCustomFields(t, path)
		assert.NotContains(t, cf, "complexity_source")
		assert.NotContains(t, cf, "complexity_ruleset_version")
	})

	t.Run("empty complexity clears all three fields and rejects provenance", func(t *testing.T) {
		ws, id, path := setupComplexityWorkspace(t)
		setProvenanceComplete(t, ws, id)
		_, err := core.SetArtifactComplexityWithProvenance(ctx, ws, id, core.ComplexityMutation{
			Complexity: strPtr(""), Source: strPtr("human"), Actor: core.ActorContextHuman,
		})
		require.ErrorIs(t, err, blerrors.ErrValidation)
		_, err = core.SetArtifactComplexityWithProvenance(ctx, ws, id, core.ComplexityMutation{
			Complexity: strPtr(""), RulesetVersion: strPtr("ah-v1"), Actor: core.ActorContextHuman,
		})
		require.ErrorIs(t, err, blerrors.ErrValidation)
		_, err = core.SetArtifactComplexityWithProvenance(ctx, ws, id, core.ComplexityMutation{
			Complexity: strPtr(""), Actor: core.ActorContextHuman,
		})
		require.NoError(t, err)
		cf := readCustomFields(t, path)
		assert.NotContains(t, cf, "complexity")
		assert.NotContains(t, cf, "complexity_source")
		assert.NotContains(t, cf, "complexity_ruleset_version")
	})

	t.Run("provenance completeness and value validation", func(t *testing.T) {
		ws, id, path := setupComplexityWorkspace(t)
		before, err := os.ReadFile(path)
		require.NoError(t, err)
		cases := map[string]core.ComplexityMutation{
			"source without ruleset":   {Complexity: strPtr("low"), Source: strPtr("human")},
			"ruleset without source":   {Complexity: strPtr("low"), RulesetVersion: strPtr("v1")},
			"invalid source":           {Complexity: strPtr("low"), Source: strPtr("robot"), RulesetVersion: strPtr("v1")},
			"whitespace ruleset":       {Complexity: strPtr("low"), Source: strPtr("human"), RulesetVersion: strPtr("   ")},
			"control char ruleset":     {Complexity: strPtr("low"), Source: strPtr("human"), RulesetVersion: strPtr("v1\n2")},
			"provenance without value": {Source: strPtr("human"), RulesetVersion: strPtr("v1")},
			"invalid complexity":       {Complexity: strPtr("extreme")},
			"explicit empty ruleset":   {Complexity: strPtr("low"), RulesetVersion: strPtr("")},
			"explicit empty source":    {Complexity: strPtr("low"), Source: strPtr("")},
		}
		for name, m := range cases {
			m.Actor = core.ActorContextHuman
			_, err := core.SetArtifactComplexityWithProvenance(ctx, ws, id, m)
			require.ErrorIs(t, err, blerrors.ErrValidation, name)
		}
		after, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, before, after, "rejected mutations must not write")
		assert.Equal(t, 0, complexityEventCount(t, ws, id))
	})

	t.Run("audit append failure leaves file byte-identical", func(t *testing.T) {
		ws, id, path := setupComplexityWorkspace(t)
		logPath := events.LogPathForItem(core.WorkspaceLogsRoot(ws.RootPath), id)
		require.NoError(t, os.MkdirAll(logPath, 0o755)) // a directory makes the append fail
		before, err := os.ReadFile(path)
		require.NoError(t, err)
		_, err = core.SetArtifactComplexityWithProvenance(ctx, ws, id, core.ComplexityMutation{
			Complexity: strPtr("high"), Source: strPtr("human"), RulesetVersion: strPtr("ah-v1"), Actor: core.ActorContextHuman,
		})
		require.Error(t, err)
		after, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, before, after)
	})

	t.Run("generic create refuses and generic update preserves reserved keys", func(t *testing.T) {
		ws, id, path := setupComplexityWorkspace(t)
		for _, k := range []string{"complexity_source", "complexity_ruleset_version"} {
			_, err := core.CreateArtifact(ctx, ws, "Off seam", "task", core.WithFields(map[string]any{k: "x"}))
			require.ErrorIs(t, err, blerrors.ErrValidation, k)
		}
		setProvenanceComplete(t, ws, id)
		_, err := core.UpdateArtifact(ctx, ws, id, map[string]any{
			"custom_fields": map[string]any{
				"complexity": "low", "complexity_source": "agent", "complexity_ruleset_version": "evil",
			},
		})
		require.NoError(t, err)
		cf := readCustomFields(t, path)
		assert.Equal(t, "high", cf["complexity"])
		assert.Equal(t, "human", cf["complexity_source"])
		assert.Equal(t, "ah-v1", cf["complexity_ruleset_version"])
	})
}
