package core

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// initGitRepoWithCommits initializes a real git repo in dir (which must be the
// same directory as the Workspace.RootPath the helper under test reads via
// cmd.Dir) with a small deterministic history:
//
//	A (main) --> B (main, descendant)
//	 \
//	  D (branch "divergent", sibling off A)
//
// A is an ancestor of B; D is a real sibling commit that is NOT an ancestor of B
// (so `git merge-base --is-ancestor D B` exits 1, the definitive not-an-ancestor
// signal, distinct from an exit-128 bad-object error). Returns (A, B, D). The
// test is skipped when git is not on PATH.
func initGitRepoWithCommits(t *testing.T, dir string) (base, head, divergent string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available on PATH")
	}
	// Set BOTH author and committer identity so commits never fail with an
	// "empty ident" error on a clean CI runner with no global git config.
	identity := append(os.Environ(),
		"GIT_AUTHOR_NAME=Ancestry Test", "GIT_AUTHOR_EMAIL=ancestry@example.com",
		"GIT_COMMITTER_NAME=Ancestry Test", "GIT_COMMITTER_EMAIL=ancestry@example.com",
	)
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = identity
		out, err := cmd.CombinedOutput()
		require.NoErrorf(t, err, "git %s: %s", strings.Join(args, " "), out)
		return strings.TrimSpace(string(out))
	}
	writeFile := func(name, content string) {
		t.Helper()
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644))
	}

	run("-c", "init.defaultBranch=main", "init")
	// Belt-and-suspenders local identity in addition to the env above.
	run("config", "user.name", "Ancestry Test")
	run("config", "user.email", "ancestry@example.com")

	writeFile("f.txt", "a\n")
	run("add", "f.txt")
	run("commit", "-m", "A")
	base = run("rev-parse", "HEAD")

	writeFile("f.txt", "a\nb\n")
	run("add", "f.txt")
	run("commit", "-m", "B")
	head = run("rev-parse", "HEAD")

	// Divergent sibling branched from A: a real commit that is not reachable
	// from B, so --is-ancestor exits 1 (not 128).
	run("checkout", "-b", "divergent", base)
	writeFile("g.txt", "d\n")
	run("add", "g.txt")
	run("commit", "-m", "D")
	divergent = run("rev-parse", "HEAD")

	run("checkout", "main")
	return base, head, divergent
}

// initGitRepoNoCommits initializes a real git repo in dir with NO commit (an
// unborn branch): `git rev-parse --is-inside-work-tree` reports `true` (a real
// work tree) while `git rev-parse HEAD` fails (no commit yet), so it is the
// load-bearing fixture for the empty-shipment-head-in-a-real-worktree case
// (1AEA2B0E). The test is skipped when git is not on PATH (mirrors
// initGitRepoWithCommits).
func initGitRepoNoCommits(t *testing.T, dir string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available on PATH")
	}
	cmd := exec.Command("git", "-c", "init.defaultBranch=main", "init")
	cmd.Dir = dir
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	require.NoErrorf(t, err, "git init: %s", out)
}

