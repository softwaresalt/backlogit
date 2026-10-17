package redhonesty

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/softwaresalt/backlogit/internal/faultline"
)

// Request is one gate invocation.
type Request struct {
	// TaskID is the backlog task the evidence is produced for.
	TaskID string
	// BaseRef is the pre-fix/base ref; it is resolved to an immutable SHA.
	BaseRef string
	// HeadRef is the change under review; empty means HEAD. It is resolved to an
	// immutable SHA and exported like the baseline, so the working tree (dirty,
	// untracked or git-ignored files included) never influences the result.
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
// immutable baseline plus the test overlay, run them against an immutable export
// of head, classify, and record the evidence.
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
	// The base must be an ancestor of head: an arbitrary (older) base could make
	// any test look red because the code it depends on did not exist yet.
	if err := g.Git.Run(ctx, g.Root, io.Discard, "merge-base", "--is-ancestor", baseSHA, headSHA); err != nil {
		return rep, fmt.Errorf("redhonesty: base %s is not an ancestor of head %s: %w", baseSHA[:12], headSHA[:12], err)
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
	if len(tests) > faultline.MaxArrayElements {
		return rep, fmt.Errorf("redhonesty: %d new tests exceed the evidence limit of %d; split the change", len(tests), faultline.MaxArrayElements)
	}
	symbols, err := ChangedSymbols(changes)
	if err != nil {
		return rep, err
	}

	baseline, err := g.runIn(ctx, baseSHA, changes, tests)
	if err != nil {
		return rep, err
	}
	// Head is measured from an immutable export of headSHA with NO overlay, so
	// only committed inputs can influence the result.
	head, err := g.runIn(ctx, headSHA, nil, tests)
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

// runIn exports sha (plus an optional test overlay) into a fresh ignored
// directory, runs the tests there and always removes the directory.
func (g *Gate) runIn(ctx context.Context, sha string, overlay []FileChange, tests []NewTest) (map[string]Result, error) {
	dir, cleanup, err := PrepareBaseline(ctx, g.Git, g.Root, sha, overlay)
	if err != nil {
		return nil, err
	}
	res, runErr := RunTests(ctx, g.Exec, dir, tests)
	if cerr := cleanup(); cerr != nil {
		runErr = errors.Join(runErr, fmt.Errorf("redhonesty: cleanup %s: %w", sha[:12], cerr))
	}
	if runErr != nil {
		return nil, runErr
	}
	return res, nil
}
