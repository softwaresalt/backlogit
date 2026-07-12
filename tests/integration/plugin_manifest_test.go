package integration_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type pluginManifest struct {
	MCPServers map[string]pluginMCPServer `json:"mcpServers"`
}

type pluginMCPServer struct {
	Type    string   `json:"type"`
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

func TestPluginManifestsLaunchBacklogitFromPath(t *testing.T) {
	repoRoot := testRepoRoot(t)
	manifestPaths := []string{
		filepath.Join(repoRoot, "plugin", ".mcp.json"),
		filepath.Join(repoRoot, "plugin", "plugin.json"),
	}

	for _, manifestPath := range manifestPaths {
		t.Run(filepath.Base(manifestPath), func(t *testing.T) {
			data, err := os.ReadFile(manifestPath)
			require.NoError(t, err)
			assert.NotContains(t, string(data), "npx")
			assert.NotContains(t, string(data), "@backlogit/")

			var manifest pluginManifest
			require.NoError(t, json.Unmarshal(data, &manifest))

			serverRaw, ok := manifest.MCPServers["backlogit"]
			require.True(t, ok, "backlogit MCP server must be declared")
			assertServerHasOnlyLaunchFields(t, data)

			server := serverRaw
			assert.Equal(t, "stdio", server.Type)
			assert.Equal(t, "backlogit", server.Command)
			assert.Equal(t, []string{"mcp"}, server.Args)
		})
	}
}

func TestActivePluginDocsDoNotReferenceRetiredNPMWrapper(t *testing.T) {
	repoRoot := testRepoRoot(t)
	activePaths := []string{
		".autoharness/backlog-registry.yaml",
		".autoharness/harness-manifest.yaml",
		".github/workflows/ci.yml",
		".github/workflows/release.yml",
		".gitignore",
		"Makefile",
		"README.md",
		"docs/installation.md",
		"docs/plugin-guide.md",
		"docs/workflow.md",
		"plugin/.mcp.json",
		"plugin/plugin.json",
	}
	forbidden := []string{
		"npx ",
		"npm install",
		"@backlogit/",
		"package-npm",
		"npm/backlogit-mcp",
		"npm/platforms",
		"!npm/backlogit-mcp/bin",
	}

	for _, activePath := range activePaths {
		t.Run(filepath.ToSlash(activePath), func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(repoRoot, activePath))
			require.NoError(t, err)

			content := string(data)
			for _, needle := range forbidden {
				assert.NotContains(t, content, needle)
			}
		})
	}
}

func assertServerHasOnlyLaunchFields(t *testing.T, data []byte) {
	t.Helper()

	var rawManifest struct {
		MCPServers map[string]map[string]json.RawMessage `json:"mcpServers"`
	}
	require.NoError(t, json.Unmarshal(data, &rawManifest))
	server, ok := rawManifest.MCPServers["backlogit"]
	require.True(t, ok, "backlogit MCP server must be declared")

	fields := make([]string, 0, len(server))
	for field := range server {
		fields = append(fields, field)
	}
	assert.ElementsMatch(t, []string{"type", "command", "args"}, fields)
}

func testRepoRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok, "resolve repo root: runtime caller unavailable")
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}
