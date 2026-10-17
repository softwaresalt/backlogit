package policyengine

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestU163_002_NewRecordBindsAndDigestsEvidence(t *testing.T) {
	at := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	r, err := NewRecord("163.002-T", NodeHarnessScaffold, sha(1), at, OutcomePass, nil)
	require.NoError(t, err)
	assert.Equal(t, EvidenceSchemaVersion, r.SchemaVersion)
	assert.Len(t, r.Digest, 64)

	again, err := NewRecord("163.002-T", NodeHarnessScaffold, sha(1), at.In(time.FixedZone("x", 3600)), OutcomePass, nil)
	require.NoError(t, err)
	assert.Equal(t, r.Digest, again.Digest, "the digest must not depend on the time zone")

	for name, mut := range map[string]func() (Record, error){
		"task": func() (Record, error) {
			return NewRecord("163.003-T", NodeHarnessScaffold, sha(1), at, OutcomePass, nil)
		},
		"node": func() (Record, error) { return NewRecord("163.002-T", NodeCIPass, sha(1), at, OutcomePass, nil) },
		"commit": func() (Record, error) {
			return NewRecord("163.002-T", NodeHarnessScaffold, sha(2), at, OutcomePass, nil)
		},
		"time": func() (Record, error) {
			return NewRecord("163.002-T", NodeHarnessScaffold, sha(1), at.Add(time.Second), OutcomePass, nil)
		},
		"outcome": func() (Record, error) {
			return NewRecord("163.002-T", NodeHarnessScaffold, sha(1), at, OutcomeFail, nil)
		},
		"artifact": func() (Record, error) {
			return NewRecord("163.002-T", NodeHarnessScaffold, sha(1), at, OutcomePass, json.RawMessage(`{"a":1}`))
		},
	} {
		other, err := mut()
		require.NoError(t, err, name)
		assert.NotEqual(t, r.Digest, other.Digest, "digest must cover %s", name)
	}
}

func TestU163_002_DigestIgnoresArtifactWhitespaceOnly(t *testing.T) {
	at := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	a, err := NewRecord("163.002-T", NodeRedPhaseVerify, sha(1), at, OutcomePass, json.RawMessage(`{"a":1,"b":[1,2]}`))
	require.NoError(t, err)
	b, err := NewRecord("163.002-T", NodeRedPhaseVerify, sha(1), at, OutcomePass, json.RawMessage("{ \"a\": 1,\n \"b\": [1, 2] }"))
	require.NoError(t, err)
	assert.Equal(t, a.Digest, b.Digest)
}

func TestU163_002_NewRecordRejectsInvalidInput(t *testing.T) {
	at := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	big := json.RawMessage(`"` + strings.Repeat("a", MaxArtifactBytes) + `"`)
	cases := map[string]func() (Record, error){
		"bad task":     func() (Record, error) { return NewRecord("x", NodeCIPass, sha(1), at, OutcomePass, nil) },
		"unknown node": func() (Record, error) { return NewRecord("163.002-T", "deploy", sha(1), at, OutcomePass, nil) },
		"short sha":    func() (Record, error) { return NewRecord("163.002-T", NodeCIPass, "abc", at, OutcomePass, nil) },
		"uppercase sha": func() (Record, error) {
			return NewRecord("163.002-T", NodeCIPass, strings.ToUpper(sha(255)), at, OutcomePass, nil)
		},
		"zero time": func() (Record, error) {
			return NewRecord("163.002-T", NodeCIPass, sha(1), time.Time{}, OutcomePass, nil)
		},
		"bad outcome": func() (Record, error) { return NewRecord("163.002-T", NodeCIPass, sha(1), at, "ok", nil) },
		"invalid artifact": func() (Record, error) {
			return NewRecord("163.002-T", NodeCIPass, sha(1), at, OutcomePass, json.RawMessage(`{`))
		},
		"oversized": func() (Record, error) { return NewRecord("163.002-T", NodeCIPass, sha(1), at, OutcomePass, big) },
	}
	for name, f := range cases {
		_, err := f()
		require.Error(t, err, name)
		assert.ErrorIs(t, err, ErrInvalidRecord, name)
	}
}

