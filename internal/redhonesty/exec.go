package redhonesty

import (
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
	maxTreeListBytes   = 64 << 20  // full `git ls-tree -r` of the base commit
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
	gitTimeout         = 10 * time.Minute // wall-clock bound for any one git invocation
	processWaitDelay   = 5 * time.Second
)

var errOutputTooLarge = errors.New("redhonesty: command output exceeds limit")

// limitedBuffer collects at most max bytes. Past the limit it keeps accepting
// (and discarding) writes and records the overflow, so the child never blocks on
// a full pipe; callers check exceeded after the process ends.
type limitedBuffer struct {
	buf      bytes.Buffer
	max      int
	exceeded bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.exceeded || b.buf.Len()+len(p) > b.max {
		b.exceeded = true
		return len(p), nil
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

var secretEnvMarkers = []string{
	"TOKEN", "SECRET", "PASSWORD", "PASSWD", "CREDENTIAL", "APIKEY", "API_KEY", "PRIVATE_KEY",
	"ACCESS_KEY", "AUTH", "DSN", "DATABASE_URL", "_PAT",
}

// goTestEnv returns the environment for `go test`: the caller's environment
// without likely credentials or git redirection variables (GIT_DIR and friends
// would point repository tests at the live repository instead of the exported
// baseline), with the toolchain pinned to the local one, the workspace file and
// go env file disabled and any ambient GOFLAGS/GOCACHEPROG dropped so the
// measurement is reproducible. This limits accidental exposure; it is not a
// sandbox (see the package trust-model note).
func goTestEnv() []string {
	env := make([]string, 0, 64)
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		upper := strings.ToUpper(name)
		switch upper {
		case "GOFLAGS", "GOTOOLCHAIN", "GOWORK", "GOENV", "GOCACHEPROG":
			continue
		}
		if strings.HasPrefix(upper, "GIT_") {
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
	return append(env, "GOTOOLCHAIN=local", "GOWORK=off", "GOFLAGS=", "GOENV=off")
}

// ExecGit is the production Git implementation backed by the git binary.
type ExecGit struct{}

// Run implements Git. Arguments are passed as discrete argv entries; nothing is
// interpolated into a shell string.
func (ExecGit) Run(ctx context.Context, dir string, stdout io.Writer, args ...string) error {
	ctx, cancel := context.WithTimeout(ctx, gitTimeout)
	defer cancel()
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
	if lb.exceeded {
		return nil, errOutputTooLarge
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
	if out.exceeded {
		return nil, fmt.Errorf("redhonesty: go test %s: %w", pkg, errOutputTooLarge)
	}
	// go test's own timeout fires first (see above) and test2json then reports
	// the hung test as a failure, which would look like an honest red. A timed-out
	// run is an abort, never a result.
	if bytes.Contains(out.buf.Bytes(), []byte("test timed out after")) {
		return nil, fmt.Errorf("redhonesty: go test %s: test run timed out", pkg)
	}
	var exitErr *exec.ExitError
	failedExit := errors.As(waitErr, &exitErr)
	if waitErr != nil && !failedExit {
		return nil, fmt.Errorf("redhonesty: go test %s: %w: %s", pkg, waitErr, strings.TrimSpace(stderr.String()))
	}
	res, err := parseGoTestJSON(bytes.NewReader(out.buf.Bytes()))
	if err != nil {
		return nil, err
	}
	return reconcileExit(res, failedExit), nil
}

// reconcileExit applies the process exit status to the parsed results. A
// non-zero exit is the normal signal of a failing or unbuildable package, and a
// reported test failure explains it (those results are kept as observed). A
// non-zero exit with NO reported test failure (a TestMain that exits non-zero
// after m.Run, a panic outside any test, a build error) means the run as a whole
// failed for a reason the per-test events do not capture, so any "pass" it
// printed is not trustworthy evidence and becomes unrunnable.
func reconcileExit(res map[string]Result, failedExit bool) map[string]Result {
	if !failedExit {
		return res
	}
	for _, r := range res {
		if r == ResultFail {
			return res
		}
	}
	for n, r := range res {
		if r == ResultPass {
			res[n] = ResultUnrunnable
		}
	}
	return res
}
