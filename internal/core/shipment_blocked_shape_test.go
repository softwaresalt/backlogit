package core

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUR1S_ShipmentBlockedStatusShape(t *testing.T) {
	file := parseUR1SFile(t, "shipment.go")
	statusSpec := findUR1STypeSpec(file, "ShipmentStatus")
	require.NotNil(t, statusSpec, "ShipmentStatus is not declared in shipment.go")
	assert.False(t, statusSpec.Assign.IsValid(), "ShipmentStatus must be a defined type, not a type alias")

	underlyingType, ok := statusSpec.Type.(*ast.Ident)
	require.True(t, ok, "ShipmentStatus must have an identifier underlying type")
	assert.Equal(t, "string", underlyingType.Name, "ShipmentStatus underlying type")

	spec := findUR1SValueSpec(file, "ShipmentBlocked")
	require.NotNil(t, spec, "ShipmentBlocked is not declared in shipment.go")
	assert.Equal(t, "ShipmentStatus", ur1sExprName(spec.Type), "ShipmentBlocked type")
	require.Len(t, spec.Values, 1, "ShipmentBlocked value count")

	value, ok := spec.Values[0].(*ast.BasicLit)
	require.True(t, ok, "ShipmentBlocked value must be a basic literal, got %T", spec.Values[0])
	assert.Equal(t, token.STRING, value.Kind, "ShipmentBlocked literal kind")
	assert.Equal(t, `"blocked"`, value.Value, "ShipmentBlocked value")
}

func TestUR1S_BlockAndUnblockAPIDeclarationShape(t *testing.T) {
	file := parseUR1SFile(t, "shipment.go")

	tests := []struct {
		name         string
		optionsType  string
		fields       []string
		functionName string
	}{
		{
			name:        "block",
			optionsType: "BlockOptions",
			fields: []string{
				"Reason string",
				"BlockedBy string",
				"ResumeCheckpointRef string",
			},
			functionName: "BlockShipment",
		},
		{
			name:        "unblock",
			optionsType: "UnblockOptions",
			fields: []string{
				"Target ShipmentStatus",
				"Confirm bool",
				"UnblockedBy string",
			},
			functionName: "UnblockShipment",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertUR1SStructFields(t, file, test.optionsType, test.fields)
			assertUR1SFunctionShape(t, file, test.functionName, test.optionsType)
		})
	}
}

func TestUR1S_BlockAndUnblockSentinelShape(t *testing.T) {
	// Transient stub-body verification belongs to the 174.052-T task-time
	// gate/review, not this permanent declaration-shape harness.
	file := parseUR1SFile(t, "../errors/errors.go")
	tests := []struct {
		name        string
		wantMessage string
	}{
		{name: "ErrNotImplemented", wantMessage: `"backlogit: not implemented"`},
		{name: "ErrShipmentBlockedRequiresEnvelope"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assertUR1SErrorSentinel(t, file, test.name, test.wantMessage)
		})
	}
}

func parseUR1SFile(t *testing.T, path string) *ast.File {
	t.Helper()

	file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.AllErrors)
	require.NoError(t, err, "parse %s", path)
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

func findUR1STypeSpec(file *ast.File, name string) *ast.TypeSpec {
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}
		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if ok && typeSpec.Name.Name == name {
				return typeSpec
			}
		}
	}
	return nil
}

func assertUR1SStructFields(t *testing.T, file *ast.File, typeName string, want []string) {
	t.Helper()

	typeSpec := findUR1STypeSpec(file, typeName)
	require.NotNil(t, typeSpec, "%s struct is not declared in shipment.go", typeName)
	assert.False(t, typeSpec.Assign.IsValid(), "%s must be a defined struct type, not a type alias", typeName)

	structType, ok := typeSpec.Type.(*ast.StructType)
	require.True(t, ok, "%s must be declared with an underlying struct type", typeName)

	got := make([]string, 0, len(structType.Fields.List))
	for _, field := range structType.Fields.List {
		if !assert.NotEmpty(t, field.Names, "%s must not contain anonymous or embedded fields", typeName) {
			continue
		}
		for _, name := range field.Names {
			got = append(got, name.Name+" "+ur1sExprName(field.Type))
		}
	}
	assert.Equal(t, want, got, "%s fields", typeName)
}

func assertUR1SFunctionShape(t *testing.T, file *ast.File, functionName, optionsType string) {
	t.Helper()

	decl := findUR1SFunction(file, functionName)
	require.NotNil(t, decl, "%s is not declared in shipment.go", functionName)
	assert.Nil(t, decl.Recv, "%s must be receiver-less", functionName)
	assert.Nil(t, decl.Type.TypeParams, "%s must not declare type parameters", functionName)

	wantParams := []string{
		"ctx context.Context",
		"ws *Workspace",
		"shipmentID string",
		"opts " + optionsType,
	}
	gotParams := ur1sFieldList(decl.Type.Params)
	assert.Equal(t, wantParams, gotParams, "%s parameters", functionName)

	wantResults := []string{"*models.Artifact", "error"}
	gotResults := ur1sFieldList(decl.Type.Results)
	assert.Equal(t, wantResults, gotResults, "%s results", functionName)
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
				require.Less(t, index, len(valueSpec.Values), "%s must be initialized with errors.New", name)

				call, ok := valueSpec.Values[index].(*ast.CallExpr)
				require.True(t, ok, "%s must be initialized with errors.New", name)
				assert.Equal(t, "errors.New", ur1sExprName(call.Fun), "%s initializer", name)
				require.Len(t, call.Args, 1, "%s errors.New argument count", name)

				if wantMessage != "" {
					message, ok := call.Args[0].(*ast.BasicLit)
					require.True(t, ok, "%s message must be a basic literal", name)
					assert.Equal(t, token.STRING, message.Kind, "%s message literal kind", name)
					assert.Equal(t, wantMessage, message.Value, "%s message", name)
				}
				return
			}
		}
	}
	assert.Fail(t, name+" is not declared in internal/errors/errors.go")
}
