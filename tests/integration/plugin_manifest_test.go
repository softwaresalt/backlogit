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
	"gopkg.in/yaml.v3"
)

type pluginManifest struct {
	MCPServers map[string]pluginMCPServer `json:"mcpServers"`
	Agents     string                     `json:"agents"`
	Skills     string                     `json:"skills"`
	Version    string                     `json:"version"`
	License    string                     `json:"license"`
}

type pluginMCPServer struct {
	Type    string   `json:"type"`
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

var expectedPluginAgents = []string{
	"stage.agent.md",
	"ship.agent.md",
}

var expectedPluginSkills = []string{
	"build-feature",
	"compact-context",
	"compound",
	"compound-refresh",
	"deliberate",
	"file-lock",
	"fix-ci",
	"harness-architect",
	"harvest",
	"impl-plan",
	"operational-closure",
	"plan-harden",
	"plan-review",
	"pr-lifecycle",
	"review",
	"runtime-verification",
	"safety-modes",
	"skill-search",
	"spike",
}

type marketplaceIndex struct {
	Name    string              `json:"name"`
	Plugins []marketplacePlugin `json:"plugins"`
}

type marketplacePlugin struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	License string `json:"license"`
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

	for _, agentFile := range expectedPluginAgents {
		t.Run("agents/"+agentFile, func(t *testing.T) {
			assertPluginFileExists(t, repoRoot, manifest.Agents+"/"+agentFile)
		})
	}

	for _, skillDir := range expectedPluginSkills {
		t.Run("skills/"+skillDir, func(t *testing.T) {
			assertPluginDir(t, repoRoot, manifest.Skills+"/"+skillDir)
			assertPluginFileExists(t, repoRoot, manifest.Skills+"/"+skillDir+"/SKILL.md")
		})
	}
}

