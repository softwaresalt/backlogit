package core

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestUR1S_ShipmentBlockedStatusShape(t *testing.T) {
	file := parseUR1SFile(t, "shipment.go")
	spec := findUR1SValueSpec(file, "ShipmentBlocked")
	if spec == nil {
		t.Fatal("ShipmentBlocked is not declared in shipment.go")
	}
	if got := ur1sExprName(spec.Type); got != "ShipmentStatus" {
		t.Errorf("ShipmentBlocked type = %q, want ShipmentStatus", got)
	}
	if len(spec.Values) != 1 {
		t.Fatalf("ShipmentBlocked value count = %d, want 1", len(spec.Values))
	}
	value, ok := spec.Values[0].(*ast.BasicLit)
	if !ok || value.Kind != token.STRING || value.Value != `"blocked"` {
		t.Errorf("ShipmentBlocked value must be the string literal \"blocked\", got %T", spec.Values[0])
	}
}

func TestUR1S_BlockAndUnblockAPIDeclarationShape(t *testing.T) {
	file := parseUR1SFile(t, "shipment.go")

	assertUR1SStructFields(t, file, "BlockOptions", []string{
		"Reason string",
		"BlockedBy string",
		"ResumeCheckpointRef string",
	})
	assertUR1SStructFields(t, file, "UnblockOptions", []string{
		"Target ShipmentStatus",
		"Confirm bool",
		"UnblockedBy string",
	})

	assertUR1SFunctionShape(t, file, "BlockShipment", "BlockOptions")
	assertUR1SFunctionShape(t, file, "UnblockShipment", "UnblockOptions")
}

func TestUR1S_BlockAndUnblockSentinelShape(t *testing.T) {
	shipmentFile := parseUR1SFile(t, "shipment.go")
	for _, functionName := range []string{"BlockShipment", "UnblockShipment"} {
		decl := findUR1SFunction(shipmentFile, functionName)
		if decl == nil {
			t.Errorf("%s is not declared in shipment.go", functionName)
			continue
		}
		if !ur1sContainsSelector(decl.Body, "blerrors", "ErrNotImplemented") {
			t.Errorf("%s must reference blerrors.ErrNotImplemented", functionName)
		}
	}

	errorsFile := parseUR1SFile(t, "../errors/errors.go")
	assertUR1SErrorSentinel(t, errorsFile, "ErrNotImplemented", `"backlogit: not implemented"`)
	assertUR1SErrorSentinel(t, errorsFile, "ErrShipmentBlockedRequiresEnvelope", "")
}

func parseUR1SFile(t *testing.T, path string) *ast.File {
	t.Helper()

	file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.AllErrors)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return file
}

func findUR1SValueSpec(file *ast.File, name string) *ast.ValueSpec {
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.CONST {
			continue
		}
		for _, spec := range genDecl.Specs {
			valueSpec, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for _, ident := range valueSpec.Names {
				if ident.Name == name {
					return valueSpec
				}
			}
		}
	}
	return nil
}

func assertUR1SStructFields(t *testing.T, file *ast.File, typeName string, want []string) {
	t.Helper()

	var structType *ast.StructType
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}
		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok || typeSpec.Name.Name != typeName {
				continue
			}
			structType, _ = typeSpec.Type.(*ast.StructType)
		}
	}
	if structType == nil {
		t.Errorf("%s struct is not declared in shipment.go", typeName)
		return
	}

	got := make([]string, 0, len(structType.Fields.List))
	for _, field := range structType.Fields.List {
		for _, name := range field.Names {
			got = append(got, name.Name+" "+ur1sExprName(field.Type))
		}
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("%s fields = %v, want %v", typeName, got, want)
	}
}

func assertUR1SFunctionShape(t *testing.T, file *ast.File, functionName, optionsType string) {
	t.Helper()

	decl := findUR1SFunction(file, functionName)
	if decl == nil {
		t.Errorf("%s is not declared in shipment.go", functionName)
		return
	}
	if decl.Recv != nil {
		t.Errorf("%s must be receiver-less", functionName)
	}

	wantParams := []string{
		"ctx context.Context",
		"ws *Workspace",
		"shipmentID string",
		"opts " + optionsType,
	}
	gotParams := ur1sFieldList(decl.Type.Params)
	if strings.Join(gotParams, ",") != strings.Join(wantParams, ",") {
		t.Errorf("%s parameters = %v, want %v", functionName, gotParams, wantParams)
	}

	wantResults := []string{"*models.Artifact", "error"}
	gotResults := ur1sFieldList(decl.Type.Results)
	if strings.Join(gotResults, ",") != strings.Join(wantResults, ",") {
		t.Errorf("%s results = %v, want %v", functionName, gotResults, wantResults)
	}
}

func findUR1SFunction(file *ast.File, name string) *ast.FuncDecl {
	for _, decl := range file.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if ok && funcDecl.Name.Name == name {
			return funcDecl
		}
	}
	return nil
}

func ur1sFieldList(list *ast.FieldList) []string {
	if list == nil {
		return nil
	}

	fields := make([]string, 0, len(list.List))
	for _, field := range list.List {
		typeName := ur1sExprName(field.Type)
		if len(field.Names) == 0 {
			fields = append(fields, typeName)
			continue
		}
		for _, name := range field.Names {
			fields = append(fields, name.Name+" "+typeName)
		}
	}
	return fields
}

func ur1sExprName(expr ast.Expr) string {
	switch value := expr.(type) {
	case *ast.Ident:
		return value.Name
	case *ast.SelectorExpr:
		return ur1sExprName(value.X) + "." + value.Sel.Name
	case *ast.StarExpr:
		return "*" + ur1sExprName(value.X)
	default:
		return ""
	}
}

func ur1sContainsSelector(node ast.Node, packageName, selectorName string) bool {
	found := false
	ast.Inspect(node, func(candidate ast.Node) bool {
		selector, ok := candidate.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		ident, ok := selector.X.(*ast.Ident)
		if ok && ident.Name == packageName && selector.Sel.Name == selectorName {
			found = true
			return false
		}
		return true
	})
	return found
}

func assertUR1SErrorSentinel(t *testing.T, file *ast.File, name, wantMessage string) {
	t.Helper()

	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.VAR {
			continue
		}
		for _, spec := range genDecl.Specs {
			valueSpec, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for index, ident := range valueSpec.Names {
				if ident.Name != name {
					continue
				}
				if index >= len(valueSpec.Values) {
					t.Errorf("%s must be initialized with errors.New", name)
					return
				}
				call, ok := valueSpec.Values[index].(*ast.CallExpr)
				if !ok || ur1sExprName(call.Fun) != "errors.New" || len(call.Args) != 1 {
					t.Errorf("%s must be initialized with errors.New", name)
					return
				}
				if wantMessage != "" {
					message, ok := call.Args[0].(*ast.BasicLit)
					if !ok || message.Kind != token.STRING || message.Value != wantMessage {
						t.Errorf("%s message must be %s", name, wantMessage)
					}
				}
				return
			}
		}
	}
	t.Errorf("%s is not declared in internal/errors/errors.go", name)
}
