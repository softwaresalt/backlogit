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
	Agents     []string                   `json:"agents"`
	Skills     []string                   `json:"skills"`
}

type pluginMCPServer struct {
	Type    string   `json:"type"`
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

func TestPluginManifestsLaunchBacklogitFromPath(t *testing.T) {
	repoRoot := testRepoRoot(t)
	manifestPath := filepath.Join(repoRoot, ".github", "plugin", "plugin.json")

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

	assert.Equal(t, []string{
		"plugin/agents/stage.agent.md",
		"plugin/agents/ship.agent.md",
	}, manifest.Agents)
	assert.Equal(t, []string{
		"plugin/skills/build-feature/SKILL.md",
		"plugin/skills/compact-context/SKILL.md",
		"plugin/skills/compound/SKILL.md",
		"plugin/skills/compound-refresh/SKILL.md",
		"plugin/skills/deliberate/SKILL.md",
		"plugin/skills/file-lock/SKILL.md",
		"plugin/skills/fix-ci/SKILL.md",
		"plugin/skills/harness-architect/SKILL.md",
		"plugin/skills/harvest/SKILL.md",
		"plugin/skills/impl-plan/SKILL.md",
		"plugin/skills/operational-closure/SKILL.md",
		"plugin/skills/plan-harden/SKILL.md",
		"plugin/skills/plan-review/SKILL.md",
		"plugin/skills/pr-lifecycle/SKILL.md",
		"plugin/skills/review/SKILL.md",
		"plugin/skills/runtime-verification/SKILL.md",
		"plugin/skills/safety-modes/SKILL.md",
		"plugin/skills/skill-search/SKILL.md",
		"plugin/skills/spike/SKILL.md",
	}, manifest.Skills)

	for _, assetPath := range append(manifest.Agents, manifest.Skills...) {
		t.Run(assetPath, func(t *testing.T) {
			assertPluginAssetExists(t, repoRoot, assetPath)
		})
	}
}

func TestPluginManifestHasNoLegacyDriftCopies(t *testing.T) {
	repoRoot := testRepoRoot(t)
	legacyPaths := []string{
		filepath.Join(repoRoot, "plugin", ".mcp.json"),
		filepath.Join(repoRoot, "plugin", "plugin.json"),
	}

	for _, legacyPath := range legacyPaths {
		t.Run(filepath.Base(legacyPath), func(t *testing.T) {
			_, err := os.Stat(legacyPath)
			require.ErrorIs(t, err, os.ErrNotExist)
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
		".github/plugin/plugin.json",
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

func TestActivePluginDocsKeepPlainOwnerRepoInstallCanonical(t *testing.T) {
	repoRoot := testRepoRoot(t)
	activePaths := []string{
		"README.md",
		"docs/installation.md",
		"docs/plugin-guide.md",
	}

	for _, activePath := range activePaths {
		t.Run(filepath.ToSlash(activePath), func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(repoRoot, activePath))
			require.NoError(t, err)

			content := string(data)
			assert.Contains(t, content, "copilot plugin install softwaresalt/backlogit")
			assert.NotContains(t, content, "copilot plugin install softwaresalt/backlogit:plugin")
		})
	}
}

func TestPluginClosureRecordsWindowsSubdirInstallFailure(t *testing.T) {
	repoRoot := testRepoRoot(t)
	closurePath := filepath.Join(repoRoot, "docs", "closure", "2026-07-12-plugin-install-path-fix-closure.md")
	data, err := os.ReadFile(closurePath)
	require.NoError(t, err)

	content := string(data)
	assert.Contains(t, content, "copilot plugin install softwaresalt/backlogit:plugin")
	assert.Contains(t, content, "Windows")
	assert.Contains(t, content, "os error 267")
	assert.Contains(t, content, "The directory name is invalid")
	assert.NotContains(t, content, "workaround remains available")
}

func assertPluginAssetExists(t *testing.T, repoRoot string, assetPath string) {
	t.Helper()

	require.NotEmpty(t, assetPath)
	require.False(t, filepath.IsAbs(assetPath), "plugin asset path must be repo-root-relative")

	targetPath := filepath.Clean(filepath.Join(repoRoot, filepath.FromSlash(assetPath)))
	relativeTarget, err := filepath.Rel(repoRoot, targetPath)
	require.NoError(t, err)
	require.NotContains(t, relativeTarget, "..", "plugin asset path must stay inside repo root")

	info, err := os.Stat(targetPath)
	require.NoError(t, err, "plugin asset path must exist: %s", assetPath)
	require.False(t, info.IsDir(), "plugin asset path must reference a file: %s", assetPath)
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
