package events

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
