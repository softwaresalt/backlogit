package gate

import (
	"bytes"
	"context"
	stderrors "errors"
	"fmt"
	"io/fs"
	"os/exec"
	"strconv"
	"strings"

	"github.com/softwaresalt/backlogit/internal/config"
	bkerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// VersionRunner is the injected seam for the autoharness version/contract probe.
type VersionRunner interface {
	// Version returns the autoharness version string (e.g. "1.4.7"). It wraps
	// ErrGateBinaryNotFound when the binary is unresolvable.
	Version(ctx context.Context) (string, error)
}

// Probe resolves the autoharness binary and checks the version/contract floor.
// It returns whether gates are enforceable and, when they are not enforceable
// under strict enablement, a setup-class error.
//
//   - enabled:false      -> enforce=false, nil (kill switch).
//   - enabled:auto  + unresolvable/incompatible -> enforce=false, nil (fail open).
//   - enabled:true  + unresolvable/incompatible -> enforce=false, GateError{setup} (fail closed).
//   - resolvable + compatible (>= MinAutoharnessVersion) -> enforce=true, nil.
func Probe(ctx context.Context, vr VersionRunner, enabled EnabledMode) (enforce bool, err error) {
	if enabled == EnabledFalse {
		return false, nil
	}

	ver, verr := vr.Version(ctx)
	if verr != nil {
		return failProbe(enabled, "autoharness binary not resolvable: "+verr.Error())
	}

	ok, cmpErr := versionAtLeast(ver, MinAutoharnessVersion)
	if cmpErr != nil {
		return failProbe(enabled, fmt.Sprintf("cannot parse autoharness version %q: %v", ver, cmpErr))
	}
	if !ok {
		return failProbe(enabled, fmt.Sprintf(
			"autoharness %s is below the required floor %s (repeated_failure contract)",
			strings.TrimSpace(ver), MinAutoharnessVersion))
	}
	return true, nil
}

// failProbe returns the fail-open (auto) or fail-closed (true) probe outcome.
func failProbe(enabled EnabledMode, msg string) (bool, error) {
	if enabled == EnabledTrue {
		return false, &bkerrors.GateError{Class: "setup", Message: msg, Err: bkerrors.ErrGateSetup}
	}
	return false, nil
}

// versionAtLeast reports whether have >= want using dotted numeric comparison.
func versionAtLeast(have, want string) (bool, error) {
	hv, err := parseVersion(have)
	if err != nil {
		return false, err
	}
	wv, err := parseVersion(want)
	if err != nil {
		return false, err
	}
	for i := 0; i < 3; i++ {
		if hv[i] != wv[i] {
			return hv[i] > wv[i], nil
		}
	}
	return true, nil
}

// parseVersion extracts a major.minor.patch triple from a version string,
// tolerating a leading "v" and trailing pre-release/build metadata.
func parseVersion(v string) ([3]int, error) {
	var out [3]int
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	// Cut at the first non-version separator (space, '-', '+').
	if i := strings.IndexAny(v, " -+"); i >= 0 {
		v = v[:i]
	}
	parts := strings.Split(v, ".")
	if len(parts) == 0 || parts[0] == "" {
		return out, fmt.Errorf("empty version")
	}
	for i := 0; i < 3 && i < len(parts); i++ {
		n, err := strconv.Atoi(parts[i])
		if err != nil {
			return out, fmt.Errorf("invalid version segment %q", parts[i])
		}
		out[i] = n
	}
	return out, nil
}

// ExecVersionRunner is the default VersionRunner backed by `autoharness version`.
type ExecVersionRunner struct {
	Binary string
	Dir    string
	Env    []string
}

// Version runs `<binary> version` and returns the first non-empty output line.
func (r ExecVersionRunner) Version(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, r.Binary, "version")
	cmd.Dir = r.Dir
	if r.Env != nil {
		cmd.Env = r.Env
	} else {
		// Default to the ambient environment scrubbed of the formal-gate
		// evidence key (106-F F1/U2) rather than leaving Env nil, which would
		// otherwise inherit the full ambient environment unfiltered.
		cmd.Env = config.ChildProcessEnv()
	}
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		if stderrors.Is(err, exec.ErrNotFound) || stderrors.Is(err, fs.ErrNotExist) {
			return "", fmt.Errorf("resolve autoharness binary %q: %w", r.Binary, bkerrors.ErrGateBinaryNotFound)
		}
		return "", fmt.Errorf("run autoharness version: %w", err)
	}
	return firstVersionToken(stdout.String()), nil
}

// firstVersionToken extracts a version-looking token from `autoharness version`
// output, which may be "1.4.7" or "version 1.4.7".
func firstVersionToken(out string) string {
	for _, field := range strings.Fields(out) {
		if _, err := parseVersion(field); err == nil {
			return field
		}
	}
	return strings.TrimSpace(out)
}
