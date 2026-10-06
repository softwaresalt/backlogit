package mcp

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/db"
)

func TestUSR7_ServedRootSelfReportCharacterization(t *testing.T) {
	s := setupCatalogServer(t)
	ctx := context.Background()
	root := s.RootPath
	storageRoot := filepath.Join(root, ".backlogit")

	canonicalPath := func(t *testing.T, path string) string {
		t.Helper()
		resolved, err := filepath.EvalSymlinks(path)
		require.NoError(t, err)
		absolute, err := filepath.Abs(resolved)
		require.NoError(t, err)
		return filepath.Clean(absolute)
	}

	t.Run("MetadataCatalog", func(t *testing.T) {
		req := mcplib.CallToolRequest{}
		req.Params.Name = "backlogit_get_metadata_catalog"
		req.Params.Arguments = map[string]any{}

		result, err := s.handleGetMetadataCatalog(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.False(t, result.IsError, "metadata catalog request should succeed")
		require.NotEmpty(t, result.Content)

		content, ok := result.Content[0].(mcplib.TextContent)
		require.True(t, ok, "metadata catalog result should be text")

		var catalog core.MetadataCatalog
		require.NoError(t, json.Unmarshal([]byte(content.Text), &catalog))
		require.True(t, filepath.IsAbs(catalog.Workspace.RootPath))
		require.True(t, filepath.IsAbs(catalog.Workspace.StorageRoot))

		rootPath := canonicalPath(t, catalog.Workspace.RootPath)
		storagePath := canonicalPath(t, catalog.Workspace.StorageRoot)
		require.Equal(t, canonicalPath(t, root), rootPath)
		require.Equal(t, canonicalPath(t, storageRoot), storagePath)
		require.Equal(t, rootPath, filepath.Dir(storagePath),
			"storage root should be a direct child of the workspace root")
	})

	t.Run("IndexFile", func(t *testing.T) {
		const query = "SELECT name, file FROM pragma_database_list WHERE name = 'main'"

		req := mcplib.CallToolRequest{}
		req.Params.Name = "backlogit_query_sql"
		req.Params.Arguments = map[string]any{"sql": query}

		result, err := s.handleQuerySQL(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.False(t, result.IsError, "database-list query should succeed")
		require.NotEmpty(t, result.Content)

		content, ok := result.Content[0].(mcplib.TextContent)
		require.True(t, ok, "database-list result should be text")

		var rows []struct {
			Name string `json:"name"`
			File string `json:"file"`
		}
		require.NoError(t, json.Unmarshal([]byte(content.Text), &rows))
		require.Len(t, rows, 1)
		require.Equal(t, "main", rows[0].Name)
		require.True(t, filepath.IsAbs(rows[0].File))
		require.Equal(t,
			canonicalPath(t, filepath.Join(storageRoot, "backlogit.db")),
			canonicalPath(t, rows[0].File),
		)

		gate := db.ValidateQuery(query)
		require.True(t, gate.Allowed, "database-list query should pass the read-only gate: %s", gate.Reason)
	})
}
