package telemetry

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

type u175_037Target struct {
	file      string
	functions []string
}

func TestU175_037_LintFix(t *testing.T) {
	u175_037AssertCloseErrorsHandled(t, []u175_037Target{
		{file: "events_harvest.go", functions: []string{"HarvestEventsFacts"}},
		{file: "harvest_since_test.go", functions: []string{
			"TestHarvestTelemetry_Since_ExcludesOlderEvents",
			"TestHarvestTelemetry_Since_FullHarvestWhenAllEventsNewer",
			"TestHarvestTelemetry_Since_FutureDate_ReturnsZeroSessions",
			"TestHarvestTelemetry_ZeroTimestampEvents_AlwaysIncluded",
		}},
	})
}

func u175_037AssertCloseErrorsHandled(t *testing.T, targets []u175_037Target) {
	t.Helper()

	for _, target := range targets {
		fileSet := token.NewFileSet()
		parsed, err := parser.ParseFile(fileSet, target.file, nil, parser.AllErrors)
		if err != nil {
			t.Fatalf("parse %s: %v", target.file, err)
		}

		for _, functionName := range target.functions {
			var function *ast.FuncDecl
			for _, declaration := range parsed.Decls {
				candidate, ok := declaration.(*ast.FuncDecl)
				if ok && candidate.Name.Name == functionName {
					function = candidate
					break
				}
			}
			if function == nil || function.Body == nil {
				t.Errorf("%s: function %s is missing", target.file, functionName)
				continue
			}

			ast.Inspect(function.Body, func(node ast.Node) bool {
				if u175_037HasUncheckedClose(node) {
					position := fileSet.Position(node.Pos())
					t.Errorf("%s:%d: unchecked Close error in %s", target.file, position.Line, functionName)
				}
				return true
			})
		}
	}
}

func u175_037HasUncheckedClose(node ast.Node) bool {
	switch statement := node.(type) {
	case *ast.ExprStmt:
		return u175_037IsCloseCall(statement.X)
	case *ast.DeferStmt:
		return u175_037IsCloseCall(statement.Call)
	case *ast.AssignStmt:
		for _, right := range statement.Rhs {
			if !u175_037IsCloseCall(right) {
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

func u175_037IsCloseCall(expression ast.Expr) bool {
	call, ok := expression.(*ast.CallExpr)
	if !ok || len(call.Args) != 0 {
		return false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	return ok && selector.Sel.Name == "Close"
}
