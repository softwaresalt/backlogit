package telemetry

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// u175_038ResidualWriteStringSprintf is the count still owned by the later slice of reporter.go.
const u175_038ResidualWriteStringSprintf = 14

func TestU175_038_LintFix(t *testing.T) {
	fileSet := token.NewFileSet()
	parsed, err := parser.ParseFile(fileSet, "reporter.go", nil, parser.AllErrors)
	if err != nil {
		t.Fatalf("parse reporter.go: %v", err)
	}

	closeFunctions := map[string]bool{"readSessionFacts": false, "readToolCallFacts": false}
	sprintfCount := 0

	for _, declaration := range parsed.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Body == nil {
			continue
		}
		_, checkClose := closeFunctions[function.Name.Name]
		if checkClose {
			closeFunctions[function.Name.Name] = true
		}

		ast.Inspect(function.Body, func(node ast.Node) bool {
			if u175_038IsWriteStringSprintf(node) {
				sprintfCount++
			}
			if checkClose && u175_038HasUncheckedClose(node) {
				position := fileSet.Position(node.Pos())
				t.Errorf("reporter.go:%d: unchecked Close error in %s", position.Line, function.Name.Name)
			}
			return true
		})
	}

	for name, found := range closeFunctions {
		if !found {
			t.Errorf("reporter.go: function %s is missing", name)
		}
	}
	if sprintfCount > u175_038ResidualWriteStringSprintf {
		t.Errorf("reporter.go: %d WriteString(fmt.Sprintf(...)) calls remain; want at most %d (QF1012)", sprintfCount, u175_038ResidualWriteStringSprintf)
	}
}

func u175_038IsWriteStringSprintf(node ast.Node) bool {
	statement, ok := node.(*ast.ExprStmt)
	if !ok {
		return false
	}
	call, ok := statement.X.(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "WriteString" {
		return false
	}
	inner, ok := call.Args[0].(*ast.CallExpr)
	if !ok {
		return false
	}
	innerSelector, ok := inner.Fun.(*ast.SelectorExpr)
	if !ok || innerSelector.Sel.Name != "Sprintf" {
		return false
	}
	pkg, ok := innerSelector.X.(*ast.Ident)
	return ok && pkg.Name == "fmt"
}

func u175_038HasUncheckedClose(node ast.Node) bool {
	switch statement := node.(type) {
	case *ast.ExprStmt:
		return u175_038IsCloseCall(statement.X)
	case *ast.DeferStmt:
		return u175_038IsCloseCall(statement.Call)
	case *ast.AssignStmt:
		for _, right := range statement.Rhs {
			if !u175_038IsCloseCall(right) {
				continue
			}
			for _, left := range statement.Lhs {
				identifier, ok := left.(*ast.Ident)
				if ok && identifier.Name == "_" {
					return true
				}
			}
		}
	}
	return false
}

func u175_038IsCloseCall(expression ast.Expr) bool {
	call, ok := expression.(*ast.CallExpr)
	if !ok || len(call.Args) != 0 {
		return false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	return ok && selector.Sel.Name == "Close"
}
