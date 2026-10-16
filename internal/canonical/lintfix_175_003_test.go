package canonical

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestU175_003_LintFix asserts canonical.go has no WriteString(fmt.Sprintf(...)) (QF1012).
func TestU175_003_LintFix(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "canonical.go", nil, 0)
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
	assert.Empty(t, offenders, "WriteString(fmt.Sprintf(...)) remains")
}
