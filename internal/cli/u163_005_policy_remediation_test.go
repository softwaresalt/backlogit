package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/policyengine"
)

func policyCheckArgs(extra ...string) []string {
	return append([]string{"policy", "check", "--registry", "registry.yaml", "--evidence", "bundle.json",
		"--protected-ref", "refs/remotes/origin/feat/x", "--format", "json"}, extra...)
}

func setPolicyRepo(t *testing.T, fn func(dir string) (string, string, error)) {
	t.Helper()
	prev := policyRepo
	policyRepo = fn
	t.Cleanup(func() { policyRepo = prev })
}

func TestU163_005_Remediation6RepoMustMatchTheOriginRemote(t *testing.T) {
	f := newPolicyFixture(t, policyTestRegistry)
	f.writeBundle("bundle.json", nil)
	installPolicySeams(t, f.mem, false)

	for name, origin := range map[string]string{
		"https":      "https://github.com/o/r.git",
		"https auth": "https://user@github.com/o/r",
		"scp":        "git@github.com:o/r.git",
		"ssh":        "ssh://git@github.com/o/r.git",
	} {
		t.Run("match "+name, func(t *testing.T) {
			setPolicyRepo(t, func(dir string) (string, string, error) { return dir, origin, nil })
			out, err := runPolicy(t, f.root, policyCheckArgs("--repo", "O/R")...)
			require.NoError(t, err, out)
		})
	}
	t.Run("mismatch is refused", func(t *testing.T) {
		setPolicyRepo(t, func(dir string) (string, string, error) { return dir, "https://github.com/o/r.git", nil })
		_, err := runPolicy(t, f.root, policyCheckArgs("--repo", "evil/other")...)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "does not match")
	})
	t.Run("unresolvable origin fails closed in enforce mode", func(t *testing.T) {
		setPolicyRepo(t, func(dir string) (string, string, error) { return dir, "", errors.New("no origin") })
		_, err := runPolicy(t, f.root, policyCheckArgs("--repo", "o/r")...)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "origin")
	})
	t.Run("non-github origin fails closed in enforce mode", func(t *testing.T) {
		setPolicyRepo(t, func(dir string) (string, string, error) { return dir, "https://example.com/o/r.git", nil })
		_, err := runPolicy(t, f.root, policyCheckArgs("--repo", "o/r")...)
		require.Error(t, err)
	})
	t.Run("unresolvable origin is tolerated in report-only mode", func(t *testing.T) {
		g := newPolicyFixture(t, strings.Replace(policyTestRegistry, "mode: enforce", "mode: report-only", 1))
		g.writeBundle("bundle.json", nil)
		installPolicySeams(t, g.mem, false)
		setPolicyRepo(t, func(dir string) (string, string, error) { return dir, "", errors.New("no origin") })
		_, err := runPolicy(t, g.root, policyCheckArgs("--repo", "o/r")...)
		require.NoError(t, err)
	})
}

func TestU163_005_Remediation6GitToplevelIsTheRootNotTheCwd(t *testing.T) {
	f := newPolicyFixture(t, policyTestRegistry)
	f.writeBundle("bundle.json", nil)
	installPolicySeams(t, f.mem, false)
	sub := filepath.Join(f.root, "sub", "dir")
	require.NoError(t, os.MkdirAll(sub, 0o755))
	setPolicyRepo(t, func(string) (string, string, error) { return f.root, "https://github.com/o/r.git", nil })

	out, err := runPolicy(t, sub, policyCheckArgs("--repo", "o/r")...)
	require.NoError(t, err, "registry and evidence resolve against the toplevel, not the cwd: %s", out)

	// A trust/ledger/anchor path inside the toplevel but outside the cwd is still
	// agent-writable and must be refused.
	_, err = runPolicy(t, sub, policyCheckArgs("--repo", "o/r",
		"--trust", filepath.Join(f.root, "trust.json"),
		"--ledger", filepath.Join(f.root, "ledger.jsonl"),
		"--anchor", filepath.Join(f.root, "anchor.json"))...)
	require.Error(t, err)

	_, err = runPolicy(t, sub, "policy", "verify-overrides", "--genesis",
		"--ledger", filepath.Join(f.root, "ledger.jsonl"), "--anchor", filepath.Join(f.root, "anchor.json"))
	require.Error(t, err)
}

