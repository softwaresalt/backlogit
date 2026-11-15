package integration_test

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func lfSHA256Hex(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func TestU12_HarnessManifestNamespaceReconciliation(t *testing.T) {
	// Non-vacuity negative self-test: duplicate YAML keys must be rejected.
	var dupDoc yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte("a: 1\na: 2\n"), &dupDoc))
	require.Error(t, checkUniqueYAMLMappingKeys(&dupDoc), "duplicate YAML key must be rejected")

	repoRoot := testRepoRoot(t)
	rawManifest := readGovernedSurface(t, repoRoot, ".autoharness/harness-manifest.yaml")

	var doc yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(rawManifest), &doc))
	require.NoError(t, checkUniqueYAMLMappingKeys(&doc), "harness-manifest.yaml must not contain duplicate YAML keys")

	var manifest map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(rawManifest), &manifest))

	assert.Equal(t, "1.1.0", manifest["schema_version"])
	assert.Equal(t, "1.6.0", manifest["autoharness_version"])
	assert.Equal(t, "full", manifest["install_preset"])

	vars, ok := manifest["variables_used"].(map[string]any)
	require.True(t, ok, "variables_used must exist")
	assert.Equal(t, "plans", vars["DOCS_PLANS_DIR"])
	assert.Equal(t, "docs", vars["DOCS_ROOT"])
	assert.Equal(t, "compound", vars["DOCS_COMPOUND_DIR"])

	rawConfig := readGovernedSurface(t, repoRoot, ".autoharness/config.yaml")
	wantConfigHash := lfSHA256Hex(rawConfig)
	assert.Equal(t, wantConfigHash, manifest["config_hash"], "config_hash must match LF-normalized .autoharness/config.yaml")
	assert.NotEqual(t, "f37e5b412dd024201601e1ba5d8451c1356cd8e88f22116645835bb47407e9c7", manifest["config_hash"], "config_hash must differ from pre-215-S hash")

	// Historical notes must be preserved unchanged.
	assert.Contains(t, rawManifest, "DOCS_PLANS corrected to docs/exec-plans per workspace config.")

	artifactsRaw, ok := manifest["artifacts"].([]any)
	require.True(t, ok, "artifacts list must exist")
	byPath := make(map[string]map[string]any, len(artifactsRaw))
	for _, entry := range artifactsRaw {
		m, ok := entry.(map[string]any)
		require.True(t, ok, "artifact entry must be a map")
		p, _ := m["path"].(string)
		byPath[p] = m
	}

	modifiedArtifacts := []struct {
		path    string
		pre215S string
	}{
		{"AGENTS.md", "e2713b69ad133b1bc4aadcff3bb8d24fe64eb666c0449346a615fcd3d6b72d6e"},
		{".github/copilot-instructions.md", "f798aab688486ebf075b1a64af572850626b14843eb1bb21ccc76a0665f2c4bc"},
		{".github/instructions/backlogit.instructions.md", "22fd08ec8a21dec45067ad703e60fa2a64f4de4c38ac20db574531e05ce751dc"},
		{".github/instructions/architecture-doc.instructions.md", "2894f4fac16ad83b703927991e751e0df28b7d8ec58373f6ecf76019fd9c82ca"},
		{".github/skills/plan-harden/SKILL.md", "2d6d6e0496356ba718a61b6365c65753b449ce2330647a3902154eec9db285ff"},
		{".github/skills/spike/SKILL.md", "2a6adbf66ebf5a791552bf6de5093712f97162f66bcd482abf6fe51159d05ae4"},
	}

	for _, item := range modifiedArtifacts {
		item := item
		t.Run(item.path, func(t *testing.T) {
			entry, exists := byPath[item.path]
			require.True(t, exists, "manifest entry must exist for %s", item.path)
			diskContent := readGovernedSurface(t, repoRoot, item.path)
			wantChecksum := lfSHA256Hex(diskContent)
			gotChecksum, _ := entry["checksum"].(string)
			assert.Equal(t, wantChecksum, gotChecksum, "manifest checksum must match LF-normalized %s", item.path)
			assert.NotEqual(t, item.pre215S, gotChecksum, "manifest checksum must differ from pre-215-S checksum for %s", item.path)
		})
	}
}
