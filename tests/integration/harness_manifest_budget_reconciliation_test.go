package integration_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestU19R3_ManifestBudgetReconciliation(t *testing.T) {
	type artifact struct {
		Path         string `yaml:"path"`
		Checksum     string `yaml:"checksum"`
		DriftAllowed bool   `yaml:"drift_allowed"`
		DriftReason  string `yaml:"drift_reason"`
	}

	var manifest struct {
		Artifacts []artifact `yaml:"artifacts"`
	}

	manifestPath := filepath.Join(testRepoRoot(t), ".autoharness", "harness-manifest.yaml")
	data, err := os.ReadFile(manifestPath)
	require.NoError(t, err)
	require.NoError(t, yaml.Unmarshal(data, &manifest))

	checksumPattern := regexp.MustCompile(`^[0-9a-f]{64}$`)
	driftSentence := "073-DL rev22: explicit governed full-suite budget `go test -timeout=30m ./...` (Steps 4.6/5, P-002.6, P-004)."
	tests := []struct {
		name           string
		path           string
		staleChecksum  string
	}{
		{
			name:          "workflow-policies",
			path:          ".github/policies/workflow-policies.md",
			staleChecksum: "d941c1c78f33a46c0afd9e743ccd606d3da4316cada7260871708c4b91d9290b",
		},
		{
			name:          "ship-agent",
			path:          ".github/agents/_ship.agent.md",
			staleChecksum: "d6b046f8d2299e8b146df95994fa7d5d6472c28eec56335a8e8988fbf242a3d9",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			matches := make([]artifact, 0, 1)
			for _, candidate := range manifest.Artifacts {
				if candidate.Path == test.path {
					matches = append(matches, candidate)
				}
			}

			require.Len(t, matches, 1, "manifest must contain exactly one entry for %s", test.path)

			entry := matches[0]
			assert.True(t, entry.DriftAllowed, "drift_allowed must be true for %s", test.path)
			assert.Equal(t, 1, strings.Count(entry.DriftReason, driftSentence),
				"drift_reason must contain the governed budget sentence exactly once for %s", test.path)
			assert.Regexp(t, checksumPattern, entry.Checksum,
				"checksum must be 64 lowercase hexadecimal characters for %s", test.path)
			assert.NotEqual(t, test.staleChecksum, entry.Checksum,
				"checksum must differ from its pre-reconciliation value for %s", test.path)
		})
	}
}
