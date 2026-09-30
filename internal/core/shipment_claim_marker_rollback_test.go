package core

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"testing"
)

func TestU3_RollbackDeliverableHasBothScenarios(t *testing.T) {
	source, err := os.ReadFile("shipment_claim_marker_rollback_test.go")
	if err != nil {
		t.Fatalf("read rollback test deliverable: %v", err)
	}

	file, err := parser.ParseFile(token.NewFileSet(), "shipment_claim_marker_rollback_test.go", source, 0)
	if err != nil {
		t.Fatalf("parse rollback test deliverable: %v", err)
	}

	type scenarioShape struct {
		name        string
		identifiers []string
		selectors   []string
	}

	scenarios := []scenarioShape{
		{
			name: "TestU3_InProcessSnapshotRollbackRestoresPreimage",
			identifiers: []string{
				"ClaimShipment",
				"CreateShipment",
				"persistArtifactWriteFn",
				"schedulerBaselineClaimKey",
				"requireP1C6DurableStateRestored",
			},
			selectors: []string{"Cleanup", "StatusActive", "StatusQueued"},
		},
		{
			name: "TestU3_JournalRecoveryRollbackRestoresPreimage",
			identifiers: []string{
				"ClaimShipment",
				"CreateShipment",
				"persistArtifactWriteFn",
				"restoreShipmentSnapshotFn",
				"NewWorkspace",
				"bldb",
				"schedulerBaselineClaimKey",
				"requireP1C6DurableStateRestored",
			},
			selectors: []string{"Cleanup", "GetItem", "Path", "StatusActive", "StatusQueued"},
		},
	}

	functions := make(map[string]*ast.FuncDecl, len(scenarios))
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if ok {
			functions[function.Name.Name] = function
		}
	}

	for _, scenario := range scenarios {
		function, ok := functions[scenario.name]
		if !ok || function.Body == nil {
			t.Errorf("rollback scenario %s is not declared in shipment_claim_marker_rollback_test.go", scenario.name)
			continue
		}

		identifiers := make(map[string]bool)
		selectors := make(map[string]bool)
		ast.Inspect(function.Body, func(node ast.Node) bool {
			switch expression := node.(type) {
			case *ast.Ident:
				identifiers[expression.Name] = true
			case *ast.SelectorExpr:
				selectors[expression.Sel.Name] = true
			}
			return true
		})

		for _, name := range scenario.identifiers {
			if !identifiers[name] {
				t.Errorf("%s must exercise %s", scenario.name, name)
			}
		}
		for _, name := range scenario.selectors {
			if !selectors[name] {
				t.Errorf("%s must exercise selector %s", scenario.name, name)
			}
		}
	}
}
