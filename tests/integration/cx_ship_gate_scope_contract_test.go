package integration_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUCXS2_ShipGateScopeContract(t *testing.T) {
	repoRoot := testRepoRoot(t)
	readSource := func(relativePath string) string {
		t.Helper()

		content, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(relativePath)))
		require.NoError(t, err, "read contract source %s", relativePath)
		return string(content)
	}
	normalizeWhitespace := func(text string) string {
		return strings.Join(strings.Fields(text), " ")
	}
	sliceBetweenUniqueAnchors := func(text, startAnchor, endAnchor string) (string, bool) {
		if strings.Count(text, startAnchor) != 1 || strings.Count(text, endAnchor) != 1 {
			return "", false
		}
		start := strings.Index(text, startAnchor)
		end := strings.Index(text, endAnchor)
		if end <= start {
			return "", false
		}
		return text[start:end], true
	}
	assertContainsAll := func(t *testing.T, text string, literals []string, surface string) {
		t.Helper()

		missing := make([]string, 0)
		for _, literal := range literals {
			if !strings.Contains(text, literal) {
				missing = append(missing, literal)
			}
		}
		assert.Empty(t, missing, "%s is missing gate-scope contract literals: %q", surface, missing)
	}

	ship := normalizeWhitespace(readSource(".github/agents/_ship.agent.md"))

	scenarios := []struct {
		name string
		run  func(*testing.T)
	}{
		{
			name: "SnapshotFallbackArgv",
			run: func(t *testing.T) {
				item1, ok := sliceBetweenUniqueAnchors(ship,
					"1. **Snapshot the whole task wave set from live state.**",
					"2. **Halt on any unsupported status.**")
				require.True(t, ok, "Step 4.0 item 1 must have unique start and end anchors")
				assertContainsAll(t, item1, []string{
					`^[0-9]+\.[0-9]+-T$`,
					"discrete argv",
					"WAVE_SNAPSHOT_UNRELIABLE",
				}, "Step 4.0 item 1 snapshot fallback")
			},
		},
		{
			name: "ClosurePRGateScope",
			run: func(t *testing.T) {
				item5a, ok := sliceBetweenUniqueAnchors(ship,
					"5a. **TOPOLOGY_GATE: lifecycle (before PR creation)**",
					"6. Invoke the **pr-lifecycle** skill to create or update the pull request")
				require.True(t, ok, "Step 5 item 5a must have unique start and end anchors")
				assertContainsAll(t, item5a, []string{
					"does not apply to Step 6.0 closure PRs",
				}, "Step 5 item 5a lifecycle gate")

				item64, ok := sliceBetweenUniqueAnchors(ship,
					"4. **After all closure work is committed**",
					"5. **Await operator approval** for the closure PR")
				require.True(t, ok, "Step 6.0 item 4 must have unique start and end anchors")
				assertContainsAll(t, item64, []string{
					"does not apply to Step 6.0 closure PRs",
				}, "Step 6.0 item 4 closure PR gates")
			},
		},
	}
	for _, scenario := range scenarios {
		t.Run(scenario.name, scenario.run)
	}
}
