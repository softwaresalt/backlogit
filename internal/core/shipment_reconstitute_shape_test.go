package core

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// TestU1_ReconstituteDeclarationsSourceShape gates 209.001-T declarations only.
// Struct fields, tags, function bodies, and internal helpers belong to later units.
func TestU1_ReconstituteDeclarationsSourceShape(t *testing.T) {
	t.Run("event constant", func(t *testing.T) {
		file := parseReconstituteShapeSource(t, "shipment_events.go")
		for _, declaration := range file.Decls {
			group, ok := declaration.(*ast.GenDecl)
			if !ok || group.Tok != token.CONST {
				continue
			}
			for _, spec := range group.Specs {
				value, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for index, name := range value.Names {
					if name.Name != "EventShipmentReconstituted" || index >= len(value.Values) {
						continue
					}
					literal, ok := value.Values[index].(*ast.BasicLit)
					if ok && literal.Kind == token.STRING &&
						(literal.Value == `"shipment_reconstituted"` || literal.Value == "`shipment_reconstituted`") {
						return
					}
				}
			}
		}
		t.Fatal(`EventShipmentReconstituted = "shipment_reconstituted" is not declared in shipment_events.go`)
	})

	t.Run("hook signatures", func(t *testing.T) {
		file := parseReconstituteShapeSource(t, "../hooks/hooks.go")
		for _, signature := range []struct {
			name    string
			params  string
			results string
		}{
			{"WithBufferedPostHooks", "context.Context", "context.Context"},
			{"FlushBufferedPostHooks", "context.Context,*HookRunner", ""},
		} {
			t.Run(signature.name, func(t *testing.T) {
				assertReconstituteShapeFunction(t, file, signature.name, signature.params, signature.results)
			})
		}
	})

	t.Run("reconstitute declarations", func(t *testing.T) {
		file := parseReconstituteShapeSource(t, "shipment_reconstitute.go")
		for _, name := range []string{
			"ReconstitutePlan",
			"ReconstituteShipmentSpec",
			"ReconstituteFeatureSpec",
			"ReconstituteCreateFeatureSpec",
			"ReconstituteRewireSpec",
			"ReconstituteOptions",
			"ReconstituteDependencyChange",
			"ReconstituteCreatedArtifact",
			"ReconstituteShipmentResult",
		} {
			t.Run(name, func(t *testing.T) {
				for _, declaration := range file.Decls {
					group, ok := declaration.(*ast.GenDecl)
					if !ok || group.Tok != token.TYPE {
						continue
					}
					for _, spec := range group.Specs {
						typ, ok := spec.(*ast.TypeSpec)
						if !ok || typ.Name.Name != name || typ.Assign.IsValid() || typ.TypeParams != nil {
							continue
						}
						if _, ok := typ.Type.(*ast.StructType); ok {
							return
						}
					}
				}
				t.Fatalf("%s struct is not declared in shipment_reconstitute.go", name)
			})
		}
		assertReconstituteShapeFunction(t, file, "ReconstituteShipments",
			"context.Context,*Workspace,ReconstituteOptions", "*ReconstituteShipmentResult,error")
	})
}

func parseReconstituteShapeSource(t *testing.T, path string) *ast.File {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatalf("required declaration source %s is missing or invalid: %v", path, err)
	}
	return file
}

func assertReconstituteShapeFunction(t *testing.T, file *ast.File, name, params, results string) {
	t.Helper()
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Name.Name != name || function.Recv != nil || function.Type.TypeParams != nil {
			continue
		}
		if reconstituteShapeFieldTypes(function.Type.Params) == params &&
			reconstituteShapeFieldTypes(function.Type.Results) == results {
			return
		}
	}
	t.Errorf("%s(%s) (%s) receiverless function signature is not declared in package %s",
		name, params, results, file.Name.Name)
}

func reconstituteShapeFieldTypes(fields *ast.FieldList) string {
	if fields == nil {
		return ""
	}
	types := make([]string, 0, len(fields.List))
	for _, field := range fields.List {
		width := len(field.Names)
		if width == 0 {
			width = 1
		}
		for range width {
			types = append(types, reconstituteShapeTypeName(field.Type))
		}
	}
	return strings.Join(types, ",")
}

func reconstituteShapeTypeName(expression ast.Expr) string {
	switch typ := expression.(type) {
	case *ast.Ident:
		return typ.Name
	case *ast.SelectorExpr:
		return reconstituteShapeTypeName(typ.X) + "." + typ.Sel.Name
	case *ast.StarExpr:
		return "*" + reconstituteShapeTypeName(typ.X)
	default:
		return "<unexpected type>"
	}
}
