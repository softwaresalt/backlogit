package cli_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestU175_008_LintFix asserts stash.go discards no Close error and that
// checkpoint_remediation_test.go returns errors as the last result (ST1008).
func TestU175_008_LintFix(t *testing.T) {
	fset := token.NewFileSet()

	stash, err := parser.ParseFile(fset, "stash.go", nil, 0)
	require.NoError(t, err)
	var unchecked []string
	ast.Inspect(stash, func(n ast.Node) bool {
		var call *ast.CallExpr
		switch s := n.(type) {
		case *ast.ExprStmt:
			call, _ = s.X.(*ast.CallExpr)
		case *ast.DeferStmt:
			call = s.Call
		case *ast.GoStmt:
			call = s.Call
		}
		if call == nil {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkg, isIdent := sel.X.(*ast.Ident)
		if sel.Sel.Name == "Close" || (isIdent && pkg.Name == "fmt" && strings.HasPrefix(sel.Sel.Name, "Fprint")) {
			unchecked = append(unchecked, fset.Position(call.Pos()).String())
		}
		return true
	})
	assert.Empty(t, unchecked, "unchecked error returns remain in stash.go")

	remediation, err := parser.ParseFile(fset, "checkpoint_remediation_test.go", nil, 0)
	require.NoError(t, err)
	var misordered []string
	for _, decl := range remediation.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Type.Results == nil {
			continue
		}
		list := fn.Type.Results.List
		for i, field := range list {
			if id, ok := field.Type.(*ast.Ident); ok && id.Name == "error" && i != len(list)-1 {
				misordered = append(misordered, fn.Name.Name)
			}
		}
	}
	assert.Empty(t, misordered, "error must be the last result (ST1008)")
}
