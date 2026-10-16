package evidence

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// Status states whether evidence was derived or the source was unavailable.
type Status string

const (
	// StatusVerified means the evidence was derived from the source.
	StatusVerified Status = "verified"
	// StatusDegraded means the source was unavailable; the evidence must never
	// be treated as a pass.
	StatusDegraded Status = "degraded"
)

// ErrCommandFailed marks a non-zero exit from the evidence subprocess.
var ErrCommandFailed = errors.New("evidence command failed")

// Runner executes an external command. Implementations must not place secrets
// in args; credentials travel through env or stdin only.
type Runner interface {
	// Run executes name with args, env (KEY=VALUE entries appended to the
	// process environment) and optional stdin, returning stdout and stderr.
	Run(ctx context.Context, name string, args, env []string, stdin []byte) (stdout, stderr []byte, err error)
}

// ExecRunner is the os/exec backed Runner.
type ExecRunner struct{}

// Run implements Runner using os/exec.
func (ExecRunner) Run(ctx context.Context, name string, args, env []string, stdin []byte) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = append(os.Environ(), env...)
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return out.Bytes(), errb.Bytes(), fmt.Errorf("run %s: %w", name, err)
	}
	return out.Bytes(), errb.Bytes(), nil
}

var scrubPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)authorization\s*[:=][^\r\n]*`),
	regexp.MustCompile(`(?i)\bbearer\s+[A-Za-z0-9._~+/=-]+`),
	regexp.MustCompile(`github_pat_[A-Za-z0-9_]+`),
	regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{20,}`),
}

const redacted = "[REDACTED]"

// Scrub redacts credential-like values (GitHub token shapes, Authorization
// headers, Bearer values) from s.
func Scrub(s string) string {
	for _, re := range scrubPatterns {
		s = re.ReplaceAllString(s, redacted)
	}
	return s
}

// degraded builds the standard scrubbed degradation reason for a failed
// subprocess. Known secrets are removed in addition to pattern scrubbing.
func degraded(what string, err error, stderr []byte, secrets ...string) string {
	msg := fmt.Sprintf("%s: %v: %s", what, err, strings.TrimSpace(string(stderr)))
	for _, s := range secrets {
		if s != "" {
			msg = strings.ReplaceAll(msg, s, redacted)
		}
	}
	return Scrub(msg)
}
