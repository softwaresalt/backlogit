package core

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestU1b_SchedulerBaselineClaimKeyDeclared(t *testing.T) {
	const wantValue = "scheduler_baseline_claim"

	file, err := parser.ParseFile(token.NewFileSet(), "shipment_lifecycle.go", nil, parser.AllErrors)
	if err != nil {
		t.Fatalf("parse shipment_lifecycle.go: %v", err)
	}

	declared := false
	ast.Inspect(file, func(node ast.Node) bool {
		declaration, ok := node.(*ast.GenDecl)
		if !ok || declaration.Tok != token.CONST {
			return true
		}

		for _, spec := range declaration.Specs {
			values, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for index, name := range values.Names {
				if name.Name != "schedulerBaselineClaimKey" || index >= len(values.Values) {
					continue
				}
				literal, ok := values.Values[index].(*ast.BasicLit)
				if ok && literal.Kind == token.STRING && strings.Trim(literal.Value, "\"") == wantValue {
					declared = true
					return false
				}
			}
		}
		return true
	})

	if !declared {
		t.Errorf("schedulerBaselineClaimKey = %q is not declared in shipment_lifecycle.go", wantValue)
	}
}
