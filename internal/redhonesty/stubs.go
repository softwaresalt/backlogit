package redhonesty

// STRUCTURAL STUBS for the red phase (P-002 / P-004). Replaced in the green
// commit by the real implementation.

import (
	"context"
	"errors"
	"io"

	"github.com/softwaresalt/backlogit/internal/faultline"
)

var errNotImplemented = errors.New("redhonesty: not implemented")

// ErrNoNewTests is returned when a change adds no regression tests.
var ErrNoNewTests = errors.New("redhonesty: no newly added tests")

// ExecGit is the production Git implementation.
type ExecGit struct{}

// Run implements Git.
func (ExecGit) Run(context.Context, string, io.Writer, ...string) error { return errNotImplemented }

// GoTestExecutor is the production TestExecutor.
type GoTestExecutor struct{}

// Run implements TestExecutor.
func (GoTestExecutor) Run(context.Context, string, string, []string) (map[string]Result, error) {
	return nil, errNotImplemented
}

// IdentifyNewTests returns the newly added tests.
func IdentifyNewTests([]FileChange) ([]NewTest, error) { return nil, errNotImplemented }

// ChangedSymbols returns the production-symbol delta.
func ChangedSymbols([]FileChange) (Symbols, error) { return Symbols{}, errNotImplemented }

// ResolveCommit resolves ref to an immutable commit SHA.
func ResolveCommit(context.Context, Git, string, string) (string, error) {
	return "", errNotImplemented
}

// CollectChanges lists the changed files between two commits.
func CollectChanges(context.Context, Git, string, string, string) ([]FileChange, error) {
	return nil, errNotImplemented
}

func extractArchive(io.Reader, string) error { return errNotImplemented }

func applyOverlay(string, []FileChange) error { return errNotImplemented }

// PrepareBaseline materializes the baseline tree plus the test overlay.
func PrepareBaseline(context.Context, Git, string, string, []FileChange) (string, func() error, error) {
	return "", nil, errNotImplemented
}

func parseGoTestJSON(io.Reader) (map[string]Result, error) { return nil, errNotImplemented }

// RunTests runs tests grouped by package and returns results keyed by Key().
func RunTests(context.Context, TestExecutor, string, []NewTest) (map[string]Result, error) {
	return nil, errNotImplemented
}

// Evaluate classifies each new test.
func Evaluate(EvalInput) []Verdict { return nil }

// BuildArtifact builds the shared-contract evidence artifact.
func BuildArtifact(string, string, string, []Verdict) (faultline.EvidenceArtifact, error) {
	return faultline.EvidenceArtifact{}, errNotImplemented
}

// Store persists evidence per task and commit.
type Store struct{}

// NewStore creates a Store rooted at dir.
func NewStore(string) (*Store, error) { return nil, errNotImplemented }

// Write persists a validated artifact.
func (*Store) Write(faultline.EvidenceArtifact) (string, error) { return "", errNotImplemented }

// Load retrieves the artifact for a task and commit.
func (*Store) Load(string, string) (faultline.EvidenceArtifact, error) {
	return faultline.EvidenceArtifact{}, errNotImplemented
}

// Request is a gate invocation.
type Request struct {
	TaskID  string
	BaseRef string
	HeadRef string
}

// Report is the gate result.
type Report struct {
	BaseSHA  string
	HeadSHA  string
	Verdicts []Verdict
	Artifact faultline.EvidenceArtifact
	Honest   bool
}

// Gate orchestrates the honesty check.
type Gate struct {
	Git   Git
	Exec  TestExecutor
	Root  string
	Store *Store
}

// Check runs the gate.
func (*Gate) Check(context.Context, Request) (Report, error) { return Report{}, errNotImplemented }

const maxJSONLineBytes = 1 << 20
