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

// TestU175_011_LintFix asserts migrate.go (slice 1/2) has retired its owned
// share of unchecked Close/fmt.Fprint* error returns. 19 occurrences existed at
// baseline; slice 1 owns 10, leaving at most 9 for the later slice.
func TestU175_011_LintFix(t *testing.T) {
	limits := map[string]int{"migrate.go": 9}
	for name, limit := range limits {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, name, nil, 0)
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
		assert.LessOrEqual(t, len(offenders), limit, "%s: unchecked error returns exceed residual: %v", name, offenders)
	}
}
