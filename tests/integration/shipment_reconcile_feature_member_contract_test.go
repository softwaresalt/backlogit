package integration_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUSR3_ShipmentReconcileExplicitFeatureMemberContract(t *testing.T) {
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
		assert.Empty(t, missing, "%s is missing explicit-feature contract literals: %q", surface, missing)
	}

	skill := normalizeWhitespace(readSource(".github/skills/shipment-reconcile/SKILL.md"))
	ship := normalizeWhitespace(readSource(".github/agents/_ship.agent.md"))

	scenarios := []struct {
		name string
		run  func(*testing.T)
	}{
		{
			name: "SkillLiterals",
			run: func(t *testing.T) {
				assertContainsAll(t, skill, []string{
					"`feature-pending-governed-completion`",
					"every explicit task member is `matched` or `pre-archived`",
					"applies only to pre-close (`expected_status: done`)",
					"A manifest with no explicit task member does not qualify",
					"Any other explicit feature status remains `status-mismatch`",
					"Safe-close re-checks this condition on the state re-read under lock",
					"Explicit feature members are completed only by governed ShipShipment (`backlogit_ship_shipment` or its registered CLI fallback)",
				}, "shipment-reconcile skill")
			},
		},
		{
			name: "ProceedAndShipStep6",
			run: func(t *testing.T) {
				proceed, ok := sliceBetweenUniqueAnchors(skill, "* `PROCEED`:", "* `PAUSED")
				require.True(t, ok, "PROCEED recommendation must have unique start and end anchors")
				assertContainsAll(t, proceed, []string{
					"every explicit member is `matched`, `pre-archived`, or `feature-pending-governed-completion`",
				}, "PROCEED recommendation")

				step6Close, ok := sliceBetweenUniqueAnchors(ship,
					"Pre-archive reconciliation gate", "b. Call `backlogit_ship_shipment` with the merge commit SHA")
				require.True(t, ok, "Ship Step 6 close path must have unique start and end anchors")
				assertContainsAll(t, step6Close, []string{"feature-pending-governed-completion"}, "Ship Step 6 close path")
			},
		},
		{
			name: "SupersededAndPreserved",
			run: func(t *testing.T) {
				assert.False(t, strings.Contains(skill, "require every non-pre-archived explicit member to be `done`"),
					"superseded close-readiness sentence must be absent")
				memberClassifications, ok := sliceBetweenUniqueAnchors(skill,
					"Each explicit manifest member receives exactly one classification:",
					"### Shipment Record Classification")
				require.True(t, ok, "member-classification table must have unique start and end anchors")
				assert.Contains(t, memberClassifications, "Archive record exists with valid provenance",
					"post-mode archive-provenance classification must remain")
				safeClose, ok := sliceBetweenUniqueAnchors(skill, "### Safe-Close Mode", "### Post-Mode")
				require.True(t, ok, "safe-close protocol must have unique start and end anchors")
				assert.Contains(t, safeClose,
					"require every member/control record that was not already validly archived at baseline to appear in `A`",
					"safe-close Step 5 archive-set invariant must remain")

				closeReady, ok := sliceBetweenUniqueAnchors(skill, "2. **Require close-ready state.**", "3. **Capture the baseline.**")
				require.True(t, ok, "Safe-Close step 2 must have unique start and end anchors")
				assertContainsAll(t, closeReady, []string{
					"every non-pre-archived explicit task member must be `done`",
					"a non-pre-archived explicit feature member must be `done` or meet the `feature-pending-governed-completion` condition",
					"any other non-pre-archived explicit member must be `done`",
					"any other status of a non-pre-archived explicit feature member halts before mutation",
					"Safe-close re-checks this condition on the state re-read under lock",
				}, "Safe-Close step 2 acceptance rule")
				assert.NotContains(t, closeReady, "every non-pre-archived explicit member", "Safe-Close step 2 must not require every explicit member to be done")
			},
		},
	}
	for _, scenario := range scenarios {
		t.Run(scenario.name, scenario.run)
	}
}