// TestInGitWorktreeBounded exercises the bounded repo-presence discriminator: a
// real work tree reports true; a no-repo temp dir reports (false, nil) so the
// legacy skip is preserved; and an already-expired bounded context fails CLOSED
// with a non-nil ctx error (never a silent false-negative that would re-open the
// empty-head hole).
func TestInGitWorktreeBounded(t *testing.T) {
	ctx := context.Background()

	// (a) real work tree -> (true, nil).
	wsRepo := newGateTestWorkspace(t)
	initGitRepoWithCommits(t, wsRepo.RootPath)
	inRepo, err := wsRepo.inGitWorktreeBounded(ctx)
	require.NoError(t, err, "a real work tree must probe cleanly")
	assert.True(t, inRepo, "a real work tree must report inside-work-tree=true")

	// (b) no-repo temp dir -> (false, nil): legacy skip preserved.
	wsNoRepo := newGateTestWorkspace(t)
	inRepo, err = wsNoRepo.inGitWorktreeBounded(ctx)
	require.NoError(t, err, "a genuine no-repo dir must be a silent skip, not an error")
	assert.False(t, inRepo, "a no-repo dir must report inside-work-tree=false")

	// (c) already-expired bounded context -> fail closed with a non-nil ctx error.
	expired, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Hour))
	defer cancel()
	inRepo, err = wsRepo.inGitWorktreeBounded(expired)
	require.Error(t, err, "an expired bounded probe must fail closed (non-nil error)")
	assert.False(t, inRepo, "a failed probe must not report inside-work-tree=true")

	// (d) present-but-broken .git pointer (a gitfile referencing a MISSING gitdir):
	// git emits `fatal: not a git repository: (NULL)` on exit 128 — NOT the
	// genuine-no-repo "(or any of the parent directories)" marker — so it is a
	// present-but-broken repo that MUST FAIL CLOSED (non-nil err), never be misread
	// as a no-repo skip. This pins the F1 fail-open hole closed: a loose
	// "not a git repository" substring match wrongly skipped this case, letting an
	// unprovable-lineage shipment ship.
	if _, gerr := exec.LookPath("git"); gerr == nil {
		wsBroken := newGateTestWorkspace(t)
		gitfile := filepath.Join(wsBroken.RootPath, ".git")
		missingGitdir := filepath.Join(wsBroken.RootPath, "nonexistent-gitdir")
		require.NoError(t, os.WriteFile(gitfile, []byte("gitdir: "+missingGitdir), 0o644))
		inRepo, err = wsBroken.inGitWorktreeBounded(ctx)
		require.Error(t, err, "a present-but-broken .git pointer must fail closed, not be misread as a no-repo skip")
		assert.False(t, inRepo, "a broken-repo probe must not report inside-work-tree=true")
	}
}

// TestIsAncestor exercises the git ancestor-lineage helper against a real repo:
// an ancestor (and an equal head) are included (exit 0), a real divergent sibling
// is a clean not-an-ancestor (exit 1 -> (false, nil)), and a valid-shape but
// absent object fails closed (error, not a silent pass).
func TestIsAncestor(t *testing.T) {
	ws := newGateTestWorkspace(t)
	base, head, divergent := initGitRepoWithCommits(t, ws.RootPath)
	ctx := context.Background()

	ok, err := ws.isAncestor(ctx, base, head)
	require.NoError(t, err)
	assert.True(t, ok, "base must be an ancestor of head")

	// A commit is an ancestor of itself (exit 0).
	ok, err = ws.isAncestor(ctx, head, head)
	require.NoError(t, err)
	assert.True(t, ok, "a commit is an ancestor of itself")

	// A real sibling commit is a clean not-an-ancestor (exit 1), NOT an error.
	ok, err = ws.isAncestor(ctx, divergent, head)
	require.NoError(t, err, "a real non-ancestor is exit 1, not an error")
	assert.False(t, ok, "a divergent sibling is not an ancestor")

	// A valid-shape but absent object must fail CLOSED (error), never pass.
	absent := "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	_, err = ws.isAncestor(ctx, absent, head)
	require.Error(t, err, "an absent object must fail closed, not be read as an ancestor")
}

// TestIsGitObjectName pins the SHA-shape guard: only full-length SHA-1 (40 hex)
// and SHA-256 (64 hex) object names are accepted; abbreviations, empty, non-hex,
// over-length, and argument-injection (leading dash) values are refused.
func TestIsGitObjectName(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"sha1-lower", strings.Repeat("a", 40), true},
		{"sha1-zeros", strings.Repeat("0", 40), true},
		{"sha1-upper", strings.Repeat("A", 40), true},
		{"sha256", strings.Repeat("f", 64), true},
		{"empty", "", false},
		{"legacy-fake", "oldsha0000", false},
		{"not-a-sha", "not-a-sha", false},
		{"leading-dash", "-foo", false},
		{"abbrev-7", strings.Repeat("a", 7), false},
		{"too-long-65", strings.Repeat("a", 65), false},
		{"non-hex", strings.Repeat("g", 40), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equalf(t, tc.want, isGitObjectName(tc.in), "isGitObjectName(%q)", tc.in)
		})
	}
}
