package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
	mcpinternal "github.com/softwaresalt/backlogit/internal/mcp"
)

// TestDocsLint_CleanCorpus_FindingsPresentEmptyArray_BothSurfaces is the sole
// scenario of 146.021-T (U9b): a clean corpus with no findings yields a
// present [] under the same .([]any) assertion on both the CLI and MCP
// surfaces. LintTree declares var findings []Finding and returns a nil
// slice for a clean corpus; NewLintReport's pre-allocation
// (make([]FindingReport, 0, len(findings))) is what converts it to [] at the
// transport boundary. This is a green-throughout shape guard pinning that
// conversion against 146.018-T (U8)'s changes to the producing path — it is
// NOT a red harness.
func TestDocsLint_CleanCorpus_FindingsPresentEmptyArray_BothSurfaces(t *testing.T) {
	root := t.TempDir()
	writeFixtureDoc(t, root, "docs/decisions/clean.md",
		"---\nchunk_strategy: h1-h2-h3\ndescription: \"\"\ndoc_type: decision\ningested_at: \"2026-06-01T00:00:00Z\"\nschema_version: \"1.0\"\nsource: docs/decisions/clean.md\ntitle: Clean\n---\nBody.\n")

	t.Run("cli", func(t *testing.T) {
		out, err := runDocs(t, root, "docs", "lint", "--format", "json")
		require.NoError(t, err, "a clean corpus must exit zero")

		var raw map[string]any
		require.NoError(t, json.Unmarshal([]byte(out), &raw))
		findings, ok := raw["findings"].([]any)
		require.True(t, ok, "findings must be present and decode as a JSON array even when empty")
		assert.Empty(t, findings)
	})

	t.Run("mcp", func(t *testing.T) {
		ctx := context.Background()
		backlogitDir := filepath.Join(root, ".backlogit")
		require.NoError(t, os.MkdirAll(backlogitDir, 0o755))
		if _, statErr := os.Stat(filepath.Join(backlogitDir, "config.yaml")); os.IsNotExist(statErr) {
			require.NoError(t, config.WriteDefaults(backlogitDir))
		}
		ws, err := core.NewWorkspace(ctx, root)
		require.NoError(t, err)
		t.Cleanup(func() { _ = ws.Close() })

		srv := mcpinternal.NewServer(ws)
		result, err := srv.InvokeTool(ctx, "backlogit_docs_lint", mcplib.CallToolRequest{})
		require.NoError(t, err)
		require.NotNil(t, result)
		require.False(t, result.IsError)

		tc, ok := result.Content[0].(mcplib.TextContent)
		require.True(t, ok)
		var raw map[string]any
		require.NoError(t, json.Unmarshal([]byte(tc.Text), &raw))
		findings, ok := raw["findings"].([]any)
		require.True(t, ok, "findings must be present and decode as a JSON array even when empty")
		assert.Empty(t, findings)
	})
}