func TestPluginBundleStructurallyValid(t *testing.T) {
	repoRoot := testRepoRoot(t)
	manifest := readPluginManifest(t, repoRoot)

	require.Equal(t, "plugin/agents", manifest.Agents)
	require.Equal(t, "plugin/skills", manifest.Skills)

	agentFiles := collectPluginAgentFiles(t, repoRoot, manifest.Agents)
	assert.ElementsMatch(t, expectedPluginAgents, agentFiles,
		"plugin agent set must match the canonical bundle list")

	for _, agentFile := range agentFiles {
		t.Run("agent frontmatter/"+agentFile, func(t *testing.T) {
			assertPluginMarkdownHasNameAndBody(t, filepath.Join(repoRoot, filepath.FromSlash(manifest.Agents), agentFile))
		})
	}

	skillDirs := collectPluginSkillDirs(t, repoRoot, manifest.Skills)
	assert.ElementsMatch(t, expectedPluginSkills, skillDirs,
		"plugin skill set must match the canonical bundle list")

	for _, skillDir := range skillDirs {
		t.Run("skill frontmatter/"+skillDir, func(t *testing.T) {
			assertPluginMarkdownHasNameAndBody(t,
				filepath.Join(repoRoot, filepath.FromSlash(manifest.Skills), skillDir, "SKILL.md"))
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

	// The marketplace version must match the canonical plugin manifest version,
	// so a plugin-manifest bump cannot leave the marketplace advertising a stale
	// version while this guard still passes.
	manifestData, err := os.ReadFile(filepath.Join(repoRoot, ".github", "plugin", "plugin.json"))
	require.NoError(t, err)
	var manifest pluginManifest
	require.NoError(t, json.Unmarshal(manifestData, &manifest))
	require.NotEmpty(t, manifest.Version)
	assert.Equal(t, manifest.Version, backlogit.Version,
		"marketplace version must match .github/plugin/plugin.json version")
}

func TestPluginLicenseMetadataAligned(t *testing.T) {
	repoRoot := testRepoRoot(t)

	// The root LICENSE file is authoritative; all plugin distribution metadata
	// must agree with it. This guards against a one-sided edit re-introducing
	// the LICENSE-vs-metadata drift this change repaired.
	const wantLicense = "Apache-2.0"

	manifestData, err := os.ReadFile(filepath.Join(repoRoot, ".github", "plugin", "plugin.json"))
	require.NoError(t, err)
	var manifest pluginManifest
	require.NoError(t, json.Unmarshal(manifestData, &manifest))
	assert.Equal(t, wantLicense, manifest.License, "plugin.json license must match the LICENSE file")

	marketData, err := os.ReadFile(filepath.Join(repoRoot, ".claude-plugin", "marketplace.json"))
	require.NoError(t, err)
	var index marketplaceIndex
	require.NoError(t, json.Unmarshal(marketData, &index))
	var found bool
	for i := range index.Plugins {
		if index.Plugins[i].Name == "backlogit" {
			found = true
			assert.Equal(t, wantLicense, index.Plugins[i].License, "marketplace license must match the LICENSE file")
		}
	}
	require.True(t, found, "marketplace must index the backlogit plugin")

	readme, err := os.ReadFile(filepath.Join(repoRoot, "README.md"))
	require.NoError(t, err)
	assert.NotContains(t, string(readme), "license-MIT", "README license badge must not advertise MIT")
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

func readPluginManifest(t *testing.T, repoRoot string) pluginManifest {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(repoRoot, ".github", "plugin", "plugin.json"))
	require.NoError(t, err)

	var manifest pluginManifest
	require.NoError(t, json.Unmarshal(data, &manifest))
	return manifest
}

func collectPluginAgentFiles(t *testing.T, repoRoot string, agentsDir string) []string {
	t.Helper()

	matches, err := filepath.Glob(filepath.Join(repoRoot, filepath.FromSlash(agentsDir), "*.agent.md"))
	require.NoError(t, err)

	agentFiles := make([]string, 0, len(matches))
	for _, match := range matches {
		agentFiles = append(agentFiles, filepath.Base(match))
	}
	return agentFiles
}

func collectPluginSkillDirs(t *testing.T, repoRoot string, skillsDir string) []string {
	t.Helper()

	skillsPath := filepath.Join(repoRoot, filepath.FromSlash(skillsDir))
	entries, err := os.ReadDir(skillsPath)
	require.NoError(t, err)

	skillDirs := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillPath := filepath.Join(skillsPath, entry.Name(), "SKILL.md")
		info, err := os.Stat(skillPath)
		require.NoError(t, err, "plugin skill directory must contain SKILL.md: %s", entry.Name())
		require.False(t, info.IsDir(), "plugin skill path must reference a file: %s", skillPath)
		skillDirs = append(skillDirs, entry.Name())
	}
	return skillDirs
}

func assertPluginMarkdownHasNameAndBody(t *testing.T, path string) {
	t.Helper()

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	frontmatter, body := splitPluginFrontmatter(t, string(data), path)

	var header struct {
		Name string `yaml:"name"`
	}
	require.NoError(t, yaml.Unmarshal([]byte(frontmatter), &header), "parse plugin YAML frontmatter: %s", path)

	require.NotEmpty(t, strings.TrimSpace(header.Name), "plugin frontmatter name must not be empty: %s", path)
	require.NotEmpty(t, strings.TrimSpace(body), "plugin markdown body must not be empty: %s", path)
}

func splitPluginFrontmatter(t *testing.T, content string, path string) (string, string) {
	t.Helper()

	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(normalized, "\n")
	require.NotEmpty(t, lines)
	require.Equal(t, "---", lines[0], "plugin markdown must start with YAML frontmatter: %s", path)

	for i := 1; i < len(lines); i++ {
		if lines[i] == "---" {
			return strings.Join(lines[1:i], "\n"), strings.Join(lines[i+1:], "\n")
		}
	}

	require.Failf(t, "missing frontmatter close", "plugin markdown must close YAML frontmatter: %s", path)
	return "", ""
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
