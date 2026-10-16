package atomicfile

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestU175_002_LintFix asserts the owned file discards no Close error.
func TestU175_002_LintFix(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "atomicfile_bench_test.go", nil, 0)
	require.NoError(t, err)

	var offenders []string
	ast.Inspect(file, func(n ast.Node) bool {
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
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Close" {
			offenders = append(offenders, fset.Position(call.Pos()).String())
		}
		return true
	})
	assert.Empty(t, offenders, "unchecked Close results remain")
}