func TestU163_002_DecodeBundleIsStrictAndBounded(t *testing.T) {
	e := newEnv(t, enforceRegistry)
	req := e.request(e.good())
	b := Bundle{SchemaVersion: EvidenceSchemaVersion, TaskID: req.TaskID, HeadCommit: req.HeadCommit, PR: 7, Records: req.Records}
	raw, err := json.Marshal(b)
	require.NoError(t, err)

	got, err := DecodeBundle(raw)
	require.NoError(t, err)
	r2 := got.Request(SurfaceMCP)
	assert.Equal(t, SurfaceMCP, r2.Surface)
	assert.Equal(t, req.TaskID, r2.TaskID)
	assert.Equal(t, req.HeadCommit, r2.HeadCommit)
	assert.Equal(t, 7, r2.PR)
	require.Len(t, r2.Records, len(req.Records))
	assert.False(t, r2.Interactive, "interactivity is never carried by evidence")
	assert.Empty(t, r2.OverrideTokens)

	rep := func(old, new string) []byte { return []byte(strings.Replace(string(raw), old, new, 1)) }
	cases := map[string][]byte{
		"empty":               nil,
		"yaml":                []byte("schema_version: 1\n"),
		"self-asserted owner": rep(`"records"`, `"principal":"operator","records"`),
		"operator identity":   rep(`"records"`, `"operator":"alice","records"`),
		"interactive flag":    rep(`"records"`, `"interactive":true,"records"`),
		"duplicate key":       rep(`"pr":7`, `"pr":7,"pr":8`),
		"fold duplicate":      rep(`"pr":7`, `"pr":7,"PR":8`),
		"trailing data":       append(append([]byte{}, raw...), []byte(`{}`)...),
		"wrong version":       rep(`"schema_version":1`, `"schema_version":9`),
		"oversized":           []byte(`{"schema_version":1,"records":[],"task_id":"` + strings.Repeat("a", MaxBundleBytes) + `"}`),
		"too many records": []byte(`{"schema_version":1,"task_id":"163.002-T","head_commit":"` + sha(1) + `","records":[` +
			strings.TrimSuffix(strings.Repeat("{},", MaxRecords+1), ",") + `]}`),
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := DecodeBundle(data)
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrInvalidRecord)
		})
	}
}

func gitRun(t *testing.T, dir string, env []string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), append([]string{
		"GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_CONFIG_SYSTEM=" + os.DevNull,
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com",
	}, env...)...)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
	return strings.TrimSpace(string(out))
}

// realRepo builds n chained empty commits dated t0+i minutes and returns the
// repository directory and SHAs.
func realRepo(t *testing.T, n int, t0 time.Time) (string, []string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not available")
	}
	dir := t.TempDir()
	gitRun(t, dir, nil, "init", "-q", "-b", "main")
	var shas []string
	for i := 0; i < n; i++ {
		d := t0.Add(time.Duration(i) * time.Minute).Format(time.RFC3339)
		gitRun(t, dir, []string{"GIT_AUTHOR_DATE=" + d, "GIT_COMMITTER_DATE=" + d}, "commit", "-q", "--allow-empty", "-m", "c")
		shas = append(shas, gitRun(t, dir, nil, "rev-parse", "HEAD"))
	}
	return dir, shas
}

