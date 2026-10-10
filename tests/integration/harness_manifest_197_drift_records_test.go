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

// TestUCXS17_RefreshedDriftRecords pins the 197-F (U17, 197.019-T) harness-manifest drift records
// for the eight files refreshed by this release.
//
// Shape-and-pin contract: each entry must keep exactly one manifest record with drift_allowed
// true, a 64-character lowercase hex checksum that differs from the pre-refresh value pinned
// below, the drift_reason phrases and citations already present today, and the governing stash
// IDs. This test does NOT recompute file hashes and does NOT assert that a checksum equals the
// current file hash, so later edits to these files do not redden it.
func TestUCXS17_RefreshedDriftRecords(t *testing.T) {
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

	governingStashIDs := []string{
		"75E02C17", "52D18E44", "497D20E3", "67F17B6B",
		"FBD6E6F8", "8F1CF1E1", "6AB5E7FC", "F05661B1",
	}
	checksumPattern := regexp.MustCompile(`^[0-9a-f]{64}$`)

	tests := []struct {
		name          string
		path          string
		staleChecksum string
		preserved     []string
	}{
		{
			name:          "ShipAgent",
			path:          ".github/agents/_ship.agent.md",
			staleChecksum: "88930b6d78e495eda1df50aab2e0dd79b52c8b1ac3349dab55c459a07a98f69c",
			preserved:     []string{"Do not auto-revert.", "B83081F5", "227A2930"},
		},
		{
			name:          "OrchestratorAgent",
			path:          ".github/agents/_orchestrator.agent.md",
			staleChecksum: "2b109518d981e2e980970b590c682ab66c01c3f20175b29396520a803b151cd7",
			preserved:     []string{"Do not auto-revert.", "227A2930"},
		},
		{
			name:          "AcquireLockPowerShell",
			path:          "scripts/acquire_lock.ps1",
			staleChecksum: "926873500147222d2dc195f61551d4458a8a0be099d4759c63418dee86f82645",
			preserved:     nil,
		},
		{
			name:          "ReleaseLockPowerShell",
			path:          "scripts/release_lock.ps1",
			staleChecksum: "f4ec95d0da608f022de2974ee5a4afb05dc976b72d949e2dc543ed0d1c91a53f",
			preserved:     nil,
		},
		{
			name:          "AcquireLockBash",
			path:          "scripts/acquire_lock.sh",
			staleChecksum: "b8b6d07eb8e5fb2e5da1e05a7c80a53e3b435f86694a6d7c7eb06db40126d27f",
			preserved:     nil,
		},
		{
			name:          "ReleaseLockBash",
			path:          "scripts/release_lock.sh",
			staleChecksum: "aa0a4d5139769580f42f396a9fbc017fcbbe2d7ff66317fd537463b0bab62ff8",
			preserved:     nil,
		},
		{
			name:          "ConcurrencyInstructions",
			path:          ".github/instructions/concurrency.instructions.md",
			staleChecksum: "e2d15fe60339ac12251bd6883d982e0aa2059c4ab3f88215fc089f2e21fdd26f",
			preserved:     nil,
		},
		{
			name:          "FileLockSkill",
			path:          ".github/skills/file-lock/SKILL.md",
			staleChecksum: "f03ee9d27ea1e7e2264262107664f83e97746c05d778b97c27ba513449866f4b",
			preserved:     nil,
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
			assert.NotEqual(t, test.staleChecksum, entry.Checksum,
				"checksum must differ from its pre-refresh value for %s", test.path)

			var missingPreserved []string
			for _, literal := range test.preserved {
				if !strings.Contains(entry.DriftReason, literal) {
					missingPreserved = append(missingPreserved, literal)
				}
			}
			assert.Empty(t, missingPreserved,
				"drift_reason for %s must keep its existing phrases and citations", test.path)

			var missingStashIDs []string
			for _, stashID := range governingStashIDs {
				if !strings.Contains(entry.DriftReason, stashID) {
					missingStashIDs = append(missingStashIDs, stashID)
				}
			}
			assert.Empty(t, missingStashIDs,
				"drift_reason for %s must cite the governing stash IDs", test.path)
		})
	}
}