func TestU163_005_Remediation6NotAGitRepositoryFailsClosed(t *testing.T) {
	f := newPolicyFixture(t, policyTestRegistry)
	f.writeBundle("bundle.json", nil)
	installPolicySeams(t, f.mem, false)
	setPolicyRepo(t, func(string) (string, string, error) { return "", "", errors.New("not a git repository") })
	_, err := runPolicy(t, f.root, policyCheckArgs("--repo", "o/r")...)
	require.Error(t, err)
	_, err = runPolicy(t, f.root, "policy", "verify-overrides", "--genesis",
		"--ledger", filepath.Join(t.TempDir(), "l.jsonl"), "--anchor", filepath.Join(t.TempDir(), "a.json"))
	require.Error(t, err)
}

func TestU163_005_Remediation6CheckHonoursTheCommandContext(t *testing.T) {
	f := newPolicyFixture(t, policyTestRegistry)
	f.writeBundle("bundle.json", nil)
	installPolicySeams(t, f.mem, false)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(append([]string{"--cwd", f.root, "--no-update-check"}, policyCheckArgs("--repo", "o/r")...))
	err := cmd.ExecuteContext(ctx)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestU163_005_Remediation6WaiverFlagErrorsAreDeterministic(t *testing.T) {
	f := newPolicyFixture(t, policyTestRegistry)
	f.writeBundle("bundle.json", nil)
	installPolicySeams(t, f.mem, false)
	for i := 0; i < 20; i++ {
		_, err := runPolicy(t, f.root, policyCheckArgs("--repo", "o/r", "--trust", "t.json", "--ledger", "l.jsonl", "--anchor", "a.json")...)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--trust", "the first offending flag in declaration order is always reported")
	}
}

func TestU163_005_Remediation7VerifyOverridesFailsClosedWithoutAnAnchor(t *testing.T) {
	f := newPolicyFixture(t, policyTestRegistry)
	installPolicySeams(t, f.mem, false)
	lpath := filepath.Join(t.TempDir(), "overrides.jsonl")
	apath := filepath.Join(t.TempDir(), "anchor.json")

	_, err := runPolicy(t, f.root, "policy", "verify-overrides", "--ledger", lpath, "--anchor", apath, "--format", "json")
	require.Error(t, err, "an absent anchor is not a pass")
	assert.Contains(t, err.Error(), "anchor")

	out, err := runPolicy(t, f.root, "policy", "verify-overrides", "--genesis", "--ledger", lpath, "--anchor", apath, "--format", "json")
	require.NoError(t, err, out)
	var log policyengine.OverrideLog
	require.NoError(t, json.Unmarshal([]byte(out), &log), out)
	assert.Zero(t, log.Records)
}

func TestU163_005_Remediation7LedgerAndAnchorMustLiveInDifferentDirectories(t *testing.T) {
	f := newPolicyFixture(t, policyTestRegistry)
	f.writeBundle("bundle.json", nil)
	installPolicySeams(t, f.mem, false)
	dir := t.TempDir()

	_, err := runPolicy(t, f.root, "policy", "verify-overrides", "--genesis",
		"--ledger", filepath.Join(dir, "overrides.jsonl"), "--anchor", filepath.Join(dir, "anchor.json"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "different director")

	_, err = runPolicy(t, f.root, policyCheckArgs("--repo", "o/r",
		"--trust", filepath.Join(t.TempDir(), "trust.json"),
		"--ledger", filepath.Join(dir, "overrides.jsonl"), "--anchor", filepath.Join(dir, "anchor.json"))...)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "different director")
}

func TestU163_005_Remediation7VerifyOverridesHelpStatesItNeedsWriteAccess(t *testing.T) {
	f := newPolicyFixture(t, policyTestRegistry)
	installPolicySeams(t, f.mem, false)
	out, err := runPolicy(t, f.root, "policy", "verify-overrides", "--help")
	require.NoError(t, err)
	assert.Contains(t, out, "write access")
	assert.Contains(t, out, "--genesis")
}

func TestU163_005_Remediation8PolicyFilesRefuseNonRegularFiles(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, "dir"), 0o755))
	_, err := readPolicyFile(root, "evidence", "dir", 1024)
	require.Error(t, err)
	_, err = readPolicyFile(root, "evidence", "../x", 1024)
	require.Error(t, err)
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".github", "p"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".github", "p", "r.yaml"), []byte("ok"), 0o600))
	got, err := readPolicyFile(root, "registry", ".github/p/r.yaml", 1024)
	require.NoError(t, err)
	assert.Equal(t, "ok", string(got))
}