func TestU163_002_ExecGitResolvesAncestryAndTime(t *testing.T) {
	t0 := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	dir, c := realRepo(t, 3, t0)
	gitRun(t, dir, nil, "update-ref", "refs/remotes/origin/main", c[2])
	g := ExecGit{Dir: dir}
	ctx := context.Background()

	got, err := g.ResolveCommit(ctx, "refs/remotes/origin/main")
	require.NoError(t, err)
	assert.Equal(t, c[2], got)

	_, err = g.ResolveCommit(ctx, "refs/remotes/origin/missing")
	assert.ErrorIs(t, err, ErrRefNotFound)
	for _, bad := range []string{"", "--help", "-x", "refs/remotes/origin/a b", "refs/remotes/..", "HEAD~1", "refs/remotes/o/\x00"} {
		_, err = g.ResolveCommit(ctx, bad)
		assert.Error(t, err, bad)
	}

	ok, err := g.IsAncestor(ctx, c[0], c[2])
	require.NoError(t, err)
	assert.True(t, ok)
	ok, err = g.IsAncestor(ctx, c[2], c[0])
	require.NoError(t, err)
	assert.False(t, ok)
	ok, err = g.IsAncestor(ctx, c[1], c[1])
	require.NoError(t, err)
	assert.True(t, ok)
	_, err = g.IsAncestor(ctx, sha(999), c[2])
	assert.Error(t, err, "an unknown commit is an error, never a plain false")
	_, err = g.IsAncestor(ctx, "--all", c[2])
	assert.Error(t, err)

	at, err := g.CommitTime(ctx, c[1])
	require.NoError(t, err)
	assert.True(t, at.Equal(t0.Add(time.Minute)), at.String())
	_, err = g.CommitTime(ctx, sha(999))
	assert.Error(t, err)
	_, err = g.CommitTime(ctx, "-p")
	assert.Error(t, err)
}

func TestU163_002_ExecGitIgnoresAmbientGitEnvironment(t *testing.T) {
	t0 := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	dir, c := realRepo(t, 2, t0)
	other, _ := realRepo(t, 1, t0)
	t.Setenv("GIT_DIR", other+string(os.PathSeparator)+".git")
	ok, err := ExecGit{Dir: dir}.IsAncestor(context.Background(), c[0], c[1])
	require.NoError(t, err, "an ambient GIT_DIR must not redirect the repository")
	assert.True(t, ok)
}

func TestU163_002_ExecGitRequiresARepositoryDirectory(t *testing.T) {
	_, err := ExecGit{}.ResolveCommit(context.Background(), "refs/remotes/origin/main")
	assert.Error(t, err)
	_, err = ExecGit{Dir: t.TempDir()}.ResolveCommit(context.Background(), "refs/remotes/origin/main")
	assert.Error(t, err)
}

func TestU163_002_EngineEndToEndOnARealRepository(t *testing.T) {
	t0 := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	dir, c := realRepo(t, 5, t0)
	gitRun(t, dir, nil, "update-ref", "refs/remotes/origin/main", c[0])
	gitRun(t, dir, nil, "update-ref", "refs/remotes/origin/feat/x", c[4])

	e := newEnv(t, enforceRegistry)
	copy(e.c[:], c)
	e.t0 = t0
	e.deriv.review.HeadSHA = c[4]
	e.deriv.ci.HeadSHA = c[4]
	eng := e.engine(func(o *Options) { o.Git = ExecGit{Dir: dir}; o.Now = func() time.Time { return t0.Add(time.Hour) } })
	d := check(t, eng, e.request(e.good()))
	assert.Equal(t, DispositionPass, d.Disposition, "%+v", d.Errors)

	// A fabricated record whose commit does not exist in this repository blocks.
	m := e.good()
	m[NodeGreenPhaseVerify] = e.rec(NodeGreenPhaseVerify, sha(4242), e.at(3, 30), OutcomePass, nil)
	d = check(t, eng, e.request(m))
	assert.False(t, d.Allowed)

	// The head published only on a local branch is not trusted.
	gitRun(t, dir, nil, "update-ref", "-d", "refs/remotes/origin/feat/x")
	gitRun(t, dir, nil, "update-ref", "refs/heads/feat/x", c[4])
	d = check(t, eng, e.request(e.good()))
	assert.False(t, d.Allowed)
	assert.True(t, hasViolation(d, "", CodeHeadUnpublished), "%+v", d.Errors)
}
