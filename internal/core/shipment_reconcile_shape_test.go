package core

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"
)

// TestShipmentReconcileDeclarationsShape is the U1 source-shape harness
// (167.003-T). It inspects the SOURCE of internal/core/shipment_reconcile.go
// via go/parser/go/ast rather than referencing the declared symbols directly,
// so it compiles against a declaration-free tree and fails on a missing
// declaration rather than a build error (P-002.1/P-002.6 source-shape
// harness convention).
func TestShipmentReconcileDeclarationsShape(t *testing.T) {
	path := filepath.Join(".", "shipment_reconcile.go")
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.AllErrors)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	wantTypes := map[string]bool{
		"ShipmentShippedReconcileRequest": false,
		"ShipmentShippedReconcileResult":  false,
		"ShipmentReconcileOutcome":        false,
	}
	wantFuncs := map[string]bool{
		"classifyShipmentReconcileState":    false,
		"ReconcileShipmentToShipped":        false,
		"writeShipmentReconcileArchiveFile": false,
		"lockShipmentReconcileItemLog":      false,
		"appendShipmentReconcileEvent":      false,
		"snapshotShipmentReconcile":         false,
		"restoreShipmentReconcile":          false,
	}

	ast.Inspect(file, func(n ast.Node) bool {
		switch decl := n.(type) {
		case *ast.TypeSpec:
			if _, ok := wantTypes[decl.Name.Name]; ok {
				wantTypes[decl.Name.Name] = true
			}
		case *ast.FuncDecl:
			if decl.Recv == nil {
				if _, ok := wantFuncs[decl.Name.Name]; ok {
					wantFuncs[decl.Name.Name] = true
				}
			}
		}
		return true
	})

	for name, found := range wantTypes {
		if !found {
			t.Errorf("expected type declaration %s in shipment_reconcile.go", name)
		}
	}
	for name, found := range wantFuncs {
		if !found {
			t.Errorf("expected function declaration %s in shipment_reconcile.go", name)
		}
	}
}
