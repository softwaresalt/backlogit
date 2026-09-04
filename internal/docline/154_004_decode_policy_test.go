package docline

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 154.004-T (U2c) harness: PlanMigration must report-and-continue on a
// frontmatter decode failure using the single shared classifyDecodeFailure /
// applyDecodeFailure policy. normalize.go must wrap the decode error with
// ErrFrontmatterDecode so classifyDecodeFailure can classify it correctly.
//
// RED until:
//   - normalize.go adds the two-%w ErrFrontmatterDecode discriminator to
//     Normalize's decode-error wrap, AND
//   - service.go's PlanMigration calls applyDecodeFailure(err, rel) on a
//     Normalize error, appends findings, and continues (rather than aborting).

// newDecodeCorpusRoot creates a temp dir with a malformed file (broken.md),
// a valid but unnormalized file (valid.md), and a memory dir that must be
// skipped. Returns the root.
func newDecodeCorpusRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	// A file whose YAML frontmatter cannot be decoded (unclosed sequence).
	require.NoError(t, os.MkdirAll(filepath.Join(root, "docs", "decisions"), 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(root, "docs", "decisions", "broken.md"),
		[]byte("---\ntitle: [unclosed yaml\n---\nBody.\n"), 0o644))

	// A valid (but unnormalized) file alongside — should be included in Changes.
	require.NoError(t, os.WriteFile(
		filepath.Join(root, "docs", "decisions", "valid.md"),
		[]byte("---\ntitle: Valid\n---\nBody.\n"), 0o644))

	// Out-of-scope memory file — must never appear in Changes or Findings.
	require.NoError(t, os.MkdirAll(filepath.Join(root, "docs", "memory"), 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(root, "docs", "memory", "skip.md"),
		[]byte("---\ntitle: Skip\n---\n"), 0o644))

	return root
}

// TestU2cNormalize_DecodeErrorWrapsErrFrontmatterDecode asserts that when
// Normalize encounters a frontmatter decode failure, the returned error wraps
// ErrFrontmatterDecode (enabling classifyDecodeFailure to classify it as
// decodeFailureFrontmatter). RED until normalize.go adds the two-%w pattern.
func TestU2cNormalize_DecodeErrorWrapsErrFrontmatterDecode(t *testing.T) {
	t.Parallel()

	raw := []byte("---\ntitle: [unclosed yaml\n---\nBody.\n")
	_, err := Normalize("docs/decisions/broken.md", raw, NormalizeOptions{Now: time.Now().UTC()})
	require.Error(t, err, "Normalize must return an error for malformed frontmatter")
	assert.True(t, errors.Is(err, ErrFrontmatterDecode),
		"Normalize must wrap ErrFrontmatterDecode in its decode-error return so "+
			"classifyDecodeFailure classifies it as decodeFailureFrontmatter; "+
			"add the two-%%w pattern: fmt.Errorf(...%%w: %%w, ErrFrontmatterDecode, err)")
}

// TestU2cPlanMigration_ReportsContinuesPastDecodeError asserts that
// PlanMigration does NOT abort when a file has a frontmatter decode error:
// it emits a decode_error Finding for that file and continues with the rest
// of the corpus. RED until service.go calls applyDecodeFailure on Normalize
// errors and continues instead of returning a fatal error.
func TestU2cPlanMigration_ReportsContinuesPastDecodeError(t *testing.T) {
	t.Parallel()

	root := newDecodeCorpusRoot(t)
	plan, err := PlanMigration(Options{Root: root, Now: time.Now().UTC()})
	require.NoError(t, err,
		"PlanMigration must not return a fatal error for a frontmatter decode failure; "+
			"it must report-and-continue")

	// plan.Findings must contain exactly one decode_error for broken.md.
	require.Len(t, plan.Findings, 1,
		"PlanMigration must emit exactly one Finding for the malformed file")
	f := plan.Findings[0]
	assert.Equal(t, "docs/decisions/broken.md", f.File)
	assert.Equal(t, RuleDecodeError, f.Rule)
	assert.Equal(t, SeverityError, f.Severity)
	assert.True(t, errors.Is(
		// The Finding.Fix contains the error string; wrap it to check ErrFrontmatterDecode.
		fmt.Errorf("%w", ErrFrontmatterDecode),
		ErrFrontmatterDecode),
		"sanity check — the above errors.Is call format is valid")
	// Fix must contain ErrFrontmatterDecode's error message, not raw body bytes.
	assert.Contains(t, f.Fix, ErrFrontmatterDecode.Error(),
		"Finding.Fix must contain the ErrFrontmatterDecode error string")

	// plan.Changes must include valid.md (continued past broken.md).
	var changePaths []string
	for _, c := range plan.Changes {
		changePaths = append(changePaths, c.File)
	}
	assert.Contains(t, changePaths, "docs/decisions/valid.md",
		"PlanMigration must include valid files in Changes after continuing past decode error")
	assert.NotContains(t, changePaths, "docs/decisions/broken.md",
		"PlanMigration must NOT include the malformed file in Changes")
}

// TestU2cPlanMigration_ContainmentStaysFatal asserts that a containment error
// in PlanMigration (ErrPathEscapesWorkspace from SafeResolve) still returns a
// fatal error and an empty plan. This must stay GREEN throughout — before and
// after U2c.
func TestU2cPlanMigration_ContainmentStaysFatal(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	// Pass a path that escapes the root (".." from within the root).
	_, err := PlanMigration(Options{Root: root, Path: "../escape", Now: time.Now().UTC()})
	require.Error(t, err, "PlanMigration must return an error for an escaping path")
	assert.True(t, errors.Is(err, ErrPathEscapesWorkspace),
		"a containment error in PlanMigration must remain fatal (not become a finding)")
}

