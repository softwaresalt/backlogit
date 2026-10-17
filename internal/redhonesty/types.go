// Package redhonesty implements the S9 red-test honesty gate (fault-line
// sequence 6/7).
//
// A claimed regression ("red") test is only honest when it demonstrably fails
// against the pre-fix baseline and passes at head. The gate:
//
//   - U1 identifies newly added top-level tests between an immutable base SHA
//     (which must be an ancestor of head) and head and runs them against
//     baseline production code (base tree plus ONLY the head test-file overlay)
//     inside a workspace-contained, git-ignored directory;
//   - U2 rejects dishonest reds (already green at baseline, not referencing the
//     changed code, asserting only sibling fields, or deriving the expected value
//     from the changed production function) and fails closed when the baseline
//     cannot be verified;
//   - U3 records the observed red/green outcome as a RedGreenEvidence artifact
//     conforming to the shared internal/faultline evidence contract (S9 is a
//     producer, not the schema owner).
//
// Honest limits. The behavioural rules (baseline result, head result) are exact.
// The structural rules are NAME-BASED SYNTACTIC HEURISTICS over the test's AST:
// they match bare identifier and selector names with no type resolution, only
// follow helper functions in the same file (depth-bounded), and only recognise an
// expected value bound to a variable/field named want/expected/exp/golden. They
// reduce, but do not eliminate, false greens; false rejections are possible too.
// Only NEW top-level Test functions are gated: cases added as subtests or table
// rows inside an existing test are not. A test that cannot build at the baseline
// (for example it references a symbol the fix introduces) is unverifiable and is
// rejected; declaration-adding work is covered by source-shape harnesses instead.
//
// Trust model. The gate runs the repository's own test code, exactly as CI does;
// it is not a sandbox. It strips likely credentials from the child environment,
// pins the local toolchain and bounds time and output, but a malicious test can
// still act as the invoking user, so run it only on code you would run in CI.
// Evidence integrity is a consumer obligation (S4 accept path: authenticate the
// Canonical() bytes, check the producer, anti-replay); stored evidence here is
// shape-validated, bound to task and commit, and not silently replaceable, but
// not signed.
//
// All external effects (git, go test) sit behind small interfaces and every
// read, extraction and output path is bounded.
package redhonesty

import (
	"context"
	"io"
)

// Result is the observed outcome of one test run.
type Result string

// Result values. ResultUnrunnable covers build failures, tests the run did not
// report, and skips: anything that is not a verifiable pass or assertion
// failure. (A run that times out aborts with an error instead.)
const (
	ResultPass       Result = "pass"
	ResultFail       Result = "fail"
	ResultUnrunnable Result = "unrunnable"
)

// FileChange is one changed file between base and head. Base is nil for an
// added file and Head is nil for a deleted file. Path is slash-separated and
// relative to the repository root.
type FileChange struct {
	Path   string
	Status byte // 'A', 'M' or 'D'
	Base   []byte
	Head   []byte
}

// NewTest identifies one newly added test function.
type NewTest struct {
	// Package is the slash-separated package directory relative to the
	// repository root ("." for the root package).
	Package string
	// File is the slash-separated repository-relative test file path.
	File string
	// Name is the top-level test function name.
	Name string
}

// Key is the stable identity of a test across baseline and head runs.
func (t NewTest) Key() string { return t.Package + "::" + t.Name }

// Symbols is the production-symbol delta between base and head. Sets hold bare
// identifier names.
type Symbols struct {
	// Funcs are functions and methods that were added or whose text changed.
	Funcs map[string]struct{}
	// Types are named types that were added or whose declaration changed.
	Types map[string]struct{}
	// Fields are struct fields that were added or whose type/tag changed.
	Fields map[string]struct{}
	// Siblings are the unchanged fields of structs that have a changed field.
	Siblings map[string]struct{}
}

// Empty reports whether no production symbol changed.
func (s Symbols) Empty() bool {
	return len(s.Funcs)+len(s.Types)+len(s.Fields) == 0
}

// Git runs one git command with stdout streamed to the supplied writer.
type Git interface {
	Run(ctx context.Context, dir string, stdout io.Writer, args ...string) error
}

// TestExecutor runs the named top-level tests of one package inside dir and
// returns the observed result for every test it could observe. Tests it could
// not observe are simply absent from the map.
type TestExecutor interface {
	Run(ctx context.Context, dir, pkg string, names []string) (map[string]Result, error)
}

// RejectionClass names one dishonest-red (or unverifiable) classification.
type RejectionClass string

// Rejection classes.
const (
	// RejectAlreadyPasses: the test passes against the baseline.
	RejectAlreadyPasses RejectionClass = "already_passes_at_baseline"
	// RejectNoChangedBranch: the test (and same-file helpers) mention no changed
	// production identifier (name-based heuristic, no type resolution).
	RejectNoChangedBranch RejectionClass = "does_not_exercise_changed_code"
	// RejectSiblingOnly: the test selects only unchanged sibling fields of a
	// changed struct, never a changed field (name-based heuristic).
	RejectSiblingOnly RejectionClass = "asserts_only_sibling_fields"
	// RejectBuggyPathExpected: an expected-value binding (want/expected/exp/golden)
	// is computed by calling a changed production function (name-based heuristic).
	RejectBuggyPathExpected RejectionClass = "expected_output_via_changed_path"
	// RejectUnverifiableBaseline: the baseline run produced no verifiable result.
	RejectUnverifiableBaseline RejectionClass = "baseline_unverifiable"
	// RejectHeadNotGreen: the test does not pass at head.
	RejectHeadNotGreen RejectionClass = "head_not_green"
	// RejectUnanalyzable: the test source could not be located or parsed.
	RejectUnanalyzable RejectionClass = "test_source_unanalyzable"
)

// Verdict is the honesty decision for one new test.
type Verdict struct {
	Test       NewTest
	Baseline   Result
	Head       Result
	Rejections []RejectionClass
}

// Honest reports whether the test is an honest red/green reproducer.
func (v Verdict) Honest() bool { return len(v.Rejections) == 0 }

// EvalInput carries everything Evaluate needs; it performs no I/O.
type EvalInput struct {
	Tests []NewTest
	// Sources maps repository-relative test file path to its head source.
	Sources map[string][]byte
	// Baseline and Head map NewTest.Key() to the observed result.
	Baseline map[string]Result
	Head     map[string]Result
	Symbols  Symbols
}
