package policyengine

import (
	"context"
	"errors"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestU163_008_ExecGitSurfacesContextErrorsNotExitCodes(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	dir := t.TempDir()
	runGitFixture(t, dir, "init", "-q", "-b", "main")
	runGitFixture(t, dir, "commit", "-q", "--allow-empty", "-m", "c")
	head := runGitFixture(t, dir, "rev-parse", "HEAD")
	g := ExecGit{Dir: dir}

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

	_, err := g.ResolveCommit(ctx, "refs/remotes/origin/main")
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrRefNotFound)
	assert.ErrorIs(t, err, context.DeadlineExceeded)

	ok, err := g.IsAncestor(ctx, head, head)
	require.Error(t, err)
	assert.False(t, ok)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

type cancelOnCommitTimeGit struct {
	Git
	cancel context.CancelFunc
}

func (c *cancelOnCommitTimeGit) CommitTime(ctx context.Context, sha string) (time.Time, error) {
	t, err := c.Git.CommitTime(ctx, sha)
	c.cancel()
	return t, err
}

func TestU163_008_CancellationAfterGateLoopIsNeverAllowed(t *testing.T) {
	for name, registry := range map[string]string{"enforce": enforceRegistry, "report-only": canonicalRegistryYAML} {
		t.Run(name, func(t *testing.T) {
			e := newEnv(t, registry)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			eng := e.engine(func(o *Options) { o.Git = &cancelOnCommitTimeGit{Git: e.mem, cancel: cancel} })
			d, err := eng.Check(ctx, e.request(e.good()))
			require.Error(t, err)
			assert.True(t, errors.Is(err, context.Canceled), "%v", err)
			assert.False(t, d.Allowed)
			assert.NotEqual(t, DispositionPass, d.Disposition)
		})
	}
}
