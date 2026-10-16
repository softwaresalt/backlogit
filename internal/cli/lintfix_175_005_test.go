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

// TestU175_005_LintFix asserts the owned files discard no Close/fmt.Fprint* error.
func TestU175_005_LintFix(t *testing.T) {
	fset := token.NewFileSet()
	var offenders []string
	for _, name := range []string{"adopt.go", "queue_cmd.go"} {
		file, err := parser.ParseFile(fset, name, nil, 0)
		require.NoError(t, err)
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
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, isIdent := sel.X.(*ast.Ident)
			if sel.Sel.Name == "Close" || (isIdent && pkg.Name == "fmt" && strings.HasPrefix(sel.Sel.Name, "Fprint")) {
				offenders = append(offenders, fset.Position(call.Pos()).String())
			}
			return true
		})
	}
	assert.Empty(t, offenders, "unchecked error returns remain")
}
