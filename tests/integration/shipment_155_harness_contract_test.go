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
	shipToolList := strings.Split(shipTools, ",")
	for i := range shipToolList {
		shipToolList[i] = strings.TrimSpace(shipToolList[i])
	}
	// The backlogit workspace dogfoods its own MCP server, so Ship is granted
	// the full backlogit tool surface rather than a hand-maintained allowlist.
	require.Contains(t, shipToolList, "backlogit/*",
		"Ship must be granted the full backlogit MCP tool surface")
	for _, tool := range shipToolList {
		require.False(t, strings.HasPrefix(tool, "backlogit/backlogit_"),
			"Ship must not narrow the backlogit wildcard with explicit tool %s", tool)
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

// TestWorkspaceAgentsGrantFullBacklogitToolSurface guards the 2026-10-04
// operator directive: agents that mutate or dogfood this workspace receive
// every backlogit MCP operation through the backlogit/* wildcard.
func TestWorkspaceAgentsGrantFullBacklogitToolSurface(t *testing.T) {
	repoRoot := testRepoRoot(t)
	agents := []string{
		".github/agents/_orchestrator.agent.md",
		".github/agents/_stage.agent.md",
		".github/agents/_ship.agent.md",
		".github/agents/subagents/go-engineer.agent.md",
		".github/agents/subagents/prompt-builder.agent.md",
	}
	for _, agent := range agents {
		t.Run(agent, func(t *testing.T) {
			frontmatter := parseFrontmatterDoc(t, filepath.Join(repoRoot, filepath.FromSlash(agent)))
			tools, ok := frontmatter["tools"].(string)
			require.True(t, ok, "tools frontmatter must be a scalar string")
			toolList := strings.Split(tools, ",")
			for i := range toolList {
				toolList[i] = strings.TrimSpace(toolList[i])
			}
			require.Contains(t, toolList, "backlogit/*",
				"agent must be granted the full backlogit MCP tool surface")
			for _, tool := range toolList {
				require.False(t, strings.HasPrefix(tool, "backlogit/backlogit_"),
					"agent must not narrow the backlogit wildcard with explicit tool %s", tool)
			}
		})
	}
}
