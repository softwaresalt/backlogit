package integration_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// TestGitHubSpikeSkillFrontmatterMatchesPluginCopy guards the in-repo
// .github/skills/spike/SKILL.md frontmatter against the authoritative
// plugin/skills/spike/SKILL.md copy.
//
// This is the RED-phase test for shipment 093-S / task 104.001-T. It exists
// because TestPluginBundleStructurallyValid CANNOT cover the .github copy:
// that test resolves skills from manifest.Skills (== "plugin/skills") and only
// ever reads plugin/skills/*/SKILL.md, so it passes both before AND after the
// .github edit and provides no red phase (PR #234 review threads 3575108875 /
// 3575121058). TestPluginBundleStructurallyValid remains a secondary
// regression gate for the PLUGIN copies only.
//
// The test fails on the pre-change .github copy (which declares only a
// description: key and no name:) and passes once name: spike is added. It
// enforces FULL top-level frontmatter parity — every key AND value — via a
// complete map comparison, so drift in any shared scalar (e.g. description)
// between the two copies is also caught, matching the parity contract the test
// name advertises.
func TestGitHubSpikeSkillFrontmatterMatchesPluginCopy(t *testing.T) {
	repoRoot := testRepoRoot(t)

	githubPath := filepath.Join(repoRoot, ".github", "skills", "spike", "SKILL.md")
	pluginPath := filepath.Join(repoRoot, "plugin", "skills", "spike", "SKILL.md")

	githubDoc := parseFrontmatterDoc(t, githubPath)
	pluginDoc := parseFrontmatterDoc(t, pluginPath)

	// Sharp, targeted assertion for the RED scenario: the pre-change .github
	// copy lacks the top-level name: key. Keeping this as a distinct check
	// yields a clear failure message for the specific gap 104.001-T closes.
	require.Contains(t, githubDoc, "name",
		".github/skills/spike/SKILL.md frontmatter must declare a top-level name: key to match the plugin copy")
	require.NotEmpty(t, pluginDoc["name"], "plugin spike SKILL.md must declare a non-empty name:")

	// Full parity: identical top-level frontmatter keys AND values across both
	// copies. This catches the name: gap (the 104.001-T fix) and any future
	// drift in description or other shared scalars.
	require.Equal(t, pluginDoc, githubDoc,
		".github and plugin spike SKILL.md must have identical top-level frontmatter (keys and values)")
}

// parseFrontmatterDoc reads the file at path, splits its leading YAML
// frontmatter, and unmarshals it into a generic map for structural comparison.
func parseFrontmatterDoc(t *testing.T, path string) map[string]any {
	t.Helper()

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	frontmatter, _ := splitPluginFrontmatter(t, string(data), path)

	var doc map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(frontmatter), &doc), "parse YAML frontmatter: %s", path)
	return doc
}
