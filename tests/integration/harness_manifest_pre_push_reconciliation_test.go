package integration_test

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestU177_004_ManifestPrePushChecksumReconciliation(t *testing.T) {
	type artifact struct {
		Path         string `yaml:"path"`
		Checksum     string `yaml:"checksum"`
		DriftAllowed bool   `yaml:"drift_allowed"`
		DriftReason  string `yaml:"drift_reason"`
	}
	var manifest struct {
		Artifacts []artifact `yaml:"artifacts"`
	}

	repoRoot := testRepoRoot(t)
	data, err := os.ReadFile(filepath.Join(repoRoot, ".autoharness", "harness-manifest.yaml"))
	require.NoError(t, err)
	require.NoError(t, yaml.Unmarshal(data, &manifest))

	driftSentence := "073-DL E2: pre-push Test gate runs the governed full-suite budget `go test -timeout=30m ./...`."
	tests := []struct {
		name          string
		path          string
		staleChecksum string
		reasonPrefix  string
	}{
		{
			name:          "pre-push-ps1",
			path:          "scripts/pre-push-quality-gates.ps1",
			staleChecksum: "7dea315196283603e96fb3c282a3daf0a630d885cbeff0f41342482e4c465e4b",
			reasonPrefix:  "Copilot review fix (PR #400).",
		},
		{
			name:          "pre-push-sh",
			path:          "scripts/pre-push-quality-gates.sh",
			staleChecksum: "0b9fcac0da7eaf85a25e89b659dc6e8bb1daa188edf7aea1d134e8e6f1585758",
			reasonPrefix:  "Copilot review fix (PR #400).",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var matches []artifact
			for _, candidate := range manifest.Artifacts {
				if candidate.Path == test.path {
					matches = append(matches, candidate)
				}
			}
			require.Len(t, matches, 1, "manifest must contain exactly one entry for %s", test.path)
			entry := matches[0]

			script, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(test.path)))
			require.NoError(t, err)
			// The manifest checksums the committed (LF) form of the script.
			normalized := strings.ReplaceAll(string(script), "\r\n", "\n")
			sum := sha256.Sum256([]byte(normalized))

			assert.True(t, entry.DriftAllowed, "drift_allowed must stay true for %s", test.path)
			assert.NotEqual(t, test.staleChecksum, entry.Checksum, "checksum must rotate away from its pre-reconciliation value for %s", test.path)
			assert.Equal(t, hex.EncodeToString(sum[:]), entry.Checksum, "checksum must match the post-177.002-T script for %s", test.path)
			assert.Contains(t, entry.DriftReason, test.reasonPrefix, "existing PR #400 drift reason must be preserved for %s", test.path)
			assert.Equal(t, 1, strings.Count(entry.DriftReason, driftSentence), "drift_reason must contain the 073-DL budget sentence exactly once for %s", test.path)
		})
	}
}
