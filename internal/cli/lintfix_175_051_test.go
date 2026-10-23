package cli_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestU175_051_LintFix asserts the owned functions discard no ws.Close/fmt.Fprint* error.
func TestU175_051_LintFix(t *testing.T) {
	owned := map[string][]string{
		"root.go":                            {"newInitCommand", "newSyncCommand"},
		"shipment_reconcile_shipped_test.go": {"reconcileShippedArchivePath", "reconcileShippedBuildFixture", "reconcileShippedBuildUnarchivedFixture"},
	}
	fset := token.NewFileSet()
	var offenders []string
	visited := map[string]bool{}
	allBlank := func(exprs []ast.Expr) bool {
		for _, e := range exprs {
			if id, ok := e.(*ast.Ident); !ok || id.Name != "_" {
				return false
			}
		}
		return true
	}
	for name, funcs := range owned {
		file, err := parser.ParseFile(fset, name, nil, 0)
		require.NoError(t, err)
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || !slices.Contains(funcs, fn.Name.Name) {
				continue
			}
			visited[name+":"+fn.Name.Name] = true
			ast.Inspect(fn, func(n ast.Node) bool {
				var call *ast.CallExpr
				switch s := n.(type) {
				case *ast.ExprStmt:
					call, _ = s.X.(*ast.CallExpr)
				case *ast.DeferStmt:
					call = s.Call
				case *ast.GoStmt:
					call = s.Call
				case *ast.AssignStmt:
					if len(s.Rhs) == 1 && allBlank(s.Lhs) {
						call, _ = s.Rhs[0].(*ast.CallExpr)
					}
				}
				if call == nil {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				recv, isIdent := sel.X.(*ast.Ident)
				if isIdent && ((recv.Name == "ws" && sel.Sel.Name == "Close") || (recv.Name == "fmt" && strings.HasPrefix(sel.Sel.Name, "Fprint"))) {
					offenders = append(offenders, fset.Position(call.Pos()).String())
				}
				return true
			})
		}
	}
	var missing []string
	for name, funcs := range owned {
		for _, f := range funcs {
			if !visited[name+":"+f] {
				missing = append(missing, name+":"+f)
			}
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("owned functions not found: %v", missing)
	}
	sort.Strings(offenders)
	assert.Empty(t, offenders, "unchecked error returns remain")
}
