package integration_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestShipment155HarnessContractsUseFlatMembershipAndGovernedRecovery(t *testing.T) {
	repoRoot := testRepoRoot(t)
	read := func(relativePath string) string {
		t.Helper()

		content, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(relativePath)))
		require.NoError(t, err)
		return string(content)
	}

	skill := read(".github/skills/shipment-reconcile/SKILL.md")
	ship := read(".github/agents/_ship.agent.md")
	orchestrator := read(".github/agents/_orchestrator.agent.md")
	policies := read(".github/policies/workflow-policies.md")
	normalize := func(content string) string {
		return strings.Join(strings.Fields(content), " ")
	}
	skillText := normalize(skill)
	shipText := normalize(ship)
	orchestratorText := normalize(orchestrator)
	policyText := normalize(policies)
	shipFrontmatter := parseFrontmatterDoc(t,
		filepath.Join(repoRoot, ".github", "agents", "_ship.agent.md"))
	shipTools, ok := shipFrontmatter["tools"].(string)
	require.True(t, ok, "Ship tools frontmatter must be a string")

	for _, operation := range []string{
		"backlogit_block_shipment",
		"backlogit_unblock_shipment",
		"backlogit_normalize_blocked_shipment",
	} {
		require.Contains(t, shipTools, operation,
			"Ship must declare governed shipment lifecycle operation %s", operation)
	}

	require.Contains(t, skillText, "Shipment membership is flat and explicit.")
	require.Contains(t, skillText, "The shipment control record is the only non-manifest transactional record")
	require.Contains(t, skillText, "Unlisted descendants and linked deliberations remain untouched")
	require.NotContains(t, skillText, "fully-covered-root")
	require.NotContains(t, skillText, "Cascade Close Sub-Procedure")

	require.Contains(t, shipText, "A listed feature does not imply shipment membership for any descendant or linked deliberation.")
	require.NotContains(t, shipText, "its entire terminal-status descendant subtree archives")

	require.Contains(t, policyText, "**Statement**: Shipment membership is flat and explicit.")
	require.Contains(t, policyText, "The shipment control record is the only non-manifest transactional record")
	require.NotContains(t, policyText, "**Subtree exception")

	require.Contains(t, orchestratorText, "`blocked` is a governed, resumable nonterminal shipment status")
	require.Contains(t, orchestratorText, "`backlogit_normalize_blocked_shipment`")
	require.Contains(t, orchestratorText, "MCP-only")
	require.NotContains(t, orchestratorText, "there is no shipment `blocked` lifecycle")
	require.Contains(t, orchestratorText, "no CLI fallback")
}
