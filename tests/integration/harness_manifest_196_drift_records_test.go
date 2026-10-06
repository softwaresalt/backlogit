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

// TestUSR6_HarnessManifestDriftRecords pins the 196-F (U6, 196.006-T) harness-manifest drift
// records for the three installed artifacts edited by shipment 196-S.
//
// Stable 196-F contract: the citation assertions (227A2930 and/or B83081F5) and the
// checksum-differs-from-pre-reconciliation assertions must survive any later manifest
// re-render (for example the 731CE551 frontmatter model-routing re-render). This test does
// NOT recompute file hashes; checksum currency is verified once by the task's AC1 evidence.
func TestUSR6_HarnessManifestDriftRecords(t *testing.T) {
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
	tests := []struct {
		name          string
		path          string
		staleChecksum string
		citations     []string
	}{
		{
			name:          "OrchestratorAgent",
			path:          ".github/agents/_orchestrator.agent.md",
			staleChecksum: "00173cb089ea7f6f9f317874d96135b9636b5220ec3641c762c1ebbf304d0e73",
			citations:     []string{"227A2930"},
		},
		{
			name:          "ShipmentReconcileSkill",
			path:          ".github/skills/shipment-reconcile/SKILL.md",
			staleChecksum: "8f15392dd8ba09de6a90bb2cca63c4565444f4e4a5341f47fe4cd70595c824e3",
			citations:     []string{"B83081F5"},
		},
		{
			name:          "ShipAgent",
			path:          ".github/agents/_ship.agent.md",
			staleChecksum: "fd69f15a7cd3bd0ad76db4fe22cf085fcd65cf4caddd61833aeac68f11f1cd67",
			citations:     []string{"B83081F5", "227A2930"},
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
			assert.Regexp(t, checksumPattern, entry.Checksum,
				"checksum must be 64 lowercase hexadecimal characters for %s", test.path)
			// Stable 196-F contract: checksum must differ from the pre-reconciliation value.
			assert.NotEqual(t, test.staleChecksum, entry.Checksum,
				"checksum must differ from its pre-reconciliation value for %s", test.path)
			assert.True(t, strings.Contains(entry.DriftReason, "Do not auto-revert."),
				"drift_reason must contain 'Do not auto-revert.' for %s", test.path)
			// Stable 196-F contract: the drift_reason must cite the governing stash IDs.
			for _, citation := range test.citations {
				assert.True(t, strings.Contains(entry.DriftReason, citation),
					"drift_reason for %s must cite %s", test.path, citation)
			}
		})
	}
}
