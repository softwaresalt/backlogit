package integration_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUSR8_ShipServedRootAttestationContract(t *testing.T) {
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
		if start < 0 || end <= start {
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
		assert.Empty(t, missing, "%s is missing served-root attestation literals: %q", surface, missing)
	}

	ship := normalizeWhitespace(readSource(".github/agents/_ship.agent.md"))

	scenarios := []struct {
		name string
		run  func(*testing.T)
	}{
		{
			name: "DefinitionLiterals",
			run: func(t *testing.T) {
				step41b, ok := sliceBetweenUniqueAnchors(ship, "#### Step 4.1b: Claim Task", "#### Step 4.1c")
				require.True(t, ok, "Ship Step 4.1b must have unique start and end anchors")

				attestation := strings.Index(step41b, "**Served-Root Attestation (shared read-only procedure):**")
				rawLogSafety := strings.Index(step41b, "**Raw-log path safety (shared read-only procedure):**")
				assert.True(t, attestation >= 0 && rawLogSafety > attestation,
					"served-root attestation must precede raw-log path safety")

				assertContainsAll(t, step41b, []string{
					"never treats roots or binding evidence passed by the Orchestrator as proof",
					"`backlogit_get_metadata_catalog`",
					"`workspace.root_path`",
					"`workspace.storage_root`",
					"`pragma_database_list`",
					"exactly one row",
					"`backlogit.db` as a direct child",
					"missing, empty, or relative",
					"No sync or retry applies",
					"there is no CLI attestation",
					"once per wave admission, before any raw item-log read",
					"at each task claim, before that task's first raw item-log read",
					"immediately before any CLI fallback that passes the served workspace root",
					"`SERVED_ROOT_ATTESTATION_FAILED`",
					"before any claim, raw item-log read, or CLI fallback",
				}, "Ship Step 4.1b")
				assertContainsAll(t, step41b, []string{
					"`workspace.root_path` must equal the canonical served workspace root",
					"`workspace.storage_root` must equal the canonical served storage root",
					"SELECT name, file FROM pragma_database_list WHERE name = 'main'",
					"exactly one row whose `file` is `backlogit.db` as a direct child of the canonical served storage root",
					"or mismatch halts with `SERVED_ROOT_ATTESTATION_FAILED: {reason}` before any claim, raw item-log read, or CLI fallback",
				}, "Ship Step 4.1b root equality")
			},
		},
		{
			name: "CallSites",
			run: func(t *testing.T) {
				step40Item4, ok := sliceBetweenUniqueAnchors(ship,
					"4. **Classify active members before any other active check.**",
					"5. **Check for completion.**")
				require.True(t, ok, "Ship Step 4.0 item 4 must have unique start and end anchors")
				assertContainsAll(t, step40Item4, []string{
					"Served-Root Attestation",
					"SERVED_ROOT_ATTESTATION_FAILED",
				}, "Ship Step 4.0 item 4")

				appendFallback, ok := sliceBetweenUniqueAnchors(ship,
					"4. If the MCP append errors", "5. Re-read the item log")
				require.True(t, ok, "Ship Step 4.1b fallback path must have unique start and end anchors")
				attestation := strings.Index(appendFallback, "Served-Root Attestation")
				reRead := strings.Index(appendFallback, "re-reads the item log before any CLI fallback")
				cliFallback := strings.Index(appendFallback, "`backlogit comment add")
				assert.True(t, attestation >= 0 && reRead > attestation && cliFallback > attestation,
					"fallback attestation must precede the item-log re-read and CLI fallback")
				rootReq40 := strings.Index(step40Item4, "Require both served roots before wave admission.")
				attest40 := strings.Index(step40Item4, "Served-Root Attestation")
				rawRead40 := strings.Index(step40Item4, "and `logs/<id>.jsonl` under the served storage root")
				assert.True(t, rootReq40 >= 0 && attest40 > rootReq40 && rawRead40 > attest40, "Step 4.0 attestation must follow the root requirement and precede the first raw item-log read")
				assertContainsAll(t, step40Item4, []string{
					"A failure halts with `SERVED_ROOT_ATTESTATION_FAILED` before any claim or raw item-log read",
				}, "Ship Step 4.0 item 4 failure scope")
				assertContainsAll(t, appendFallback, []string{
					"If it fails, halt with `SERVED_ROOT_ATTESTATION_FAILED` and do not use the CLI fallback",
				}, "Ship Step 4.1b fallback halt")
				assert.True(t, attestation >= 0 && reRead > attestation && cliFallback > reRead, "fallback item-log re-read must precede the CLI fallback")
			},
		},
		{
			name: "PreservedInvariants",
			run: func(t *testing.T) {
				// Stable contract: served-root attestation must not replace raw-log safety or claim guards.
				for _, literal := range []string{
					"**Raw-log path safety (shared read-only procedure):**",
					"WAVE_CLAIM_STATE_INDETERMINATE",
					"Never infer either root",
					"re-reads the item log before any CLI fallback",
				} {
					assert.Contains(t, ship, literal, "Ship must preserve %q", literal)
				}
			},
		},
	}
	for _, scenario := range scenarios {
		t.Run(scenario.name, scenario.run)
	}
}
