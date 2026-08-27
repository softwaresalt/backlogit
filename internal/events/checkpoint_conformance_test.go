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
