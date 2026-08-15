package mcp

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/models"
)

func TestMCPServerLazyInitDefersDBOnlyLinkMigration(t *testing.T) {
	ctx := context.Background()
	root := setupDurableWorkspaceRoot(t, false)

	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)
	feature, err := core.CreateArtifact(ctx, ws, "MCP startup feature", "feature")
	require.NoError(t, err)
	source, err := core.CreateArtifact(ctx, ws, "MCP startup source", "task", core.WithParent(feature.ID))
	require.NoError(t, err)
	target, err := core.CreateArtifact(ctx, ws, "MCP startup target", "task", core.WithParent(feature.ID))
	require.NoError(t, err)
	require.NoError(t, db.AddLink(ctx, ws.DB, source.ID, target.ID, "informs"))
	sourcePath, err := core.FindArtifactPath(ctx, ws, source.ID)
	require.NoError(t, err)
	require.NoError(t, ws.Close())

	server := NewServerForRoot(root)
	initialized, result := server.requireWorkspace(ctx)
	require.Nil(t, result)
	require.NotNil(t, initialized)
	t.Cleanup(func() { require.NoError(t, initialized.Close()) })

	data, err := os.ReadFile(sourcePath)
	require.NoError(t, err)
	frontmatter, body, err := models.ParseFrontmatter(string(data))
	require.NoError(t, err)
	artifact, err := models.ArtifactFromFrontmatter(frontmatter, body)
	require.NoError(t, err)
	assert.Empty(t, artifact.Links)
}
