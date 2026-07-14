package core_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
)

// TestAdoptItem_CrossRefRewrite_EmitsUTCUpdatedAt proves that when AdoptItem
// rewrites a referencing artifact's frontmatter (because it depended on the
// adopted item's old ID), that artifact's updated_at is restamped in canonical
// UTC even under a non-UTC local zone (site: artifact_references.go
// findCrossArtifactReferences rewrite).
func TestAdoptItem_CrossRefRewrite_EmitsUTCUpdatedAt(t *testing.T) {
	withNonUTCLocal(t)
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	feat1, err := core.CreateArtifact(ctx, ws, "UTC origin feature", "feature")
	require.NoError(t, err)
	feat2, err := core.CreateArtifact(ctx, ws, "UTC destination feature", "feature")
	require.NoError(t, err)

	t1, err := core.CreateArtifact(ctx, ws, "UTC task to adopt", "task", core.WithParent(feat1.ID))
	require.NoError(t, err)
	t2, err := core.CreateArtifact(ctx, ws, "UTC referencing task", "task",
		core.WithParent(feat1.ID), core.WithDependencies([]string{t1.ID}))
	require.NoError(t, err)

	result, err := core.AdoptItem(ctx, ws, t1.ID, feat2.ID)
	require.NoError(t, err)
	require.Contains(t, result.RewrittenArtifactIDs, t2.ID)

	assertFrontmatterUTC(t, readArtifactContent(t, ctx, ws, t2.ID), "updated_at")
}
