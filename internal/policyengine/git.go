package policyengine

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/softwaresalt/backlogit/internal/faultline"
)

// Git is the narrow repository view the engine verifies evidence against.
// Implementations must report an unknown commit or ref as an error, never as a
// plain false.
type Git interface {
	// ResolveCommit resolves a full ref name to a commit SHA. An absent ref
	// returns ErrRefNotFound.
	ResolveCommit(ctx context.Context, ref string) (string, error)
	// IsAncestor reports whether ancestor is reachable from descendant (a
	// commit is its own ancestor).
	IsAncestor(ctx context.Context, ancestor, descendant string) (bool, error)
	// CommitTime returns the committer time of a commit.
	CommitTime(ctx context.Context, sha string) (time.Time, error)
}

// ErrRefNotFound reports that a ref does not exist.
var ErrRefNotFound = errors.New("policyengine: ref not found")

const (
	// maxProtectedRefs caps the configured protected refs.
	maxProtectedRefs = 16
	maxRefLength     = 200
	gitTimeout       = 30 * time.Second
	maxGitOutput     = 4096
)

var refPattern = regexp.MustCompile(`^[A-Za-z0-9._/-]+$`)

// validRefName reports whether ref is a conservative, full ref name: it starts
// with refs/, uses a restricted alphabet, has no empty, dot-leading or
// dash-leading component and no "..".
func validRefName(ref string) bool {
	if len(ref) == 0 || len(ref) > maxRefLength || !strings.HasPrefix(ref, "refs/") || !refPattern.MatchString(ref) {
		return false
	}
	if strings.Contains(ref, "..") || strings.HasSuffix(ref, "/") || strings.HasSuffix(ref, ".") || strings.HasSuffix(ref, ".lock") {
		return false
	}
	for _, c := range strings.Split(ref, "/") {
		if c == "" || strings.HasPrefix(c, ".") || strings.HasPrefix(c, "-") {
			return false
		}
	}
	return true
}

// validProtectedRef additionally restricts protected refs to remote-tracking
// refs: a writable local branch is never a trust anchor.
func validProtectedRef(ref string) bool {
	return validRefName(ref) && strings.HasPrefix(ref, "refs/remotes/") && len(ref) > len("refs/remotes/")
}

// MemGit is an in-memory Git for tests, dry runs and incident-replay fixtures.
type MemGit struct {
	mu      sync.Mutex
	parents map[string][]string
	times   map[string]time.Time
	refs    map[string]string
}

// NewMemGit returns an empty in-memory repository.
func NewMemGit() *MemGit {
	return &MemGit{parents: map[string][]string{}, times: map[string]time.Time{}, refs: map[string]string{}}
}

// AddCommit records a commit with its committer time and parents.
func (m *MemGit) AddCommit(sha string, at time.Time, parents ...string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.parents[sha] = append([]string(nil), parents...)
	m.times[sha] = at
}

// SetRef points ref at sha.
func (m *MemGit) SetRef(ref, sha string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.refs[ref] = sha
}

// ResolveCommit implements Git.
func (m *MemGit) ResolveCommit(_ context.Context, ref string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	sha, ok := m.refs[ref]
	if !ok {
		return "", ErrRefNotFound
	}
	if _, ok := m.parents[sha]; !ok {
		return "", fmt.Errorf("policyengine: ref %s points at an unknown commit", clip(ref, 64))
	}
	return sha, nil
}

// IsAncestor implements Git.
func (m *MemGit) IsAncestor(_ context.Context, ancestor, descendant string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range []string{ancestor, descendant} {
		if _, ok := m.parents[s]; !ok {
			return false, fmt.Errorf("policyengine: unknown commit %s", clip(s, 64))
		}
	}
	seen := map[string]struct{}{descendant: {}}
	queue := []string{descendant}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur == ancestor {
			return true, nil
		}
		ps := append([]string(nil), m.parents[cur]...)
		sort.Strings(ps)
		for _, p := range ps {
			if _, ok := seen[p]; ok {
				continue
			}
			seen[p] = struct{}{}
			queue = append(queue, p)
		}
	}
	return false, nil
}

// CommitTime implements Git.
func (m *MemGit) CommitTime(_ context.Context, sha string) (time.Time, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	at, ok := m.times[sha]
	if !ok {
		return time.Time{}, fmt.Errorf("policyengine: unknown commit %s", clip(sha, 64))
	}
	return at, nil
}

// ExecGit is a Git backed by the git executable. Every command runs in Dir
// with all ambient GIT_* variables removed, so neither the environment nor a
// parent process can redirect it to another repository.
type ExecGit struct {
	// Dir is the repository working directory.
	Dir string
}

type limitedBuffer struct {
	buf      bytes.Buffer
	max      int
	overflow bool
}

func (l *limitedBuffer) Write(p []byte) (int, error) {
	if l.buf.Len()+len(p) > l.max {
		l.overflow = true
		return len(p), nil
	}
	return l.buf.Write(p)
}

