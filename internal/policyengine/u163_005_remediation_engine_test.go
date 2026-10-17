package policyengine

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/evidence"
	"github.com/softwaresalt/backlogit/internal/evidencedag"
)

func runGitFixture(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false"}, args...)...)
	cmd.Dir = dir
	var env []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(strings.ToUpper(kv), "GIT_") {
			env = append(env, kv)
		}
	}
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, out)
	return strings.TrimSpace(string(out))
}

func TestU163_005_Remediation1ReplaceRefsCannotForgeAncestry(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	dir := t.TempDir()
	runGitFixture(t, dir, "init", "-q", "-b", "main")
	runGitFixture(t, dir, "commit", "-q", "--allow-empty", "-m", "published tip")
	tip := runGitFixture(t, dir, "rev-parse", "HEAD")
	runGitFixture(t, dir, "checkout", "-q", "--orphan", "rogue")
	runGitFixture(t, dir, "commit", "-q", "--allow-empty", "-m", "unpublished head")
	head := runGitFixture(t, dir, "rev-parse", "HEAD")
	runGitFixture(t, dir, "update-ref", "refs/remotes/origin/main", tip)
	// Forge parentage: make the unpublished head look like an ancestor of the tip.
	runGitFixture(t, dir, "replace", "--graft", tip, head)

	g := ExecGit{Dir: dir}
	ok, err := g.IsAncestor(context.Background(), head, tip)
	require.NoError(t, err)
	assert.False(t, ok, "a replace ref must not forge ancestry")

	reg, err := LoadRegistry([]byte(enforceRegistry))
	require.NoError(t, err)
	eng, err := NewEngine(reg, Options{Git: g, ProtectedRefs: []string{"refs/remotes/origin/main"}})
	require.NoError(t, err)
	e := newEnv(t, enforceRegistry)
	req := e.request(e.good())
	req.HeadCommit = head
	d, err := eng.Check(context.Background(), req)
	require.NoError(t, err)
	assert.False(t, d.Allowed)
	assert.True(t, hasViolation(d, "", CodeHeadUnpublished), "%+v", d.Errors)
}

func TestU163_005_Remediation2OverridableIsLimitedToHarnessAndRedGates(t *testing.T) {
	for _, node := range []string{"green-phase-verify", "review-complete", "ci-pass", "merge-ready"} {
		t.Run(node, func(t *testing.T) {
			doc := strings.Replace(canonicalRegistryYAML,
				"{id: "+node+", enabled: true, overridable: false}",
				"{id: "+node+", enabled: true, overridable: true}", 1)
			require.NotEqual(t, canonicalRegistryYAML, doc)
			_, err := LoadRegistry([]byte(doc))
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrInvalidRegistry)
			assert.Contains(t, err.Error(), node)
			assert.Contains(t, err.Error(), "overridable")
		})
	}
	_, err := LoadRegistry([]byte(canonicalRegistryYAML))
	require.NoError(t, err, "harness-scaffold and red-phase-verify stay overridable")
}

func TestU163_005_Remediation3NotReadyWithoutErrorsStillReportsAViolation(t *testing.T) {
	e := newEnv(t, enforceRegistry)
	eng := e.engine()
	req := e.request(e.good())
	dec := evidencedag.Decision{ReviewReady: false, Allowed: false, Reason: "the core says no terminal node is applicable"}
	out := eng.decide(req, nil, map[NodeType]fact{}, dec, overridePlan{})
	assert.False(t, out.Allowed)
	assert.Equal(t, DispositionBlock, out.Disposition)
	require.NotEmpty(t, out.Errors, "a block must carry at least one violation")
	assert.Contains(t, out.Errors[0].Reason, "no terminal node is applicable")
	assert.NotContains(t, out.Reason, "block 0 gate")
	assert.NotContains(t, out.Reason, "0 gate violation")
}

type cancellingGit struct {
	Git
	cancel context.CancelFunc
}

func (c *cancellingGit) ResolveCommit(ctx context.Context, ref string) (string, error) {
	c.cancel()
	return "", ctx.Err()
}

func TestU163_005_Remediation4CancellationIsNeverAllowedEvenReportOnly(t *testing.T) {
	for name, registry := range map[string]string{"enforce": enforceRegistry, "report-only": canonicalRegistryYAML} {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t, registry)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			eng := e.engine(func(o *Options) { o.Git = &cancellingGit{Git: e.mem, cancel: cancel} })
			d, err := eng.Check(ctx, e.request(e.good()))
			require.Error(t, err)
			assert.ErrorIs(t, err, context.Canceled)
			assert.False(t, d.Allowed)
			assert.NotEqual(t, DispositionPass, d.Disposition)
		})
	}
}

type brokenGit struct {
	Git
	resolveErr, ancestorErr error
}

func (b *brokenGit) ResolveCommit(ctx context.Context, ref string) (string, error) {
	if b.resolveErr != nil {
		return "", b.resolveErr
	}
	return b.Git.ResolveCommit(ctx, ref)
}

func (b *brokenGit) IsAncestor(ctx context.Context, a, d string) (bool, error) {
	if b.ancestorErr != nil {
		return false, b.ancestorErr
	}
	return b.Git.IsAncestor(ctx, a, d)
}

func TestU163_005_Remediation4GitExecErrorsAreSurfacedNotSwallowed(t *testing.T) {
	cases := map[string]*brokenGit{
		"ancestry exec error": {ancestorErr: errors.New("policyengine: git merge-base failed with exit code 128")},
		"resolve exec error":  {resolveErr: errors.New("policyengine: git rev-parse failed with exit code 128")},
	}
	for name, bg := range cases {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t, enforceRegistry)
			bg.Git = e.mem
			d := check(t, e.engine(func(o *Options) { o.Git = bg }), e.request(e.good()))
			assert.False(t, d.Allowed)
			require.NotEmpty(t, d.Errors)
			assert.NotEqual(t, CodeHeadUnpublished, d.Errors[0].Code, "an exec failure is not an unpublished head")
			assert.NotEqual(t, CodeNoProtectedRef, d.Errors[0].Code)
			assert.Contains(t, d.Errors[0].Reason, "exit code 128")
			assert.LessOrEqual(t, len(d.Errors[0].Reason), MaxErrorBytes)
		})
	}
}

type panickingDeriv struct{ fakeDeriv }

func (p *panickingDeriv) ReviewEvidence(context.Context, int) evidence.ReviewResult {
	panic("kaboom-" + strings.Repeat("x", 4000))
}

func TestU163_005_Remediation5PanicValueIsIncludedClipped(t *testing.T) {
	e := newEnv(t, enforceRegistry)
	pd := &panickingDeriv{}
	d := check(t, e.engine(func(o *Options) { o.Derivation = pd }), e.request(e.good()))
	assert.False(t, d.Allowed)
	var reason string
	for _, v := range d.Errors {
		if v.Node == string(NodeReviewComplete) && v.Code == CodeDerivationDegraded {
			reason = v.Reason
		}
	}
	require.NotEmpty(t, reason, "%+v", d.Errors)
	assert.Contains(t, reason, "kaboom")
	assert.LessOrEqual(t, len(reason), MaxErrorBytes)
}
