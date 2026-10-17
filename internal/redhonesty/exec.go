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
	"regexp"
	"strings"
	"time"
)

// Resource budgets. Every read, extraction and process output is bounded so a
// hostile repository or runaway test cannot exhaust memory or disk.
const (
	maxGitOutputBytes  = 8 << 20   // listing-style git output
	maxSourceBytes     = 4 << 20   // one source/test file
	maxChangedFiles    = 5000      // files considered in one change
	maxJSONLineBytes   = 1 << 20   // one `go test -json` line
	maxTestOutputBytes = 64 << 20  // total `go test -json` output
	maxArchiveBytes    = 2 << 30   // total extracted baseline bytes
	maxArchiveFileSize = 512 << 20 // one extracted file
	maxArchiveEntries  = 200000    // extracted entries
	maxStderrBytes     = 4 << 10   // retained stderr for diagnostics
	defaultTestTimeout = 10 * time.Minute
)

var testNamePattern = regexp.MustCompile(`^Test[\p{L}\p{N}_]*$`)

// limitedBuffer collects at most max bytes and then fails the write.
type limitedBuffer struct {
	buf bytes.Buffer
	max int
}

var errOutputTooLarge = errors.New("redhonesty: command output exceeds limit")

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.buf.Len()+len(p) > b.max {
		return 0, errOutputTooLarge
	}
	return b.buf.Write(p)
}

// tailBuffer keeps only the first max bytes and silently discards the rest;
// it never fails the writer. Used for diagnostic stderr.
type tailBuffer struct {
	buf bytes.Buffer
	max int
}

func (b *tailBuffer) Write(p []byte) (int, error) {
	if room := b.max - b.buf.Len(); room > 0 {
		if len(p) < room {
			room = len(p)
		}
		b.buf.Write(p[:room])
	}
	return len(p), nil
}

func (b *tailBuffer) String() string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 && r != '\n' && r != '\t' {
			return ' '
		}
		return r
	}, b.buf.String())
}

// ExecGit is the production Git implementation backed by the git binary.
type ExecGit struct{}

// Run implements Git. Arguments are passed as discrete argv entries; nothing is
// interpolated into a shell string.
func (ExecGit) Run(ctx context.Context, dir string, stdout io.Writer, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Stdout = stdout
	stderr := &tailBuffer{max: maxStderrBytes}
	cmd.Stderr = stderr
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0")
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
// -json` with an anchored -run selector and a bounded timeout.
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
		if !testNamePattern.MatchString(n) {
			return nil, fmt.Errorf("redhonesty: unsafe test name %q", n)
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
	args := []string{"test", "-json", "-count=1", "-vet=off", "-run", "^(" + strings.Join(names, "|") + ")$", target}
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = dir
	stderr := &tailBuffer{max: maxStderrBytes}
	cmd.Stderr = stderr
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("redhonesty: go test pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("redhonesty: start go test: %w", err)
	}
	res, parseErr := parseGoTestJSON(out)
	if parseErr != nil {
		// Stop the child before reaping so a flood cannot keep running.
		cancel()
	}
	waitErr := cmd.Wait()
	if parseErr != nil {
		return nil, parseErr
	}
	if ctx.Err() != nil {
		return nil, fmt.Errorf("redhonesty: go test %s: %w", pkg, ctx.Err())
	}
	var exitErr *exec.ExitError
	if waitErr != nil && !errors.As(waitErr, &exitErr) {
		return nil, fmt.Errorf("redhonesty: go test %s: %w: %s", pkg, waitErr, strings.TrimSpace(stderr.String()))
	}
	// A non-zero exit is the normal signal of a failing or unbuildable package;
	// the parsed events (or their absence) carry the per-test truth.
	return res, nil
}

// bufioLineScanner builds a scanner whose single-line limit is maxJSONLineBytes.
func bufioLineScanner(r io.Reader) *bufio.Scanner {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64<<10), maxJSONLineBytes)
	return sc
}
