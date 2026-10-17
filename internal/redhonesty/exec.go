package redhonesty

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/softwaresalt/backlogit/internal/faultline"
)

// Resource budgets. Every read, extraction and process output is bounded so a
// hostile repository or runaway test cannot exhaust memory or disk.
const (
	maxGitOutputBytes  = 8 << 20   // listing-style git output
	maxSourceBytes     = 4 << 20   // one source/test file
	maxChangeBytes     = 64 << 20  // cumulative blob content held for one change
	maxChangedFiles    = 5000      // files considered in one change
	maxJSONLineBytes   = 1 << 20   // one `go test -json` line
	maxTestOutputBytes = 64 << 20  // total `go test -json` output
	maxArchiveBytes    = 2 << 30   // total extracted baseline bytes
	maxArchiveFileSize = 512 << 20 // one extracted file
	maxArchiveEntries  = 200000    // extracted entries
	maxStderrBytes     = 4 << 10   // retained stderr for diagnostics
	defaultTestTimeout = 10 * time.Minute
	processWaitDelay   = 5 * time.Second
)

var errOutputTooLarge = errors.New("redhonesty: command output exceeds limit")

// limitedBuffer collects at most max bytes and then fails the write.
type limitedBuffer struct {
	buf bytes.Buffer
	max int
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.buf.Len()+len(p) > b.max {
		return 0, errOutputTooLarge
	}
	return b.buf.Write(p)
}

// headBuffer keeps only the first max bytes and silently discards the rest; it
// never fails the writer. Used for diagnostic stderr.
type headBuffer struct {
	buf bytes.Buffer
	max int
}

func (b *headBuffer) Write(p []byte) (int, error) {
	if room := b.max - b.buf.Len(); room > 0 {
		if len(p) < room {
			room = len(p)
		}
		b.buf.Write(p[:room])
	}
	return len(p), nil
}

func (b *headBuffer) String() string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 && r != '\n' && r != '\t' {
			return ' '
		}
		return r
	}, b.buf.String())
}

// gitEnv returns the child environment for git: the caller's environment minus
// every GIT_* variable (so GIT_DIR / GIT_WORK_TREE / GIT_INDEX_FILE cannot
// redirect the gate to another repository), plus non-interactive settings.
func gitEnv() []string {
	env := make([]string, 0, 32)
	for _, kv := range os.Environ() {
		if strings.HasPrefix(strings.ToUpper(kv), "GIT_") {
			continue
		}
		env = append(env, kv)
	}
	return append(env, "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0")
}

var secretEnvMarkers = []string{"TOKEN", "SECRET", "PASSWORD", "PASSWD", "CREDENTIAL", "APIKEY", "API_KEY", "PRIVATE_KEY"}

// goTestEnv returns the environment for `go test`: the caller's environment
// without likely credentials, with the toolchain pinned to the local one, the
// workspace file disabled and any ambient GOFLAGS dropped so the measurement is
// reproducible. This limits accidental exposure; it is not a sandbox (see the
// package trust-model note).
func goTestEnv() []string {
	env := make([]string, 0, 64)
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		upper := strings.ToUpper(name)
		if upper == "GOFLAGS" || upper == "GOTOOLCHAIN" || upper == "GOWORK" {
			continue
		}
		secret := false
		for _, m := range secretEnvMarkers {
			if strings.Contains(upper, m) {
				secret = true
				break
			}
		}
		if secret {
			continue
		}
		env = append(env, kv)
	}
	return append(env, "GOTOOLCHAIN=local", "GOWORK=off", "GOFLAGS=")
}

// ExecGit is the production Git implementation backed by the git binary.
type ExecGit struct{}

// Run implements Git. Arguments are passed as discrete argv entries; nothing is
// interpolated into a shell string.
func (ExecGit) Run(ctx context.Context, dir string, stdout io.Writer, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Stdout = stdout
	stderr := &headBuffer{max: maxStderrBytes}
	cmd.Stderr = stderr
	cmd.Env = gitEnv()
	cmd.WaitDelay = processWaitDelay
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git %s: %w: %s", firstArg(args), err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

func firstArg(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return args[0]
}

// gitOutput runs git and returns its stdout, bounded by max bytes.
func gitOutput(ctx context.Context, g Git, dir string, max int, args ...string) ([]byte, error) {
	lb := &limitedBuffer{max: max}
	if err := g.Run(ctx, dir, lb, args...); err != nil {
		return nil, err
	}
	return lb.buf.Bytes(), nil
}

// GoTestExecutor is the production TestExecutor: it shells out to `go test
// -json` with an anchored -run selector. Output is collected through
// cmd.Stdout (so Wait owns the copy and WaitDelay bounds a leaked pipe), the
// run is bounded by Timeout and by go test's own -timeout, and a timeout aborts
// the run with an error rather than being reported as a test result.
type GoTestExecutor struct {
	// Timeout bounds one package run; zero means defaultTestTimeout.
	Timeout time.Duration
}

// Run implements TestExecutor.
func (e GoTestExecutor) Run(ctx context.Context, dir, pkg string, names []string) (map[string]Result, error) {
	if err := validPackageDir(pkg); err != nil {
		return nil, err
	}
	for _, n := range names {
		if !faultline.ValidTestName(n) {
			return nil, fmt.Errorf("redhonesty: unsafe test name %q", truncate(n))
		}
	}
	if len(names) == 0 {
		return map[string]Result{}, nil
	}
	timeout := e.Timeout
	if timeout <= 0 {
		timeout = defaultTestTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	target := "./" + pkg
	if pkg == "." {
		target = "."
	}
	// go test's own timeout fires slightly before ours so the test binary is
	// stopped by the toolchain rather than orphaned by a killed parent.
	inner := timeout - processWaitDelay
	if inner < time.Second {
		inner = time.Second
	}
	args := []string{"test", "-json", "-count=1", "-vet=off", "-timeout", inner.Round(time.Second).String(),
		"-run", "^(" + strings.Join(names, "|") + ")$", target}
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = dir
	cmd.Env = goTestEnv()
	cmd.WaitDelay = processWaitDelay
	out := &limitedBuffer{max: maxTestOutputBytes}
	stderr := &headBuffer{max: maxStderrBytes}
	cmd.Stdout = out
	cmd.Stderr = stderr

	waitErr := cmd.Run()
	if ctx.Err() != nil {
		return nil, fmt.Errorf("redhonesty: go test %s: %w", pkg, ctx.Err())
	}
	if errors.Is(waitErr, errOutputTooLarge) {
		return nil, fmt.Errorf("redhonesty: go test %s: %w", pkg, errOutputTooLarge)
	}
	var exitErr *exec.ExitError
	if waitErr != nil && !errors.As(waitErr, &exitErr) {
		return nil, fmt.Errorf("redhonesty: go test %s: %w: %s", pkg, waitErr, strings.TrimSpace(stderr.String()))
	}
	// A non-zero exit is the normal signal of a failing or unbuildable package;
	// the parsed events (or their absence) carry the per-test truth.
	return parseGoTestJSON(bytes.NewReader(out.buf.Bytes()))
}

// bufioLineScanner builds a scanner whose single-line limit is maxJSONLineBytes.
func bufioLineScanner(r io.Reader) *bufio.Scanner {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64<<10), maxJSONLineBytes)
	return sc
}
