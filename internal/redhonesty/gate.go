package redhonesty

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/softwaresalt/backlogit/internal/faultline"
)

// Request is one gate invocation.
type Request struct {
	// TaskID is the backlog task the evidence is produced for.
	TaskID string
	// BaseRef is the pre-fix/base ref; it is resolved to an immutable SHA.
	BaseRef string
	// HeadRef is the change under review; empty means HEAD. The working tree
	// must be that commit.
	HeadRef string
}

// Report is the gate result.
type Report struct {
	BaseSHA  string
	HeadSHA  string
	Verdicts []Verdict
	Artifact faultline.EvidenceArtifact
	// Honest is true only when every new test is an honest red/green reproducer.
	Honest bool
	// EvidencePath is where the artifact was persisted ("" without a Store).
	EvidencePath string
}

// Gate orchestrates the honesty check: identify new tests, run them against the
// immutable baseline plus the test overlay, run them at head, classify, and
// record the evidence.
type Gate struct {
	Git   Git
	Exec  TestExecutor
	Root  string
	Store *Store
}

// Check runs the gate. It returns ErrNoNewTests when the change adds no tests.
func (g *Gate) Check(ctx context.Context, req Request) (Report, error) {
	var rep Report
	if g == nil || g.Git == nil || g.Exec == nil || g.Root == "" {
		return rep, errors.New("redhonesty: gate is not configured")
	}
	if !taskIDPattern.MatchString(req.TaskID) {
		return rep, fmt.Errorf("redhonesty: invalid task id %q", truncate(req.TaskID))
	}
	headRef := req.HeadRef
	if headRef == "" {
		headRef = "HEAD"
	}
	baseSHA, err := ResolveCommit(ctx, g.Git, g.Root, req.BaseRef)
	if err != nil {
		return rep, err
	}
	headSHA, err := ResolveCommit(ctx, g.Git, g.Root, headRef)
	if err != nil {
		return rep, err
	}
	if err := g.requireHeadCheckedOut(ctx, headSHA); err != nil {
		return rep, err
	}
	changes, err := CollectChanges(ctx, g.Git, g.Root, baseSHA, headSHA)
	if err != nil {
		return rep, err
	}
	tests, err := IdentifyNewTests(changes)
	if err != nil {
		return rep, err
	}
	if len(tests) == 0 {
		return rep, ErrNoNewTests
	}
	symbols, err := ChangedSymbols(changes)
	if err != nil {
		return rep, err
	}

	dir, cleanup, err := PrepareBaseline(ctx, g.Git, g.Root, baseSHA, changes)
	if err != nil {
		return rep, err
	}
	baseline, runErr := RunTests(ctx, g.Exec, dir, tests)
	if cerr := cleanup(); cerr != nil && runErr == nil {
		runErr = fmt.Errorf("redhonesty: cleanup baseline: %w", cerr)
	}
	if runErr != nil {
		return rep, runErr
	}
	head, err := RunTests(ctx, g.Exec, g.Root, tests)
	if err != nil {
		return rep, err
	}

	sources := map[string][]byte{}
	for _, c := range changes {
		if c.Head != nil && isTestFile(c.Path) {
			sources[c.Path] = c.Head
		}
	}
	rep.BaseSHA, rep.HeadSHA = baseSHA, headSHA
	rep.Verdicts = Evaluate(EvalInput{Tests: tests, Sources: sources, Baseline: baseline, Head: head, Symbols: symbols})
	rep.Artifact, err = BuildArtifact(req.TaskID, baseSHA, headSHA, rep.Verdicts)
	if err != nil {
		return rep, err
	}
	rep.Honest = rep.Artifact.Status == faultline.StatusPass
	if g.Store != nil {
		if rep.EvidencePath, err = g.Store.Write(rep.Artifact); err != nil {
			return rep, err
		}
	}
	return rep, nil
}

// requireHeadCheckedOut ensures the working tree is exactly headSHA for Go
// inputs, so the head run measures the code the evidence is bound to.
func (g *Gate) requireHeadCheckedOut(ctx context.Context, headSHA string) error {
	cur, err := ResolveCommit(ctx, g.Git, g.Root, "HEAD")
	if err != nil {
		return err
	}
	if cur != headSHA {
		return fmt.Errorf("redhonesty: head %s is not checked out (HEAD is %s)", headSHA[:12], cur[:12])
	}
	out, err := gitOutput(ctx, g.Git, g.Root, maxGitOutputBytes, "diff", "--name-only", "-z", "HEAD", "--")
	if err != nil {
		return fmt.Errorf("redhonesty: inspect working tree: %w", err)
	}
	for _, p := range strings.Split(strings.TrimRight(string(out), "\x00"), "\x00") {
		if p != "" && (isGoSource(p) || p == "go.mod" || p == "go.sum" || isOverlayPath(p)) {
			return fmt.Errorf("redhonesty: working tree has uncommitted Go changes (%s); head evidence would not match the commit", truncate(p))
		}
	}
	return nil
}
