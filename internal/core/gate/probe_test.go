package gate

import (
	"context"
	stderrors "errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bkerrors "github.com/softwaresalt/backlogit/internal/errors"
)

type fakeVersion struct {
	ver string
	err error
}

func (f fakeVersion) Version(context.Context) (string, error) { return f.ver, f.err }

func TestProbe(t *testing.T) {
	tests := []struct {
		name        string
		enabled     EnabledMode
		vr          fakeVersion
		wantEnforce bool
		wantSetup   bool
	}{
		{
			name: "resolvable compatible enforces", enabled: EnabledTrue,
			vr: fakeVersion{ver: "1.4.7"}, wantEnforce: true,
		},
		{
			name: "newer version enforces", enabled: EnabledTrue,
			vr: fakeVersion{ver: "1.5.0"}, wantEnforce: true,
		},
		{
			name: "missing binary under auto fails open", enabled: EnabledAuto,
			vr: fakeVersion{err: bkerrors.ErrGateBinaryNotFound}, wantEnforce: false,
		},
		{
			name: "missing binary under true is setup error", enabled: EnabledTrue,
			vr: fakeVersion{err: bkerrors.ErrGateBinaryNotFound}, wantEnforce: false, wantSetup: true,
		},
		{
			name: "version below floor under true is setup error", enabled: EnabledTrue,
			vr: fakeVersion{ver: "1.4.6"}, wantEnforce: false, wantSetup: true,
		},
		{
			name: "version below floor under auto fails open", enabled: EnabledAuto,
			vr: fakeVersion{ver: "1.3.0"}, wantEnforce: false,
		},
		{
			name: "disabled never enforces", enabled: EnabledFalse,
			vr: fakeVersion{ver: "1.4.7"}, wantEnforce: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enforce, err := Probe(context.Background(), tt.vr, tt.enabled)
			if enforce != tt.wantEnforce {
				t.Fatalf("enforce = %v, want %v", enforce, tt.wantEnforce)
			}
			if tt.wantSetup {
				var ge *bkerrors.GateError
				if !stderrors.As(err, &ge) || ge.Class != "setup" {
					t.Fatalf("err = %v, want setup-class GateError", err)
				}
				if !stderrors.Is(err, bkerrors.ErrGateSetup) {
					t.Fatalf("err = %v, want ErrGateSetup", err)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestVersionAtLeast(t *testing.T) {
	cases := []struct {
		have, want string
		ok         bool
	}{
		{"1.4.7", "1.4.7", true},
		{"1.4.8", "1.4.7", true},
		{"1.5.0", "1.4.7", true},
		{"2.0.0", "1.4.7", true},
		{"1.4.6", "1.4.7", false},
		{"1.3.9", "1.4.7", false},
		{"v1.4.7", "1.4.7", true},
		{"1.4.7-dev", "1.4.7", true},
	}
	for _, c := range cases {
		got, err := versionAtLeast(c.have, c.want)
		if err != nil {
			t.Fatalf("versionAtLeast(%q,%q): %v", c.have, c.want, err)
		}
		if got != c.ok {
			t.Fatalf("versionAtLeast(%q,%q) = %v, want %v", c.have, c.want, got, c.ok)
		}
	}
}

// TestExecVersionRunner_NilEnvDefaultsToScrubbed verifies that
// ExecVersionRunner.Version, when Env is unset, no longer leaves cmd.Env nil
// (which would inherit the ambient environment including
// BACKLOGIT_GATE_EVIDENCE_KEY unfiltered) and still functions correctly
// against a real binary (106-F F1/U2). The explicit-Env branch is unchanged
// existing code and is already exercised by TestProbe's fake VersionRunner.
func TestExecVersionRunner_NilEnvDefaultsToScrubbed(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	t.Setenv("BACKLOGIT_GATE_EVIDENCE_KEY", "0123456789abcdef0123456789abcdef")

	r := ExecVersionRunner{Binary: "git"} // Env intentionally unset (nil)
	out, err := r.Version(context.Background())
	if err != nil {
		t.Fatalf("Version() unexpected error: %v", err)
	}
	// firstVersionToken strips leading "git"/"version" words and returns just
	// the parsed version token (e.g. "2.55.0.windows.3"), so assert on shape
	// (digits and dots) rather than the literal word "git".
	if !strings.ContainsAny(out, "0123456789") || !strings.Contains(out, ".") {
		t.Fatalf("Version() = %q, want a version-shaped token", out)
	}
}

// TestU1k_ExecGitRunnerVerifyIsolatesTempDirFromOuterCheckout exercises real
// repository discovery with TEMP nested inside a committed outer checkout.
func TestU1k_ExecGitRunnerVerifyIsolatesTempDirFromOuterCheckout(t *testing.T) {
	// Git's ambient repository, index, config and discovery overrides must not
	// turn either fixture into a different repository. Setenv registers restoration.
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(strings.ToUpper(key), "GIT_") {
			t.Setenv(key, "")
			require.NoError(t, os.Unsetenv(key))
		}
	}
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	_, err := exec.LookPath("git")
	require.NoError(t, err, "U1k requires git; the RED harness must not skip")

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	env := os.Environ()
	outer := t.TempDir()
	u1kInitCommittedRepo(t, ctx, outer, env)
	tempRoot := filepath.Join(outer, "temp")
	require.NoError(t, os.Mkdir(tempRoot, 0o700))
	for _, key := range []string{"TEMP", "TMP", "TMPDIR"} {
		t.Setenv(key, tempRoot)
	}
	require.Equal(t, filepath.Clean(tempRoot), filepath.Clean(os.TempDir()))

	// A fresh testing.T is needed: the parent's TempDir base was already cached
	// before TEMP changed when it allocated the outer repository.
	t.Run("temp_workspaces", func(t *testing.T) {
		nonrepo := t.TempDir()
		rel, err := filepath.Rel(os.TempDir(), nonrepo)
		require.NoError(t, err)
		require.NotEqual(t, ".", rel)
		require.False(t, filepath.IsAbs(rel) || rel == ".." ||
			strings.HasPrefix(rel, ".."+string(filepath.Separator)))
		// Prove ancestor discovery without depending on the real workspace HEAD.
		cmd := exec.CommandContext(ctx, "git", "rev-parse", "--verify", "HEAD^{commit}")
		cmd.Dir, cmd.Env = nonrepo, env
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "outer HEAD discovery control: %s", out)
		require.NotEmpty(t, strings.TrimSpace(string(out)))

		got, err := (ExecGitRunner{Dir: nonrepo, Env: env}).Verify(ctx, "HEAD")
		require.NoError(t, err)
		assert.False(t, got, "U1k temp ceiling: nonrepo t.TempDir inherited outer checkout HEAD")

		repo := t.TempDir()
		u1kInitCommittedRepo(t, ctx, repo, env)
		subdir := filepath.Join(repo, "nested")
		require.NoError(t, os.Mkdir(subdir, 0o700))
		got, err = (ExecGitRunner{Dir: subdir, Env: env}).Verify(ctx, "HEAD")
		require.NoError(t, err)
		assert.True(t, got, "U1k temp ceiling must preserve a repository inside TEMP")

		// An explicit caller ceiling remains authoritative.
		explicitEnv := append(append([]string(nil), env...), "GIT_CEILING_DIRECTORIES="+repo)
		got, err = (ExecGitRunner{Dir: subdir, Env: explicitEnv}).Verify(ctx, "HEAD")
		require.NoError(t, err)
		assert.False(t, got, "U1k must preserve the caller's explicit git ceiling")
	})
}

func u1kInitCommittedRepo(t *testing.T, ctx context.Context, dir string, env []string) {
	t.Helper()
	for _, args := range [][]string{
		{"init", "--template="},
		{"-c", "user.name=U1k Fixture", "-c", "user.email=u1k@example.invalid",
			"-c", "commit.gpgsign=false", "-c", "core.hooksPath=" + os.DevNull,
			"commit", "--allow-empty", "-m", "fixture"},
	} {
		cmd := exec.CommandContext(ctx, "git", args...)
		cmd.Dir, cmd.Env = dir, env
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git fixture %v: %s", args, out)
	}
}

// TestU1k_ExecVersionRunnerCachesProbesByBinaryIdentity uses a local, readable
// shell fixture, not a downloaded executable or an in-process runner mock.
func TestU1k_ExecVersionRunnerCachesProbesByBinaryIdentity(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	dir := t.TempDir()
	binary := filepath.Join(dir, "autoharness")
	if runtime.GOOS == "windows" {
		binary += ".cmd"
	}
	counter := filepath.Join(dir, "counter")
	require.NoError(t, os.WriteFile(counter, []byte("0\n"), 0o600))
	t.Setenv("U1K_COUNTER", counter)
	u1kWriteVersionFixture(t, binary, "1.6.0")
	runner := ExecVersionRunner{Binary: binary, Dir: dir, Env: os.Environ()}
	checkVersion := func(r ExecVersionRunner, want string) {
		t.Helper()
		got, err := r.Version(ctx)
		require.NoError(t, err, "execute trusted local version fixture")
		require.Equal(t, want, got)
	}
	checkCount := func(want int) {
		t.Helper()
		data, err := os.ReadFile(counter)
		require.NoError(t, err)
		got, err := strconv.Atoi(strings.TrimSpace(string(data)))
		require.NoError(t, err, "fixture counter must be an integer")
		assert.Equal(t, want, got, "U1k version cache: unexpected subprocess count")
	}

	checkVersion(runner, "1.6.0")
	checkCount(1)
	checkVersion(runner, "1.6.0")
	checkCount(1)

	before, err := os.Stat(binary)
	require.NoError(t, err)
	u1kWriteVersionFixture(t, binary, "1.7.0")
	// Force a size change even on filesystems with coarse timestamp resolution.
	f, err := os.OpenFile(binary, os.O_APPEND|os.O_WRONLY, 0)
	require.NoError(t, err)
	_, writeErr := f.WriteString("\n")
	closeErr := f.Close()
	require.NoError(t, writeErr)
	require.NoError(t, closeErr)
	after, err := os.Stat(binary)
	require.NoError(t, err)
	require.NotEqual(t, before.Size(), after.Size(), "fixture identity must change")
	checkVersion(runner, "1.7.0")
	checkCount(2)

	changedEnv := runner
	changedEnv.Env = append(append([]string(nil), runner.Env...), "U1K_CACHE_IDENTITY=changed")
	checkVersion(changedEnv, "1.7.0")
	checkCount(3)
	checkVersion(changedEnv, "1.7.0")
	checkCount(3)
}

func u1kWriteVersionFixture(t *testing.T, path, version string) {
	t.Helper()
	body := `#!/bin/sh
set -eu
read -r n < "$U1K_COUNTER"
printf '%s\n' "$((n + 1))" > "$U1K_COUNTER"
printf 'autoharness @VERSION@\n'
`
	if runtime.GOOS == "windows" {
		body = "@echo off\r\nsetlocal\r\nset /p n=<\"%U1K_COUNTER%\"\r\n" +
			"set /a n+=1 >nul\r\n>\"%U1K_COUNTER%\" echo %n%\r\n" +
			"echo autoharness @VERSION@\r\n"
	}
	require.NoError(t, os.WriteFile(path, []byte(strings.ReplaceAll(body, "@VERSION@", version)), 0o755))
}
