package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"testing"
)

func TestMulticheckerEnumeratesAll(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "main.go", nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse main.go: %v", err)
	}

	call := findMulticheckerMainCall(t, file)
	if call.Ellipsis.IsValid() {
		t.Fatal("multichecker.Main must enumerate analyzers directly, not through a registry or expanded slice")
	}

	wantSelectors := []string{
		"scannerdiscipline.Analyzer",
		"errwrap.Analyzer",
		"failopen.Analyzer",
		"auditsuccess.Analyzer",
		"locktimeout.Analyzer",
	}
	analyzerNames := map[string]string{
		"scannerdiscipline.Analyzer": "FL001scannerdiscipline",
		"errwrap.Analyzer":           "FL002errwrap",
		"failopen.Analyzer":          "FL003failopen",
		"auditsuccess.Analyzer":      "FL004auditsuccess",
		"locktimeout.Analyzer":       "FL005locktimeout",
	}

	if len(call.Args) != len(wantSelectors) {
		t.Fatalf("multichecker.Main enumerates %d analyzers, want exactly %d", len(call.Args), len(wantSelectors))
	}

	gotNames := make([]string, 0, len(call.Args))
	seen := make(map[string]int, len(call.Args))
	for index, arg := range call.Args {
		selector, ok := directSelector(arg)
		if !ok {
			t.Fatalf("multichecker.Main argument %d must be a direct package.Analyzer selector; reflection and registries are forbidden", index+1)
		}
		seen[selector]++

		name, ok := analyzerNames[selector]
		if !ok {
			t.Fatalf("multichecker.Main argument %d is %s, want one of the five fault-line analyzers", index+1, selector)
		}
		gotNames = append(gotNames, name)
	}

	for _, selector := range wantSelectors {
		if seen[selector] != 1 {
			t.Errorf("multichecker.Main references %s %d times, want exactly once", selector, seen[selector])
		}
	}

	sortedNames := append([]string(nil), gotNames...)
	sort.Strings(sortedNames)
	for index := range gotNames {
		if gotNames[index] != sortedNames[index] {
			t.Fatalf("multichecker.Main analyzer order is %v, want alphabetical analyzer-name order %v", gotNames, sortedNames)
		}
	}
}

func findMulticheckerMainCall(t *testing.T, file *ast.File) *ast.CallExpr {
	t.Helper()

	var calls []*ast.CallExpr
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := directSelector(call.Fun)
		if ok && selector == "multichecker.Main" {
			calls = append(calls, call)
		}
		return true
	})

	if len(calls) != 1 {
		t.Fatalf("main.go contains %d multichecker.Main calls, want exactly one", len(calls))
	}
	return calls[0]
}

func directSelector(expr ast.Expr) (string, bool) {
	selector, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}
	pkg, ok := selector.X.(*ast.Ident)
	if !ok {
		return "", false
	}
	return pkg.Name + "." + selector.Sel.Name, true
}
