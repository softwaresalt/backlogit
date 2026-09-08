package integration_test

// Harness-defect regression guard: after every successful PR merge to main,
// Ship must automatically return the existing local worktree to `main`,
// fast-forward it with an ff-only pull, verify HEAD == origin/main, and only
// then create any post-merge closure branch. The operator must never have to
// remind the agent to do this.
//
// This is the RED-phase test for the post-merge main-sync correction. Before
// the fix it fails because:
//   - .github/skills/pr-lifecycle/SKILL.md Step 6 tells Ship "Do NOT checkout
//     `main` and start working on it" (the contradictory post-merge rule), and
//   - neither the pr-lifecycle skill nor the Ship agent encodes the ordered,
//     fast-forward-only sync sequence.
//
// It passes once the authoritative plugin sources and their installed .github
// copies both encode the invariant:
//   MERGE_SUCCEEDED -> safe switch main -> ff-only sync -> SHA equality
//   verification -> optional post-merge branch.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// postMergeSyncInvariant is the canonical one-line protocol summary that every
// governing surface must embed verbatim so operators and future edits share a
// single, greppable contract.
const postMergeSyncInvariant = "MERGE_SUCCEEDED -> safe switch main -> ff-only sync -> SHA equality verification -> optional post-merge branch"

// globalPostMergeSyncInvariant is the generalized, cross-workflow contract that
// MUST live in the authoritative Git-merge instruction (not solely in
// Ship/pr-lifecycle). It applies to ANY successful merge into `main`, regardless
// of role or PR class.
const globalPostMergeSyncInvariant = "ANY MERGE_SUCCEEDED to main -> safe local main switch -> ff-only origin/main sync -> SHA equality -> next steps"

// gitMergeAuthoritySurface is the authoritative cross-workflow merge instruction
// that owns the base synchronization invariant. Ship/pr-lifecycle surfaces
// operationalize it and MUST reference it by name.
const gitMergeAuthoritySurface = ".github/instructions/git-merge.instructions.md"

// readGovernedSurface reads a repo-root-relative markdown surface and returns
// its content with CRLF normalised to LF so assertions are line-ending
// agnostic across Windows and POSIX checkouts.
func readGovernedSurface(t *testing.T, repoRoot, relPath string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(relPath)))
	require.NoError(t, err, "governed surface must exist: %s", relPath)
	return strings.ReplaceAll(string(data), "\r\n", "\n")
}

// TestPRLifecycleSkillEncodesPostMergeMainSync guards both the installed
// (.github) and authoritative (plugin) pr-lifecycle skill copies.
func TestPRLifecycleSkillEncodesPostMergeMainSync(t *testing.T) {
	repoRoot := testRepoRoot(t)

	surfaces := []string{
		".github/skills/pr-lifecycle/SKILL.md",
		"plugin/skills/pr-lifecycle/SKILL.md",
	}

	for _, rel := range surfaces {
		t.Run(rel, func(t *testing.T) {
			content := readGovernedSurface(t, repoRoot, rel)

			// The contradictory post-merge rule must be gone. This is scoped to
			// the exact post-merge phrasing so the legitimate pre-merge branch
			// retention rule ("Do NOT checkout `main`" while awaiting approval)
			// is not disturbed.
			assert.NotContains(t, content, "Do NOT checkout `main` and start working on it",
				"post-merge cleanup must no longer forbid returning to main")

			// The ordered fast-forward-only sync sequence must be present.
			assert.Contains(t, content, postMergeSyncInvariant,
				"pr-lifecycle must embed the post-merge sync invariant")
			assert.Contains(t, content, "git pull --ff-only origin main",
				"pr-lifecycle must require a fast-forward-only pull of main")
			assert.Contains(t, content, "HEAD == origin/main",
				"pr-lifecycle must require verifying HEAD equals origin/main")

			// Ship/pr-lifecycle must stay coupled to the global authority: they
			// operationalize the base invariant and reference it by name so the
			// two surfaces cannot drift into a contradictory rule.
			assert.Contains(t, content, "git-merge.instructions.md",
				"pr-lifecycle must reference the authoritative cross-workflow git-merge instruction")
		})
	}
}

// TestShipAgentEncodesPostMergeMainSync guards both the installed (.github)
// and authoritative (plugin) Ship agent copies.
func TestShipAgentEncodesPostMergeMainSync(t *testing.T) {
	repoRoot := testRepoRoot(t)

	surfaces := []string{
		".github/agents/_ship.agent.md",
		"plugin/agents/ship.agent.md",
	}

	for _, rel := range surfaces {
		t.Run(rel, func(t *testing.T) {
			content := readGovernedSurface(t, repoRoot, rel)

			assert.Contains(t, content, postMergeSyncInvariant,
				"Ship agent must embed the post-merge sync invariant")
			assert.Contains(t, content, "git pull --ff-only origin main",
				"Ship agent must require a fast-forward-only pull of main after merge")
			assert.Contains(t, content, "HEAD == origin/main",
				"Ship agent must verify HEAD equals origin/main and record the synchronized SHA")

			assert.Contains(t, content, "git-merge.instructions.md",
				"Ship agent must reference the authoritative cross-workflow git-merge instruction")
		})
	}
}

