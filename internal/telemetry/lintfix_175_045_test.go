package telemetry_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestU175_045_LintFix asserts reporter.go has no WriteString(fmt.Sprintf(...)) call.
func TestU175_045_LintFix(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "reporter.go", nil, 0)
	require.NoError(t, err)
	var offenders []string
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) != 1 {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "WriteString" {
			return true
		}
		inner, ok := call.Args[0].(*ast.CallExpr)
		if !ok {
			return true
		}
		isel, ok := inner.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if pkg, ok := isel.X.(*ast.Ident); ok && pkg.Name == "fmt" && isel.Sel.Name == "Sprintf" {
			offenders = append(offenders, fset.Position(call.Pos()).String())
		}
		return true
	})
	assert.Empty(t, offenders, "WriteString(fmt.Sprintf(...)) calls remain")
}
