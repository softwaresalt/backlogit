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
		})
	}
}
