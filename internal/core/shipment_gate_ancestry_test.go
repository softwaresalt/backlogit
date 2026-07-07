package core

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

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
