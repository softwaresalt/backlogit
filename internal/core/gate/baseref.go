package gate

import (
	"bytes"
	"context"
	stderrors "errors"
	"fmt"
	"os/exec"
	"strings"

	bkerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// GitRunner is the injected git seam for ref resolution/verification, mirroring
// the confineFn test-seam style in core. Unit tests supply a fake so no real git
// dependency is needed.
type GitRunner interface {
	// Verify reports whether ref resolves to a commit (git rev-parse --verify).
	Verify(ctx context.Context, ref string) (bool, error)
}

// BaseRefInput carries the operator-influenced inputs to base resolution.
type BaseRefInput struct {
	// ConfigBaseRef is config base_ref. "auto" or "" means unset (auto discovery).
	ConfigBaseRef string
	// GateBase is the operator-only --gate-base override (never set from MCP).
	GateBase string
}

// ResolvedBase is the outcome of base resolution.
type ResolvedBase struct {
	// Ref is the resolved default-branch ref passed to autoharness as --base.
	Ref string
	// NonDefault is true when the base came from an explicit override that does
	// not equal the discovered default-branch ref — a privileged break-glass that
	// MUST be audited (pre_task_completion_gate_base_override).
	NonDefault bool
	// Source records where the base came from ("config" | "gate_base" | "auto").
	Source string
}

// defaultCandidates is the auto-discovery precedence for the default-branch ref.
var defaultCandidates = []string{"origin/HEAD", "origin/main", "main"}

// ResolveBaseRef resolves and verifies the base ref, refusing (config error) when
// an explicit override or HEAD cannot be verified. Because `autoharness gate
// check` degrades a bad/unresolvable ref to an empty diff and exit 0, an
// unverified ref while enforcing would silently suppress gating; verifying here
// closes that fail-open gap. The head ref is always the fixed constant HEAD.
func ResolveBaseRef(ctx context.Context, git GitRunner, in BaseRefInput) (ResolvedBase, error) {
	if ok, err := git.Verify(ctx, HeadRef); err != nil {
		return ResolvedBase{}, fmt.Errorf("verify HEAD: %w", err)
	} else if !ok {
		return ResolvedBase{}, fmt.Errorf("HEAD does not resolve to a commit: %w", bkerrors.ErrGateConfig)
	}

	defaultRef := discoverDefault(ctx, git)

	if base := strings.TrimSpace(in.ConfigBaseRef); base != "" && base != "auto" {
		return resolveExplicit(ctx, git, base, "config", defaultRef)
	}
	if base := strings.TrimSpace(in.GateBase); base != "" {
		return resolveExplicit(ctx, git, base, "gate_base", defaultRef)
	}

	if defaultRef == "" {
		return ResolvedBase{}, fmt.Errorf(
			"no default base ref resolves (tried origin/HEAD, origin/main, main): %w",
			bkerrors.ErrGateConfig)
	}
	return ResolvedBase{Ref: defaultRef, NonDefault: false, Source: "auto"}, nil
}

// resolveExplicit verifies an operator-supplied base ref and flags whether it
// diverges from the discovered default branch.
func resolveExplicit(ctx context.Context, git GitRunner, ref, source, defaultRef string) (ResolvedBase, error) {
	ok, err := git.Verify(ctx, ref)
	if err != nil {
		return ResolvedBase{}, fmt.Errorf("verify %s base ref %q: %w", source, ref, err)
	}
	if !ok {
		return ResolvedBase{}, fmt.Errorf("%s base ref %q does not resolve: %w", source, ref, bkerrors.ErrGateConfig)
	}
	return ResolvedBase{Ref: ref, NonDefault: ref != defaultRef, Source: source}, nil
}

// discoverDefault returns the first verifying default-branch candidate, or "".
func discoverDefault(ctx context.Context, git GitRunner) string {
	for _, cand := range defaultCandidates {
		if ok, err := git.Verify(ctx, cand); err == nil && ok {
			return cand
		}
	}
	return ""
}

// ExecGitRunner is the default GitRunner backed by os/exec git.
type ExecGitRunner struct {
	// Dir is the repository working directory.
	Dir string
	// Env is an allowlisted environment (see MinimalEnv).
	Env []string
}

// Verify runs `git rev-parse --verify --quiet <ref>^{commit}` and reports whether
// it resolves.
func (g ExecGitRunner) Verify(ctx context.Context, ref string) (bool, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--verify", "--quiet", ref+"^{commit}")
	cmd.Dir = g.Dir
	if g.Env != nil {
		cmd.Env = g.Env
	}
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	err := cmd.Run()
	if err == nil {
		return strings.TrimSpace(stdout.String()) != "", nil
	}
	// Exit code 1 with no output means the ref does not resolve — not an error.
	var ee *exec.ExitError
	if stderrors.As(err, &ee) {
		return false, nil
	}
	return false, fmt.Errorf("git rev-parse %q: %w", ref, err)
}
