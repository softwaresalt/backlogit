package config

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// DefaultBranchResolver resolves the effective default branch name for a
// workspace root, used by ReconcileConfig.Normalize when trusted_refs is
// unset. It is a package-level type so tests can inject a fake resolver
// instead of shelling out to git.
type DefaultBranchResolver func(workspacePath string) (string, error)

// resolveDefaultBranchTimeout bounds the git subprocess calls in
// ResolveDefaultBranch so a hung or misconfigured remote cannot block config
// loading indefinitely.
const resolveDefaultBranchTimeout = 5 * time.Second

// ResolveDefaultBranch resolves the repository default branch for
// workspacePath by trying, in order: the symbolic ref of origin/HEAD, the
// current checked-out branch, and finally the literal fallback "main". It
// never returns an error — git resolution is best-effort by design so a
// workspace with no git remote (or no git at all) can still normalize
// reconcile.trusted_refs deterministically.
func ResolveDefaultBranch(workspacePath string) (string, error) {
	if branch, ok := gitSymbolicRefShort(workspacePath, "refs/remotes/origin/HEAD"); ok {
		return strings.TrimPrefix(branch, "origin/"), nil
	}
	if branch, ok := gitCurrentBranch(workspacePath); ok {
		return branch, nil
	}
	return "main", nil
}

func gitSymbolicRefShort(workspacePath, ref string) (string, bool) {
	out, err := runGit(workspacePath, "symbolic-ref", "--short", ref)
	if err != nil {
		return "", false
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return "", false
	}
	return out, true
}

func gitCurrentBranch(workspacePath string) (string, bool) {
	out, err := runGit(workspacePath, "branch", "--show-current")
	if err != nil {
		return "", false
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return "", false
	}
	return out, true
}

func runGit(workspacePath string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), resolveDefaultBranchTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", workspacePath}, args...)...)
	cmd.Env = ChildProcessEnv()
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %v: %w", args, err)
	}
	return stdout.String(), nil
}
