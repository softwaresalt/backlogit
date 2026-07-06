package gate

import (
	"bytes"
	"context"
	stderrors "errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"runtime"

	bkerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// GateRunner is the injectable exec seam for `autoharness gate check`. Unit tests
// supply a fake so no real binary is required; integration tests may use the
// installed autoharness. Failure-to-run is a returned error (never a struct
// field): a binary that cannot be resolved wraps ErrGateBinaryNotFound, a
// context-deadline kill wraps ErrGateTimeout, and a process that ran and exited N
// returns GateResult{ExitCode:N} with a nil error.
type GateRunner interface {
	Run(ctx context.Context, args []string, workspace string, env []string) (GateResult, error)
}

// GateCheckRequest is the fixed argv template's positional inputs. Values are
// passed as discrete argv elements — never concatenated into a shell string — so
// no shell-metacharacter injection is possible.
type GateCheckRequest struct {
	ItemID        string
	BaseRef       string
	HeadRef       string
	WorkspaceRoot string
	Force         bool
	NoCount       bool
}

// BuildArgs assembles the argv slice for `autoharness gate check`. It is a pure
// function so the exact argv is golden-testable. Force and NoCount are mutually
// exclusive in autoharness; callers must not set both.
func BuildArgs(req GateCheckRequest) []string {
	args := []string{
		"gate", "check", "--json",
		"--base", req.BaseRef,
		"--head", req.HeadRef,
		"--workspace", req.WorkspaceRoot,
	}
	if req.ItemID != "" {
		args = append(args, "--task", req.ItemID)
	}
	if req.Force {
		args = append(args, "--force")
	}
	if req.NoCount {
		args = append(args, "--no-count")
	}
	return args
}

// ExecRunner is the default GateRunner backed by os/exec with argv-array
// execution only (never a shell string).
type ExecRunner struct {
	// Binary is the resolved autoharness executable (PATH name or vetted path).
	Binary string
}

// Run executes the gate check, honoring the context deadline for the timeout.
func (r ExecRunner) Run(ctx context.Context, args []string, workspace string, env []string) (GateResult, error) {
	cmd := exec.CommandContext(ctx, r.Binary, args...)
	cmd.Dir = workspace
	cmd.Env = env

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	res := GateResult{Stdout: stdout.Bytes(), Stderr: stderr.Bytes()}
	if runErr == nil {
		res.ExitCode = 0
		return res, nil
	}

	// Timeout MUST be checked before ExitError: a killed process reports a
	// platform-dependent exit code (-1 on Windows) that must never be read as a
	// blocked (exit 1) outcome.
	if stderrors.Is(ctx.Err(), context.DeadlineExceeded) {
		return res, fmt.Errorf("gate check timed out: %w", bkerrors.ErrGateTimeout)
	}

	var ee *exec.ExitError
	if stderrors.As(runErr, &ee) {
		res.ExitCode = ee.ExitCode()
		return res, nil
	}

	if stderrors.Is(runErr, exec.ErrNotFound) || stderrors.Is(runErr, fs.ErrNotExist) {
		return res, fmt.Errorf("resolve autoharness binary %q: %w", r.Binary, bkerrors.ErrGateBinaryNotFound)
	}

	return res, fmt.Errorf("run gate check: %w", runErr)
}

// MinimalEnv returns an allowlisted environment for the gate subprocess rather
// than inheriting the full ambient environment (trust-boundary hardening). It
// forwards only the variables autoharness and git need to resolve tools and the
// repository.
func MinimalEnv() []string {
	allow := []string{
		"PATH", "HOME", "USERPROFILE", "SYSTEMROOT", "SystemRoot",
		"TEMP", "TMP", "TMPDIR", "LANG", "LC_ALL",
		"GIT_DIR", "GIT_CONFIG_GLOBAL", "GIT_CONFIG_NOSYSTEM",
		"HOMEDRIVE", "HOMEPATH", "APPDATA", "LOCALAPPDATA",
		"PATHEXT", "COMSPEC",
	}
	var env []string
	seen := map[string]struct{}{}
	for _, key := range allow {
		if _, dup := seen[key]; dup {
			continue
		}
		if v, ok := os.LookupEnv(key); ok {
			env = append(env, key+"="+v)
			seen[key] = struct{}{}
		}
	}
	_ = runtime.GOOS
	return env
}
