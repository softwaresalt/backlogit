package config

import (
	"errors"
	"os"
	"os/exec"
	"testing"
)

// TestReconcileConfig_Normalize_UnsetDefaultsToResolvedBranch covers 167.012-T:
// when TrustedRefs is unset, Normalize fills it from the injected resolver
// rather than the unrelated pre-task-gate BaseRef default.
func TestReconcileConfig_Normalize_UnsetDefaultsToResolvedBranch(t *testing.T) {
	r := &ReconcileConfig{}
	resolveCalls := 0
	resolve := func(workspacePath string) (string, error) {
		resolveCalls++
		return "release/main", nil
	}
	if err := r.Normalize("/tmp/workspace", resolve); err != nil {
		t.Fatalf("Normalize returned error: %v", err)
	}
	if resolveCalls != 1 {
		t.Fatalf("expected resolver to be called exactly once, got %d", resolveCalls)
	}
	if len(r.TrustedRefs) != 1 || r.TrustedRefs[0] != "release/main" {
		t.Fatalf("expected TrustedRefs=[release/main], got %v", r.TrustedRefs)
	}
}

// TestReconcileConfig_Normalize_ExplicitRefsParsed covers the "explicit refs
// parsed" branch: a populated TrustedRefs is preserved verbatim and the
// resolver is never invoked.
func TestReconcileConfig_Normalize_ExplicitRefsParsed(t *testing.T) {
	r := &ReconcileConfig{TrustedRefs: []string{"main", "release/2026-09"}}
	resolve := func(workspacePath string) (string, error) {
		t.Fatal("resolver must not be called when trusted_refs is already set")
		return "", nil
	}
	if err := r.Normalize("/tmp/workspace", resolve); err != nil {
		t.Fatalf("Normalize returned error: %v", err)
	}
	if len(r.TrustedRefs) != 2 || r.TrustedRefs[0] != "main" || r.TrustedRefs[1] != "release/2026-09" {
		t.Fatalf("expected explicit TrustedRefs preserved, got %v", r.TrustedRefs)
	}
}

// TestReconcileConfig_Normalize_ResolverErrorPropagates ensures a resolver
// failure fails closed rather than silently defaulting to an empty list.
func TestReconcileConfig_Normalize_ResolverErrorPropagates(t *testing.T) {
	r := &ReconcileConfig{}
	resolve := func(workspacePath string) (string, error) {
		return "", errors.New("boom")
	}
	if err := r.Normalize("/tmp/workspace", resolve); err == nil {
		t.Fatal("expected Normalize to propagate resolver error")
	}
	if len(r.TrustedRefs) != 0 {
		t.Fatalf("expected TrustedRefs to remain empty on error, got %v", r.TrustedRefs)
	}
}

// TestReconcileConfig_Normalize_ResolverBlankResultRejected ensures a
// whitespace-only resolver result is treated as a failure, not a silently
// accepted empty trusted ref.
func TestReconcileConfig_Normalize_ResolverBlankResultRejected(t *testing.T) {
	r := &ReconcileConfig{}
	resolve := func(workspacePath string) (string, error) {
		return "   ", nil
	}
	if err := r.Normalize("/tmp/workspace", resolve); err == nil {
		t.Fatal("expected Normalize to reject a blank resolved branch")
	}
}

// TestReconcileConfig_Normalize_NilReceiverIsNoop guards the nil-safety of the
// optional *ReconcileConfig field.
func TestReconcileConfig_Normalize_NilReceiverIsNoop(t *testing.T) {
	var r *ReconcileConfig
	if err := r.Normalize("/tmp/workspace", nil); err != nil {
		t.Fatalf("expected nil-receiver Normalize to be a no-op, got error: %v", err)
	}
}

// TestReconcileConfig_Validate_InvalidRefsRejected covers the "invalid
// rejected" acceptance branch: an explicit empty-string entry in trusted_refs
// must fail struct validation.
func TestReconcileConfig_Validate_InvalidRefsRejected(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Reconcile = &ReconcileConfig{TrustedRefs: []string{"main", ""}}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected Validate to reject an empty trusted_refs entry")
	}
}

// TestReconcileConfig_Validate_ValidRefsAccepted is the positive control for
// the above: a fully populated, non-empty trusted_refs list validates clean.
func TestReconcileConfig_Validate_ValidRefsAccepted(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Reconcile = &ReconcileConfig{TrustedRefs: []string{"main"}}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected Validate to accept a populated trusted_refs list, got %v", err)
	}
}

// TestResolveDefaultBranch_NeverErrors documents the best-effort contract:
// ResolveDefaultBranch always falls back rather than returning an error, even
// against a workspace path with no git repository.
func TestResolveDefaultBranch_NeverErrors(t *testing.T) {
	dir := t.TempDir()
	branch, err := ResolveDefaultBranch(dir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if branch == "" {
		t.Fatal("expected a non-empty fallback branch name")
	}
}

// TestResolveDefaultBranch_ReturnsRemoteTrackingRef covers 167.012-T (PR #440
// review round 2): when origin/HEAD resolves, ResolveDefaultBranch must
// return the remote-tracking ref itself (e.g. "origin/main"), NOT the bare
// local branch name with the "origin/" prefix stripped. Downstream
// reachability checks resolve this value against the remote-tracking state;
// returning the bare local name would let a stale or locally-ahead local
// branch of the same name be trusted instead of what the remote actually
// reports as its default.
func TestResolveDefaultBranch_ReturnsRemoteTrackingRef(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available on PATH")
	}
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = os.Environ()
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %s: %v", args, out, err)
		}
	}
	run("init", "-q")
	// Simulate a cloned repo's remote-tracking default-branch symref without
	// requiring an actual network remote: refs/remotes/origin/HEAD points at
	// refs/remotes/origin/main.
	run("symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")

	branch, err := ResolveDefaultBranch(dir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if branch != "origin/main" {
		t.Fatalf("expected the remote-tracking ref %q, got %q", "origin/main", branch)
	}
}
