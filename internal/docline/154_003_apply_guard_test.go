package docline

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 154.003-T (U2b) harness: ApplyMigration must reject a plan that carries
// Findings with ErrPlanHasFindings and perform ZERO writes (corpus
// all-or-nothing preserved). policy.go must declare ErrPlanHasFindings.
//
// RED until:
//   - policy.go adds ErrPlanHasFindings exported sentinel error, AND
//   - service.go's ApplyMigration checks len(plan.Findings)>0 at its top
//     and returns ErrPlanHasFindings with zero writes.

// applyGuardPolicyPath returns the absolute path to policy.go so the
// source-shape test can parse it without relying on the working directory.
func applyGuardPolicyPath(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok, "runtime.Caller unavailable")
	return filepath.Join(filepath.Dir(file), "policy.go")
}

// TestU2bApplyGuard_ErrPlanHasFindingsDeclared is the source-shape assertion
// for the ErrPlanHasFindings sentinel: it parses policy.go and checks for a
// top-level var declaration with that name. Compiles without the sentinel
// existing; fails RED until it is added to policy.go.
func TestU2bApplyGuard_ErrPlanHasFindingsDeclared(t *testing.T) {
	t.Parallel()

	policyPath := applyGuardPolicyPath(t)
	src, err := os.ReadFile(policyPath) //nolint:gosec // known test-file path
	require.NoError(t, err, "policy.go must be readable")

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, policyPath, src, 0)
	require.NoError(t, err, "policy.go must parse as valid Go")

	var found bool
	for _, decl := range f.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.VAR {
			continue
		}
		for _, spec := range genDecl.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for _, name := range vs.Names {
				if name.Name == "ErrPlanHasFindings" {
					found = true
				}
			}
		}
	}
	assert.True(t, found,
		"policy.go must declare an exported var ErrPlanHasFindings; add it as a sentinel error")
}

// TestU2bApplyGuard_FindingsBearingPlanReturnsError verifies that
// ApplyMigration returns a non-nil error when the plan carries one or more
// Findings. Pre-U2b, ApplyMigration ignores Findings and returns nil →
// assertion fails RED.
func TestU2bApplyGuard_FindingsBearingPlanReturnsError(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	// Construct a plan that carries a Findings entry (U2a field now exists).
	// The Changes list is empty so there is nothing to write — the guard must
	// still reject and return an error before reaching the write loop.
	plan := MigrationPlan{
		Changes: []Change{},
		Findings: []Finding{
			{
				File:     "docs/decisions/broken.md",
				Rule:     RuleDecodeError,
				Severity: SeverityError,
				Fix:      "malformed frontmatter",
			},
		},
	}

	_, err := ApplyMigration(plan, Options{Root: root, Now: time.Now().UTC()})
	require.Error(t, err,
		"ApplyMigration must return an error when plan.Findings is non-empty (ErrPlanHasFindings guard)")
}

// TestU2bApplyGuard_FindingsBearingPlanWritesZeroFiles verifies the
// all-or-nothing write invariant: a plan carrying Findings must cause
// ApplyMigration to write ZERO files, even when plan.Changes is non-empty.
// Pre-U2b, ApplyMigration would apply the changes → file count > 0 → RED.
func TestU2bApplyGuard_FindingsBearingPlanWritesZeroFiles(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	// Write a real file that the plan would migrate.
	require.NoError(t, os.MkdirAll(filepath.Join(root, "docs", "decisions"), 0o755))
	const before = "---\ntitle: T\n---\nBody.\n"
	target := filepath.Join(root, "docs", "decisions", "x.md")
	require.NoError(t, os.WriteFile(target, []byte(before), 0o644))

	// Construct a plan with both a Change and a Finding.
	normalized, err := Normalize("docs/decisions/x.md", []byte(before), NormalizeOptions{Now: time.Now().UTC()})
	require.NoError(t, err)

	plan := MigrationPlan{
		Changes: []Change{
			{
				File:   "docs/decisions/x.md",
				Action: ActionUpdate,
				Before: before,
				After:  string(normalized),
			},
		},
		Findings: []Finding{
			{
				File:     "docs/decisions/broken.md",
				Rule:     RuleDecodeError,
				Severity: SeverityError,
				Fix:      "malformed frontmatter",
			},
		},
	}

	_, err = ApplyMigration(plan, Options{Root: root, Now: time.Now().UTC()})
	// Verify the error is returned (guard fired) AND the target is unmodified.
	require.Error(t, err,
		"ApplyMigration must return ErrPlanHasFindings guard error")

	actual, readErr := os.ReadFile(target)
	require.NoError(t, readErr)
	assert.Equal(t, before, string(actual),
		"ApplyMigration must write ZERO files when plan carries Findings (corpus all-or-nothing)")
}

// TestU2bApplyGuard_EmptyFindingsAppliesNormally verifies the complement: a
// plan with NO Findings applies normally (no regression). This must stay
// GREEN throughout — before and after U2b.
func TestU2bApplyGuard_EmptyFindingsAppliesNormally(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "docs", "decisions"), 0o755))

	const before = "---\ntitle: T\n---\nBody.\n"
	target := filepath.Join(root, "docs", "decisions", "y.md")
	require.NoError(t, os.WriteFile(target, []byte(before), 0o644))

	normalized, err := Normalize("docs/decisions/y.md", []byte(before), NormalizeOptions{Now: time.Now().UTC()})
	require.NoError(t, err)

	plan := MigrationPlan{
		Changes: []Change{
			{
				File:   "docs/decisions/y.md",
				Action: ActionUpdate,
				Before: before,
				After:  string(normalized),
			},
		},
		// Findings is nil/empty — guard must NOT fire.
	}

	res, err := ApplyMigration(plan, Options{Root: root, Now: time.Now().UTC()})
	require.NoError(t, err, "ApplyMigration must not error when plan.Findings is empty")
	assert.Contains(t, res.Applied, "docs/decisions/y.md",
		"ApplyMigration must apply the change when Findings is empty")
}
