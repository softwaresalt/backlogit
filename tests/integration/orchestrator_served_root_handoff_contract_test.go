package integration_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUSR1_OrchestratorServedRootHandoffContract(t *testing.T) {
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
		assert.Empty(t, missing, "%s is missing handoff contract literals: %q", surface, missing)
	}

	orchestrator := normalizeWhitespace(readSource(".github/agents/_orchestrator.agent.md"))

	scenarios := []struct {
		name string
		run  func(*testing.T)
	}{
		{
			name: "ProcedureLiterals",
			run: func(t *testing.T) {
				step2, ok := sliceBetweenUniqueAnchors(orchestrator, "### Step 2: Route to Ship", "### Step 3")
				require.True(t, ok, "Step 2 must have unique start and end anchors")

				procedureLiterals := []string{
					"#### Served-Root Handoff Procedure",
					"Every Orchestrator invocation of the Ship subagent runs the Served-Root Handoff Procedure first",
					"handoff precondition",
					"`served_workspace_root`",
					"`served_storage_root`",
					"`git rev-parse --show-toplevel`",
					"--git-common-dir",
					"main worktree",
					"exactly one of `.backlog` or `.backlogit`",
					"neither or both exist, fail closed",
					"symlink or reparse-point",
					"direct child of the served workspace root",
					"exactly one of `queue` or `archive`",
					"`backlogit_get_shipment`",
					"`custom_fields.items`",
					"Served-Root Attestation",
					"`backlogit_get_metadata_catalog`",
					"`workspace.root_path`",
					"`workspace.storage_root`",
					"`pragma_database_list`",
					"before any raw read",
					"exactly one row",
					"`backlogit.db` as a direct child",
					"missing, empty, or relative",
					"No sync or retry applies",
					"`SERVED_ROOTS_UNRESOLVED`",
					"any failure, error, or indeterminate result",
					"do not invoke Ship",
					"read-only except at most one derived-index",
					"workspace-relative form only",
					"Never infer either root from a Ship worktree",
				}
				assertContainsAll(t, step2, procedureLiterals, "Step 2")
			},
		},
		{
			name: "CallSites",
			run: func(t *testing.T) {
				step2, ok := sliceBetweenUniqueAnchors(orchestrator, "### Step 2: Route to Ship", "### Step 3")
				require.True(t, ok, "Step 2 must have unique start and end anchors")
				invoke := strings.Index(step2, "5. Invoke the **Ship** subagent:")
				procedure := strings.Index(step2, "Run the Served-Root Handoff Procedure")
				shipmentScope := strings.Index(step2, "Pass the `shipment_id` as the session scope")
				assert.True(t, invoke >= 0 && procedure > invoke && shipmentScope > procedure,
					"Step 2 must run the Served-Root Handoff Procedure after Ship invocation and before passing shipment scope")
				assertContainsAll(t, step2, []string{"`ship {id}` that names a non-queued shipment"}, "Step 2")

				recovery, ok := sliceBetweenUniqueAnchors(orchestrator,
					"`agent: ship` → invoke the **Ship** subagent", "The Orchestrator MUST NEVER execute")
				require.True(t, ok, "Ship recovery route must have unique start and end anchors")
				assertContainsAll(t, recovery, []string{"Served-Root Handoff Procedure"}, "Ship recovery route")

				blockedRoute, ok := sliceBetweenUniqueAnchors(orchestrator,
					"Route blocked shipments to Ship", "never performs those lifecycle mutations itself")
				require.True(t, ok, "blocked-shipment route must have unique start and end anchors")
				assertContainsAll(t, blockedRoute, []string{"Served-Root Handoff Procedure"}, "blocked-shipment route")

				payload := "only after the Served-Root Handoff Procedure passes, with `served_workspace_root`, `served_storage_root`, and the binding evidence in the Ship invocation payload"
				item5, ok := sliceBetweenUniqueAnchors(step2, "5. Invoke the **Ship** subagent:", "6. Receive Ship's output:")
				require.True(t, ok, "Step 2 item 5 must have unique start and end anchors")
				assertContainsAll(t, item5, []string{payload}, "Step 2 item 5 Ship payload")
				item5Run := strings.Index(item5, "Run the Served-Root Handoff Procedure")
				item5Payload := strings.Index(item5, payload)
				assert.True(t, item5Run >= 0 && item5Payload > item5Run, "item 5 must run the procedure before invoking Ship with the payload")
				assertContainsAll(t, step2, []string{"Include the binding evidence: `id`, ordered items, and the attested `workspace.root_path`, `workspace.storage_root`, and index `file`"}, "Step 2 binding evidence")
				assertContainsAll(t, recovery, []string{payload, "Use the checkpoint summary's shipment ID only", "do not read the state dump", "If the summary names no shipment, halt to the operator"}, "Ship recovery payload")
				assertContainsAll(t, blockedRoute, []string{payload}, "blocked-shipment payload")
			},
		},
		{
			name: "OutcomeRecordTiming",
			run: func(t *testing.T) {
				// Stable contract (PR #478 Copilot review): the R8 outcome record must never dirty
				// the main worktree ahead of Ship Step 0.5 item 3a, which halts on any
				// `git status --short` output when it creates the shipment branch from main.
				step2, ok := sliceBetweenUniqueAnchors(orchestrator, "### Step 2: Route to Ship", "### Step 3")
				require.True(t, ok, "Step 2 must have unique start and end anchors")
				assertContainsAll(t, step2, []string{
					"never in the main worktree before Ship is invoked",
					"Ship Step 0.5 item 3a halts on any `git status --short` output",
					"Carry the outcome in the Ship invocation payload instead, and persist it only once the shipment branch is checked out",
					"committed through the Step 1.5 Continuity Carry-Forward Carve-Out before any later Ship invocation",
				}, "step f outcome-record timing")

				item5, ok := sliceBetweenUniqueAnchors(step2, "5. Invoke the **Ship** subagent:", "6. Receive Ship's output:")
				require.True(t, ok, "Step 2 item 5 must have unique start and end anchors")
				assertContainsAll(t, item5, []string{"handoff outcome record"}, "Step 2 item 5 outcome payload")
				recovery, ok := sliceBetweenUniqueAnchors(orchestrator,
					"`agent: ship` → invoke the **Ship** subagent", "The Orchestrator MUST NEVER execute")
				require.True(t, ok, "Ship recovery route must have unique start and end anchors")
				assertContainsAll(t, recovery, []string{"step f handoff outcome record"}, "Ship recovery outcome payload")
				blockedRoute, ok := sliceBetweenUniqueAnchors(orchestrator,
					"Route blocked shipments to Ship", "never performs those lifecycle mutations itself")
				require.True(t, ok, "blocked-shipment route must have unique start and end anchors")
				assertContainsAll(t, blockedRoute, []string{"step f handoff outcome record"}, "blocked-shipment outcome payload")

				ship := normalizeWhitespace(readSource(".github/agents/_ship.agent.md"))
				gate, ok := sliceBetweenUniqueAnchors(ship, "3a. **Branch Creation Gate (P-011, NON-NEGOTIABLE)**", "4. If the shipment is still in `queued` status")
				require.True(t, ok, "Ship Step 0.5 item 3a must have unique start and end anchors")
				assertContainsAll(t, gate, []string{
					"served-root handoff outcome record",
					"only after `BRANCH_OK` or `BRANCH_CREATED`, never before",
					"commit it on the shipment branch",
				}, "Ship Step 0.5 item 3a outcome write")
			},
		},
		{
			name: "CrossReferenceInvariant",
			run: func(t *testing.T) {
				// Stable contract: later edits may restructure surrounding recovery prose.
				assert.Contains(t, orchestrator, "5. Invoke the **Ship** subagent:")
				assert.Contains(t, orchestrator, "proceed to step 4/5")
			},
		},
	}
	for _, scenario := range scenarios {
		t.Run(scenario.name, scenario.run)
	}
}