// TestPluginShipAgentCreatesClosureBranchBeforeShipMutation guards the plugin
// Ship agent's safety guarantee that the `post-merge/` closure branch is created
// before any state-mutating closure step (notably `backlogit_ship_shipment`), so
// plugin users never mutate the protected default branch. A regression that moves
// the shipment mutation ahead of branch creation must fail this test.
func TestPluginShipAgentCreatesClosureBranchBeforeShipMutation(t *testing.T) {
	repoRoot := testRepoRoot(t)
	content := readGovernedSurface(t, repoRoot, "plugin/agents/ship.agent.md")

	branchIdx := strings.Index(content, "post-merge/{feature_slug}")
	shipIdx := strings.Index(content, "backlogit_ship_shipment")
	require.NotEqual(t, -1, branchIdx, "plugin Ship agent must create a post-merge closure branch")
	require.NotEqual(t, -1, shipIdx, "plugin Ship agent must call backlogit_ship_shipment")
	assert.Less(t, branchIdx, shipIdx,
		"plugin Ship agent must create the post-merge closure branch before the backlogit_ship_shipment mutation")
}

// TestGitMergeInstructionEncodesGlobalPostMergeMainSync guards the authoritative
// cross-workflow merge instruction. The base synchronization invariant must live
// here — not solely in Ship/pr-lifecycle — so it governs every merge-capable
// workflow (Ship, PR lifecycle, staging/planning, corrective, closure, elective,
// or any future merge-capable workflow) regardless of role or PR class.
func TestGitMergeInstructionEncodesGlobalPostMergeMainSync(t *testing.T) {
	repoRoot := testRepoRoot(t)

	content := readGovernedSurface(t, repoRoot, gitMergeAuthoritySurface)

	// The authority only governs "every merge-capable workflow" if it is actually
	// loaded repository-wide. Without applyTo: '**' the section would only reach the
	// explicitly-edited Ship/pr-lifecycle surfaces, defeating the global claim.
	assert.Contains(t, content, "applyTo: '**'",
		"git-merge instruction must be loaded repository-wide via applyTo: '**'")

	// (a) Global applicability to every successful merge to main.
	assert.Contains(t, content, globalPostMergeSyncInvariant,
		"git-merge instruction must embed the generalized global sync invariant")
	assert.Contains(t, content, "regardless of role or PR class",
		"git-merge instruction must state the rule applies regardless of role or PR class")

	// (b) Exact ordered ff-only sync and SHA equality.
	assert.Contains(t, content, "git pull --ff-only origin main",
		"git-merge instruction must require a fast-forward-only pull of main")
	assert.Contains(t, content, "HEAD == origin/main",
		"git-merge instruction must require verifying HEAD equals origin/main")

	// (c) Fail-closed blocked state.
	assert.Contains(t, content, "POST_MERGE_SYNC_BLOCKED",
		"git-merge instruction must define the fail-closed blocked state")

	// (d) Next-step prohibition before sync completes.
	assert.Contains(t, content, "before any next step",
		"git-merge instruction must prohibit next steps before the sync completes")

	// (e) Trigger is MERGE_SUCCEEDED only — failures/approved-unmerged do not fire.
	assert.Contains(t, content, "only after a confirmed",
		"git-merge instruction must run only after a confirmed MERGE_SUCCEEDED")
	assert.Contains(t, content, "MERGE_SUCCEEDED",
		"git-merge instruction must key the trigger on MERGE_SUCCEEDED")

	// (f) The ordered sequence must actually be ordered: fetch -> checkout ->
	// ff-only pull -> SHA equality, and the working tree must be recorded first.
	orderedTokens := []string{
		"git status --porcelain",
		"git fetch origin main:refs/remotes/origin/main",
		"git checkout main",
		"git pull --ff-only origin main",
		"HEAD == origin/main",
	}
	prev := -1
	for _, tok := range orderedTokens {
		idx := strings.Index(content, tok)
		require.NotEqual(t, -1, idx, "git-merge instruction must contain ordered step token: %s", tok)
		assert.Greater(t, idx, prev, "git-merge step %q must appear after the previous step", tok)
		prev = idx
	}

	// (g) Destructive shortcuts must be explicitly forbidden so a surface cannot
	// regress to stash/reset/discard/clean/force-push.
	for _, forbidden := range []string{"stash", "reset", "rebase", "discard", "git clean", "force-push"} {
		assert.Contains(t, content, forbidden,
			"git-merge instruction must explicitly forbid the destructive shortcut: %s", forbidden)
	}
}