func gitEnv() []string {
	env := make([]string, 0, 32)
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(strings.ToUpper(k), "GIT_") {
			continue
		}
		env = append(env, kv)
	}
	return append(env, "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0", "LC_ALL=C")
}

// run executes git and returns bounded stdout and the exit code (-1 when git
// did not run to completion).
func (g ExecGit) run(ctx context.Context, args ...string) (string, int, error) {
	if g.Dir == "" {
		return "", -1, errors.New("policyengine: git repository directory is required")
	}
	ctx, cancel := context.WithTimeout(ctx, gitTimeout)
	defer cancel()
	// Replace refs and grafts must never rewrite the ancestry being verified.
	full := append([]string{"--no-replace-objects", "-c", "core.useReplaceRefs=false"}, args...)
	cmd := exec.CommandContext(ctx, "git", full...)
	cmd.Dir = g.Dir
	cmd.Env = gitEnv()
	out := &limitedBuffer{max: maxGitOutput}
	cmd.Stdout = out
	cmd.Stderr = &limitedBuffer{max: maxGitOutput}
	err := cmd.Run()
	// A killed or timed-out git must never be read as an exit-code verdict.
	if cerr := ctx.Err(); cerr != nil {
		return "", -1, fmt.Errorf("policyengine: git %s: %w", args[0], cerr)
	}
	if out.overflow {
		return "", -1, errors.New("policyengine: git output exceeded its bound")
	}
	if err == nil {
		return strings.TrimSpace(out.buf.String()), 0, nil
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return "", ee.ExitCode(), fmt.Errorf("policyengine: git %s failed with exit code %d", args[0], ee.ExitCode())
	}
	return "", -1, fmt.Errorf("policyengine: run git %s: %w", args[0], err)
}

// Toplevel returns the repository's top-level working directory.
func (g ExecGit) Toplevel(ctx context.Context) (string, error) {
	out, _, err := g.run(ctx, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	if out == "" || strings.ContainsRune(out, 0) {
		return "", errors.New("policyengine: git returned an invalid repository root")
	}
	return filepath.Clean(filepath.FromSlash(out)), nil
}

// OriginURL returns the configured URL of the origin remote. A repository
// without an origin is an error.
func (g ExecGit) OriginURL(ctx context.Context) (string, error) {
	out, _, err := g.run(ctx, "remote", "get-url", "origin")
	if err != nil {
		return "", err
	}
	if out == "" || strings.ContainsRune(out, 0) {
		return "", errors.New("policyengine: git returned an invalid origin URL")
	}
	return out, nil
}

// ResolveCommit implements Git.
func (g ExecGit) ResolveCommit(ctx context.Context, ref string) (string, error) {
	if !validRefName(ref) {
		return "", fmt.Errorf("policyengine: %q is not a valid full ref name", clip(ref, 64))
	}
	out, code, err := g.run(ctx, "rev-parse", "--verify", "--quiet", ref+"^{commit}")
	if code == 1 {
		return "", ErrRefNotFound
	}
	if err != nil {
		return "", err
	}
	if !faultline.ValidCommitSHA(out) {
		return "", fmt.Errorf("policyengine: git returned an invalid commit for %s", clip(ref, 64))
	}
	return out, nil
}

func (g ExecGit) requireCommit(ctx context.Context, sha string) error {
	if !faultline.ValidCommitSHA(sha) {
		return fmt.Errorf("policyengine: %q is not a full commit SHA", clip(sha, 64))
	}
	_, code, err := g.run(ctx, "cat-file", "-e", sha+"^{commit}")
	if err != nil {
		if code > 0 {
			return fmt.Errorf("policyengine: unknown commit %s", clip(sha, 64))
		}
		return err
	}
	return nil
}

// IsAncestor implements Git.
func (g ExecGit) IsAncestor(ctx context.Context, ancestor, descendant string) (bool, error) {
	for _, s := range []string{ancestor, descendant} {
		if err := g.requireCommit(ctx, s); err != nil {
			return false, err
		}
	}
	_, code, err := g.run(ctx, "merge-base", "--is-ancestor", ancestor, descendant)
	switch {
	case err == nil:
		return true, nil
	case code == 1:
		return false, nil
	default:
		return false, err
	}
}

// CommitTime implements Git.
func (g ExecGit) CommitTime(ctx context.Context, sha string) (time.Time, error) {
	if err := g.requireCommit(ctx, sha); err != nil {
		return time.Time{}, err
	}
	out, _, err := g.run(ctx, "log", "-1", "--no-walk", "--format=%ct", sha)
	if err != nil {
		return time.Time{}, err
	}
	sec, perr := strconv.ParseInt(out, 10, 64)
	if perr != nil || sec <= 0 {
		return time.Time{}, fmt.Errorf("policyengine: git returned an invalid commit time for %s", clip(sha, 64))
	}
	return time.Unix(sec, 0).UTC(), nil
}
