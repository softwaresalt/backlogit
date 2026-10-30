package db_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/models"
)

func routeEligibleFields() map[string]any {
	return map[string]any{
		"size":                       "M",
		"size_source":                "agent",
		"size_ruleset_version":       "1.0",
		"complexity":                 "high",
		"complexity_source":          "human",
		"complexity_ruleset_version": "ah-v1",
	}
}

func routeArtifact(mutate func(map[string]any)) *models.Artifact {
	cf := routeEligibleFields()
	if mutate != nil {
		mutate(cf)
	}
	return projectionArtifact("960.001-T", cf)
}

// TestU2_ComplexityProvenanceProjectionAndRouteEligibility covers 207.002-T.
func TestU2_ComplexityProvenanceProjectionAndRouteEligibility(t *testing.T) {
	ctx := context.Background()

	t.Run("projects provenance columns for tasks and nulls them when cleared", func(t *testing.T) {
		database := setupProjectionDB(t)
		require.NoError(t, db.UpsertItem(ctx, database, projectionArtifact("961.001-T", routeEligibleFields())))
		var src, rv sql.NullString
		require.NoError(t, database.QueryRowContext(ctx,
			`SELECT complexity_source, complexity_ruleset_version FROM items WHERE id = ?`, "961.001-T").Scan(&src, &rv))
		assert.Equal(t, "human", src.String)
		assert.Equal(t, "ah-v1", rv.String)

		require.NoError(t, db.UpsertItem(ctx, database, projectionArtifact("961.001-T", map[string]any{"complexity": "trivial"})))
		require.NoError(t, database.QueryRowContext(ctx,
			`SELECT complexity_source, complexity_ruleset_version FROM items WHERE id = ?`, "961.001-T").Scan(&src, &rv))
		assert.False(t, src.Valid)
		assert.False(t, rv.Valid)
	})

	t.Run("skips provenance projection for non-tasks", func(t *testing.T) {
		database := setupProjectionDB(t)
		feature := projectionArtifact("962-F", routeEligibleFields())
		feature.ArtifactType = "feature"
		feature.ParentID = ""
		feature.Level = 1
		feature.HierarchyPath = "962"
		require.NoError(t, db.UpsertItem(ctx, database, feature))
		var src, rv, cx sql.NullString
		require.NoError(t, database.QueryRowContext(ctx,
			`SELECT complexity_source, complexity_ruleset_version, complexity FROM items WHERE id = ?`, "962-F").Scan(&src, &rv, &cx))
		assert.False(t, src.Valid)
		assert.False(t, rv.Valid)
		assert.False(t, cx.Valid)
	})

	t.Run("bound parameter filters", func(t *testing.T) {
		database := setupProjectionDB(t)
		require.NoError(t, db.UpsertItem(ctx, database, projectionArtifact("963.001-T", routeEligibleFields())))
		other := routeEligibleFields()
		other["complexity_source"] = "agent"
		other["complexity_ruleset_version"] = "ah-v2"
		require.NoError(t, db.UpsertItem(ctx, database, projectionArtifact("963.002-T", other)))
		require.NoError(t, db.UpsertItem(ctx, database, projectionArtifact("963.003-T", map[string]any{"complexity": "low"})))

		items, err := db.QueryItems(ctx, database, db.QueryFilters{ComplexitySource: "human"})
		require.NoError(t, err)
		require.Len(t, items, 1)
		assert.Equal(t, "963.001-T", items[0].ID)

		items, err = db.QueryItems(ctx, database, db.QueryFilters{ComplexityRulesetVersion: "ah-v2"})
		require.NoError(t, err)
		require.Len(t, items, 1)
		assert.Equal(t, "963.002-T", items[0].ID)

		items, err = db.QueryItems(ctx, database, db.QueryFilters{ComplexitySource: "human' OR '1'='1"})
		require.NoError(t, err)
		assert.Empty(t, items, "filter values must be bound, never interpolated")

		items, err = db.QueryItems(ctx, database, db.QueryFilters{})
		require.NoError(t, err)
		assert.Len(t, items, 3, "legacy null-provenance tasks stay queryable")
	})

	t.Run("two_axis_route_eligibility_predicate", func(t *testing.T) {
		assert.True(t, db.IsTaskRouteEligible(routeArtifact(nil)))
		assert.False(t, db.IsTaskRouteEligible(nil))

		notTask := routeArtifact(nil)
		notTask.ArtifactType = "feature"
		assert.False(t, db.IsTaskRouteEligible(notTask))

		for _, key := range []string{"size", "size_source", "size_ruleset_version", "complexity", "complexity_source", "complexity_ruleset_version"} {
			art := routeArtifact(func(cf map[string]any) { delete(cf, key) })
			assert.False(t, db.IsTaskRouteEligible(art), "missing %s", key)
		}
		invalid := map[string]string{
			"size": "XXL", "size_source": "robot", "size_ruleset_version": "  ",
			"complexity": "extreme", "complexity_source": "robot", "complexity_ruleset_version": "v\n1",
		}
		for key, val := range invalid {
			art := routeArtifact(func(cf map[string]any) { cf[key] = val })
			assert.False(t, db.IsTaskRouteEligible(art), "invalid %s", key)
		}
		// Non-string values are never eligible.
		assert.False(t, db.IsTaskRouteEligible(routeArtifact(func(cf map[string]any) { cf["size"] = 3 })))

		// A value-only complexity update drops provenance, so the task is no longer eligible.
		valueOnly := routeArtifact(func(cf map[string]any) {
			cf["complexity"] = "trivial"
			delete(cf, "complexity_source")
			delete(cf, "complexity_ruleset_version")
		})
		assert.False(t, db.IsTaskRouteEligible(valueOnly))
	})
}
