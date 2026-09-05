package docline

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestU3_ValidateClassifyPathDeclared asserts that docline package declares
// ValidateClassifyPath(root, path string) error (155.003-T / U3).
func TestU3_ValidateClassifyPathDeclared(t *testing.T) {
	// Parse all .go files in the docline package directory
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", nil, 0)
	require.NoError(t, err)

	var found *ast.FuncDecl
	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			for _, decl := range file.Decls {
				funcDecl, ok := decl.(*ast.FuncDecl)
				if ok && funcDecl.Recv == nil && funcDecl.Name.Name == "ValidateClassifyPath" {
					found = funcDecl
				}
			}
		}
	}
	if !assert.NotNil(t, found, "ValidateClassifyPath function not declared in docline package") {
		return
	}
	// Verify signature: (root, path string) error
	require.Len(t, found.Type.Params.List, 2, "ValidateClassifyPath must take 2 parameters (root, path string)")
	require.Len(t, found.Type.Results.List, 1, "ValidateClassifyPath must return exactly one value")
	resType, ok := found.Type.Results.List[0].Type.(*ast.Ident)
	assert.True(t, ok && resType.Name == "error", "ValidateClassifyPath must return error")
}
