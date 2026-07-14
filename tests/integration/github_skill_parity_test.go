package integration_test

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
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
// This test fails on the pre-change .github copy (which declares only a
// description: key and no name:) and passes once name: spike is added.
func TestGitHubSpikeSkillFrontmatterMatchesPluginCopy(t *testing.T) {
	repoRoot := testRepoRoot(t)

	githubPath := filepath.Join(repoRoot, ".github", "skills", "spike", "SKILL.md")
	pluginPath := filepath.Join(repoRoot, "plugin", "skills", "spike", "SKILL.md")

	githubKeys := topLevelFrontmatterKeys(t, githubPath)
	pluginKeys := topLevelFrontmatterKeys(t, pluginPath)

	require.Contains(t, githubKeys, "name",
		".github/skills/spike/SKILL.md frontmatter must declare a top-level name: key to match the plugin copy")

	require.ElementsMatch(t, pluginKeys, githubKeys,
		".github and plugin spike SKILL.md must share identical top-level frontmatter keys")

	githubName := frontmatterStringValue(t, githubPath, "name")
	pluginName := frontmatterStringValue(t, pluginPath, "name")
	require.NotEmpty(t, pluginName, "plugin spike SKILL.md must declare a non-empty name:")
	require.Equal(t, pluginName, githubName,
		".github spike SKILL.md name: value must match the plugin copy")
}

// topLevelFrontmatterKeys parses the YAML frontmatter at path and returns its
// sorted set of top-level keys.
func topLevelFrontmatterKeys(t *testing.T, path string) []string {
	t.Helper()

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	frontmatter, _ := splitPluginFrontmatter(t, string(data), path)

	var doc map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(frontmatter), &doc), "parse YAML frontmatter: %s", path)

	keys := make([]string, 0, len(doc))
	for key := range doc {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// frontmatterStringValue returns the string value for key in the YAML
// frontmatter at path, or "" when the key is absent or non-scalar.
func frontmatterStringValue(t *testing.T, path string, key string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	frontmatter, _ := splitPluginFrontmatter(t, string(data), path)

	var doc map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(frontmatter), &doc), "parse YAML frontmatter: %s", path)

	value, ok := doc[key]
	if !ok {
		return ""
	}
	str, _ := value.(string)
	return strings.TrimSpace(str)
}
