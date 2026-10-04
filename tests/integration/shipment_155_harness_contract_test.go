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
// operator directive: every active agent that can mutate or dogfood this
// workspace receives every backlogit MCP operation through the backlogit/*
// wildcard. Only agents whose declared tools are all read-only (read, search,
// and indexed-retrieval MCP surfaces) are exempt. No agent may narrow the
// wildcard with explicit backlogit/backlogit_* entries.
func TestWorkspaceAgentsGrantFullBacklogitToolSurface(t *testing.T) {
	repoRoot := testRepoRoot(t)
	agentsRoot := filepath.Join(repoRoot, ".github", "agents")
	readOnlyTools := map[string]bool{"read": true, "search": true}
	readOnlyPrefixes := []string{"engram/", "graphtor-docs/", "backlogit/"}
	isReadOnly := func(tool string) bool {
		if readOnlyTools[tool] {
			return true
		}
		for _, prefix := range readOnlyPrefixes {
			if strings.HasPrefix(tool, prefix) {
				return true
			}
		}
		return false
	}

	var checked []string
	err := filepath.WalkDir(agentsRoot, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if d.Name() == "deprecated" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".agent.md") {
			return nil
		}
		rel, relErr := filepath.Rel(repoRoot, path)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		t.Run(rel, func(t *testing.T) {
			frontmatter := parseFrontmatterDoc(t, path)
			rawTools, declared := frontmatter["tools"]
			if !declared {
				// No tools declaration grants the agent every available tool.
				return
			}
			var toolList []string
			switch tools := rawTools.(type) {
			case string:
				toolList = strings.Split(tools, ",")
			case []any:
				for _, tool := range tools {
					name, ok := tool.(string)
					require.True(t, ok, "tools entries must be strings")
					toolList = append(toolList, name)
				}
			default:
				require.Failf(t, "unsupported tools frontmatter", "type %T", rawTools)
			}
			mutating := false
			for i := range toolList {
				toolList[i] = strings.TrimSpace(toolList[i])
				require.False(t, strings.HasPrefix(toolList[i], "backlogit/backlogit_"),
					"agent must not narrow the backlogit wildcard with explicit tool %s", toolList[i])
				if !isReadOnly(toolList[i]) {
					mutating = true
				}
			}
			if mutating {
				require.Contains(t, toolList, "backlogit/*",
					"agent with mutating tools must be granted the full backlogit MCP tool surface")
				checked = append(checked, rel)
			}
		})
		return nil
	})
	require.NoError(t, err)

	for _, agent := range []string{
		".github/agents/_orchestrator.agent.md",
		".github/agents/_stage.agent.md",
		".github/agents/_ship.agent.md",
		".github/agents/subagents/go-engineer.agent.md",
		".github/agents/subagents/prompt-builder.agent.md",
		".github/agents/review/adversarial-review.agent.md",
		".github/agents/review/security-sentinel.agent.md",
		".github/agents/subagents/security-sentinel.agent.md",
	} {
		require.Contains(t, checked, agent, "agent must be covered by the backlogit tool-surface inventory")
	}
}