// TestU2cFindingStructurallyEquivalentToLintTree asserts that PlanMigration's
// Finding for a malformed file is STRUCTURALLY EQUIVALENT to LintTree's:
// same File, same Rule (RuleDecodeError), same Severity (SeverityError), and
// Fix containing both ErrFrontmatterDecode.Error() and the underlying YAML
// cause. Byte-equality is NOT required (prefix may differ).
func TestU2cFindingStructurallyEquivalentToLintTree(t *testing.T) {
	t.Parallel()

	root := newDecodeCorpusRoot(t)

	lintFindings, err := LintTree(Options{Root: root})
	require.NoError(t, err)

	var lintDecodeErr *Finding
	for i := range lintFindings {
		if lintFindings[i].File == "docs/decisions/broken.md" &&
			lintFindings[i].Rule == RuleDecodeError {
			lintDecodeErr = &lintFindings[i]
			break
		}
	}
	require.NotNil(t, lintDecodeErr,
		"LintTree must emit a decode_error Finding for broken.md (regression guard)")

	plan, err := PlanMigration(Options{Root: root, Now: time.Now().UTC()})
	require.NoError(t, err)
	require.Len(t, plan.Findings, 1)
	planFinding := plan.Findings[0]

	// Structural equivalence (not byte equality).
	assert.Equal(t, lintDecodeErr.File, planFinding.File,
		"Finding.File must match LintTree's")
	assert.Equal(t, lintDecodeErr.Rule, planFinding.Rule,
		"Finding.Rule must match LintTree's (both must be RuleDecodeError)")
	assert.Equal(t, lintDecodeErr.Severity, planFinding.Severity,
		"Finding.Severity must match LintTree's")
	assert.Contains(t, planFinding.Fix, ErrFrontmatterDecode.Error(),
		"PlanMigration Finding.Fix must contain ErrFrontmatterDecode.Error()")
	// The Fix must NOT carry raw body bytes (only frontmatter parser message).
	assert.NotContains(t, planFinding.Fix, "unclosed",
		"Finding.Fix must not carry the raw YAML body bytes beyond the parser message")

	// LintTree regression: its Finding.Fix must not have changed.
	assert.Equal(t, lintDecodeErr.Fix, lintDecodeErr.Fix,
		"LintTree's own Finding.Fix must not change (regression guard; byte-equal)")
}

// TestU2cEndToEnd_MixedCorpus_ApplyRejectedViaErrPlanHasFindings is the
// end-to-end integration test: a real malformed file in a mixed corpus causes
// PlanMigration to report-and-continue, and a subsequent apply is rejected by
// the U2b ErrPlanHasFindings guard with zero writes.
func TestU2cEndToEnd_MixedCorpus_ApplyRejectedViaErrPlanHasFindings(t *testing.T) {
	t.Parallel()

	root := newDecodeCorpusRoot(t)
	opts := Options{Root: root, Now: time.Now().UTC()}

	// PlanMigration must report-and-continue.
	plan, err := PlanMigration(opts)
	require.NoError(t, err,
		"PlanMigration must not abort on a mixed corpus with a malformed file")
	require.NotEmpty(t, plan.Findings,
		"PlanMigration must populate Findings for the malformed file")
	assert.Equal(t, RuleDecodeError, plan.Findings[0].Rule)

	// Record the pre-apply state of valid.md.
	validPath := filepath.Join(root, "docs", "decisions", "valid.md")
	before, err := os.ReadFile(validPath)
	require.NoError(t, err)

	// ApplyMigration must be rejected by the ErrPlanHasFindings guard.
	_, applyErr := ApplyMigration(plan, opts)
	require.Error(t, applyErr,
		"ApplyMigration must return ErrPlanHasFindings for a plan with Findings")
	assert.True(t, errors.Is(applyErr, ErrPlanHasFindings),
		"ApplyMigration must return ErrPlanHasFindings (U2b guard)")

	// ZERO writes: valid.md must be unchanged.
	after, readErr := os.ReadFile(validPath)
	require.NoError(t, readErr)
	assert.Equal(t, string(before), string(after),
		"ApplyMigration must write ZERO files when plan carries Findings (corpus all-or-nothing)")

	// MigrateReport carries the Finding.
	report := NewMigrateReport(plan, nil, true)
	raw, marshalErr := json.Marshal(report)
	require.NoError(t, marshalErr)
	var decoded map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &decoded))
	findingsRaw, ok := decoded["findings"]
	require.True(t, ok, "MigrateReport JSON must carry a findings key")
	assert.NotEqual(t, "[]", string(findingsRaw),
		"findings must be non-empty for a corpus with a decode error")
}

// TestU2cLintTreeRegression pins that LintTree's decode_error Finding.Fix
// does NOT change as a result of the U2c changes (which add ErrFrontmatterDecode
// to Normalize but must not affect decodeDoc's existing two-%w pattern).
func TestU2cLintTreeRegression(t *testing.T) {
	t.Parallel()

	root := newDecodeCorpusRoot(t)
	findings, err := LintTree(Options{Root: root})
	require.NoError(t, err)

	var decodeErr *Finding
	for i := range findings {
		if findings[i].File == "docs/decisions/broken.md" &&
			findings[i].Rule == RuleDecodeError {
			decodeErr = &findings[i]
			break
		}
	}
	require.NotNil(t, decodeErr, "LintTree must emit a decode_error Finding for broken.md")

	// The Fix must still contain ErrFrontmatterDecode — unchanged from before U2c.
	assert.Contains(t, decodeErr.Fix, ErrFrontmatterDecode.Error(),
		"LintTree's decode_error Finding.Fix must still contain ErrFrontmatterDecode "+
			"after U2c's changes to normalize.go")
}
