package events

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	backlogiterrors "github.com/softwaresalt/backlogit/internal/errors"
)

// TestU2_UnknownTopLevelKeysRefused is a source-shape harness (147-F / U2,
// cycle-31): checkpoint_conformance.go does not yet exist, so parsing it
// fails with a file-not-found error rather than a Go build error — the test
// file itself references no undeclared identifier and compiles cleanly. The
// assertion below fails via a normal test failure (require.NoError) rather
// than a compiler error, satisfying P-004's red phase. No stub file is
// committed ahead of this harness.
func TestU2_UnknownTopLevelKeysRefused(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "checkpoint_conformance.go", nil, parser.AllErrors)
	require.NoError(t, err, "checkpoint_conformance.go is not declared yet")

	funcDecl := findPackageFuncDecl(file, "CheckConformingTopLevelNamespace")
	if !assert.NotNil(t, funcDecl, "CheckConformingTopLevelNamespace is not declared in checkpoint_conformance.go") {
		return
	}
	if assert.Len(t, funcDecl.Type.Params.List, 1, "CheckConformingTopLevelNamespace must take exactly one parameter") {
		paramType, ok := funcDecl.Type.Params.List[0].Type.(*ast.ArrayType)
		assert.True(t, ok && paramType.Len == nil, "CheckConformingTopLevelNamespace's parameter must be []byte")
	}
	assert.Len(t, funcDecl.Type.Results.List, 1, "CheckConformingTopLevelNamespace must return exactly one value (error)")
}

// findPackageFuncDecl returns the package-level (non-method) func with the
// given name, or nil if absent.
func findPackageFuncDecl(file *ast.File, name string) *ast.FuncDecl {
	if file == nil {
		return nil
	}
	for _, decl := range file.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok || funcDecl.Recv != nil {
			continue
		}
		if funcDecl.Name.Name == name {
			return funcDecl
		}
	}
	return nil
}

// TestU2Guard_ConformingDocumentAccepted asserts a conforming V1 document
// returns nil.
func TestU2Guard_ConformingDocumentAccepted(t *testing.T) {
	doc := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","status":"active"}`
	assert.NoError(t, CheckConformingTopLevelNamespace([]byte(doc)))
}

// TestU2Guard_TwoUnknownTopLevelKeysRefused asserts a document with two
// unknown top-level keys returns the typed error with both keys sorted.
func TestU2Guard_TwoUnknownTopLevelKeysRefused(t *testing.T) {
	doc := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","zeta_key":"x","alpha_key":"y"}`
	err := CheckConformingTopLevelNamespace([]byte(doc))
	require.Error(t, err)
	var typed *backlogiterrors.CheckpointNonConformingError
	require.True(t, errors.As(err, &typed))
	assert.Equal(t, []string{"alpha_key", "zeta_key"}, typed.Fields)
}

// TestU2Guard_ReservedDispositionKeysAdmitted asserts a document carrying
// all four disposition* reserved keys with status:"abandoned" returns nil,
// proving reserved-key admission and the deliberate absence of a
// reserved-status-value check at the read boundary (U2 is not U4's
// create-boundary gate).
func TestU2Guard_ReservedDispositionKeysAdmitted(t *testing.T) {
	doc := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build",` +
		`"status":"abandoned","disposition":"abandoned","disposition_reason":"stale",` +
		`"disposition_operator":"ship","disposition_at":"2026-08-24T00:00:00Z"}`
	assert.NoError(t, CheckConformingTopLevelNamespace([]byte(doc)))
}

// TestU2b_UnknownNestedProgressKeyRefused asserts an unknown key nested
// inside progress is refused, reported as "progress.<key>" (147-F / U2b).
// CheckConformingTopLevelNamespace already exists (U2 landed it), so this is
// a normal behavioural red: it fails via assertion, not a build error,
// because U2's flat top-level check never yet recurses into "progress".
func TestU2b_UnknownNestedProgressKeyRefused(t *testing.T) {
	doc := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build",` +
		`"progress":{"unexpected_nested":"x"}}`
	err := CheckConformingTopLevelNamespace([]byte(doc))
	require.Error(t, err)
	var typed *backlogiterrors.CheckpointNonConformingError
	require.True(t, errors.As(err, &typed))
	assert.Equal(t, []string{"progress.unexpected_nested"}, typed.Fields)
}

// TestU2bGuard_UnmodeledContextKeysReturnNil is the permanent 146-F
// open-namespace regression guard: unmodeled context keys must never be
// swept into refusal.
func TestU2bGuard_UnmodeledContextKeysReturnNil(t *testing.T) {
	doc := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build",` +
		`"context":{"anything_the_caller_wants":"x","nested":{"a":1}}}`
	assert.NoError(t, CheckConformingTopLevelNamespace([]byte(doc)))
}

// TestU2bGuard_NonObjectProgressReturnsNilWithoutPanicking asserts a
// non-object progress value (already governed by ParseCheckpoint) is not a
// conformance failure and does not panic.
func TestU2bGuard_NonObjectProgressReturnsNilWithoutPanicking(t *testing.T) {
	doc := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","progress":null}`
	assert.NotPanics(t, func() {
		assert.NoError(t, CheckConformingTopLevelNamespace([]byte(doc)))
	})
}

// TestU2c_ExactDuplicateTopLevelKeyRefused asserts an exact duplicate
// top-level key makes the document non-conforming, reported as
// "duplicate:<key>" (147-F / U2c).
func TestU2c_ExactDuplicateTopLevelKeyRefused(t *testing.T) {
	doc := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build",` +
		`"status":"active","status":"resolved"}`
	err := CheckConformingTopLevelNamespace([]byte(doc))
	require.Error(t, err)
	var typed *backlogiterrors.CheckpointNonConformingError
	require.True(t, errors.As(err, &typed))
	assert.Contains(t, typed.Fields, "duplicate:status")
}

// TestU2c_CaseVariantDuplicateTopLevelKeyRefused asserts a case-fold-variant
// duplicate top-level key ("status" + "Status") is non-conforming.
func TestU2c_CaseVariantDuplicateTopLevelKeyRefused(t *testing.T) {
	doc := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build",` +
		`"status":"active","Status":"active"}`
	err := CheckConformingTopLevelNamespace([]byte(doc))
	require.Error(t, err)
	var typed *backlogiterrors.CheckpointNonConformingError
	require.True(t, errors.As(err, &typed))
	assert.Contains(t, typed.Fields, "duplicate:status")
}

// TestU2cGuard_OneOccurrenceOfEveryModeledKeyStaysConforming asserts a
// document with one occurrence of every modeled key remains conforming.
func TestU2cGuard_OneOccurrenceOfEveryModeledKeyStaysConforming(t *testing.T) {
	doc := `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","status":"active"}`
	assert.NoError(t, CheckConformingTopLevelNamespace([]byte(doc)))
}
