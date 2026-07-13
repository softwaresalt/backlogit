package integration_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type pluginManifest struct {
	MCPServers map[string]pluginMCPServer `json:"mcpServers"`
	Agents     string                     `json:"agents"`
	Skills     string                     `json:"skills"`
}

type pluginMCPServer struct {
	Type    string   `json:"type"`
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

type marketplaceIndex struct {
	Name    string              `json:"name"`
	Plugins []marketplacePlugin `json:"plugins"`
}

type marketplacePlugin struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Source  struct {
		Source string `json:"source"`
		Repo   string `json:"repo"`
		Path   string `json:"path"`
	} `json:"source"`
	Repository string `json:"repository"`
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

	// The Copilot CLI resolves agents/skills by calling read_dir on each entry,
	// so they MUST be directory paths (repo-root-relative), not file paths.
	// Listing individual .agent.md/SKILL.md files causes read_dir to fail on a
	// file with ERROR_DIRECTORY (os error 267) on Windows during install.
	assert.Equal(t, "plugin/agents", manifest.Agents)
	assert.Equal(t, "plugin/skills", manifest.Skills)

	assertPluginDir(t, repoRoot, manifest.Agents)
	assertPluginDir(t, repoRoot, manifest.Skills)

	for _, agentFile := range []string{"stage.agent.md", "ship.agent.md"} {
		t.Run("agents/"+agentFile, func(t *testing.T) {
			assertPluginFileExists(t, repoRoot, manifest.Agents+"/"+agentFile)
		})
	}

	for _, skillDir := range []string{
		"build-feature", "compact-context", "compound", "compound-refresh",
		"deliberate", "file-lock", "fix-ci", "harness-architect", "harvest",
		"impl-plan", "operational-closure", "plan-harden", "plan-review",
		"pr-lifecycle", "review", "runtime-verification", "safety-modes",
		"skill-search", "spike",
	} {
		t.Run("skills/"+skillDir, func(t *testing.T) {
			assertPluginDir(t, repoRoot, manifest.Skills+"/"+skillDir)
			assertPluginFileExists(t, repoRoot, manifest.Skills+"/"+skillDir+"/SKILL.md")
		})
	}
}

func TestMarketplaceIndexesBacklogitPlugin(t *testing.T) {
	repoRoot := testRepoRoot(t)
	data, err := os.ReadFile(filepath.Join(repoRoot, ".claude-plugin", "marketplace.json"))
	require.NoError(t, err)

	var index marketplaceIndex
	require.NoError(t, json.Unmarshal(data, &index))
	assert.Equal(t, "softwaresalt", index.Name)

	var backlogit *marketplacePlugin
	for i := range index.Plugins {
		if index.Plugins[i].Name == "backlogit" {
			backlogit = &index.Plugins[i]
			break
		}
	}
	require.NotNil(t, backlogit, "marketplace must index the backlogit plugin")

	// The plugin manifest is discovered at .github/plugin/plugin.json, so the
	// marketplace source points at the repo root with no subpath.
	assert.Equal(t, "github", backlogit.Source.Source)
	assert.Equal(t, "softwaresalt/backlogit", backlogit.Source.Repo)
	assert.Empty(t, backlogit.Source.Path, "backlogit manifest is at repo-root .github/plugin/; no subpath")
	assert.NotEmpty(t, backlogit.Version)
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
		"docs/rationale.md",
	}

	for _, activePath := range activePaths {
		t.Run(filepath.ToSlash(activePath), func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(repoRoot, activePath))
			require.NoError(t, err)

			content := normalizeDocWhitespace(string(data))
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

func assertPluginDir(t *testing.T, repoRoot string, relPath string) {
	t.Helper()

	target := pluginAssetTarget(t, repoRoot, relPath)
	info, err := os.Stat(target)
	require.NoError(t, err, "plugin path must exist: %s", relPath)
	require.True(t, info.IsDir(), "plugin path must reference a directory: %s", relPath)
}

func assertPluginFileExists(t *testing.T, repoRoot string, relPath string) {
	t.Helper()

	target := pluginAssetTarget(t, repoRoot, relPath)
	info, err := os.Stat(target)
	require.NoError(t, err, "plugin file must exist: %s", relPath)
	require.False(t, info.IsDir(), "plugin path must reference a file: %s", relPath)
}

func pluginAssetTarget(t *testing.T, repoRoot string, relPath string) string {
	t.Helper()

	require.NotEmpty(t, relPath)
	require.False(t, filepath.IsAbs(relPath), "plugin asset path must be repo-root-relative")

	targetPath := filepath.Clean(filepath.Join(repoRoot, filepath.FromSlash(relPath)))
	relativeTarget, err := filepath.Rel(repoRoot, targetPath)
	require.NoError(t, err)
	require.NotContains(t, relativeTarget, "..", "plugin asset path must stay inside repo root")

	return targetPath
}

func normalizeDocWhitespace(content string) string {
	return strings.Join(strings.Fields(content), " ")
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
