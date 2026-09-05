// Package format_test contains U1b source-shape harness tests.
// These tests use go/ast to inspect jsonrpc.go and verify that the
// JSONRPCError.Data field and WrapErrorData function are declared.
// They compile and run before any implementation, failing (RED) until
// the production code is added.
package format_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const jsonrpcSourceFile = "jsonrpc.go"

// parseJSONRPCFile parses jsonrpc.go and returns the AST file node.
func parseJSONRPCFile(t *testing.T) *ast.File {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, jsonrpcSourceFile, nil, parser.ParseComments)
	require.NoError(t, err, "failed to parse %s", jsonrpcSourceFile)
	return f
}

// TestU1b_JSONRPCErrorDataFieldDeclared asserts that JSONRPCError has a Data
// field with a struct tag containing json key "data" and the "omitempty" option.
// FAILS before implementation (Data field absent). PASSES after.
func TestU1b_JSONRPCErrorDataFieldDeclared(t *testing.T) {
	f := parseJSONRPCFile(t)

	var found bool
	ast.Inspect(f, func(n ast.Node) bool {
		ts, ok := n.(*ast.TypeSpec)
		if !ok || ts.Name.Name != "JSONRPCError" {
			return true
		}
		st, ok := ts.Type.(*ast.StructType)
		if !ok {
			return true
		}
		for _, field := range st.Fields.List {
			for _, name := range field.Names {
				if name.Name != "Data" {
					continue
				}
				require.NotNil(t, field.Tag, "Data field must have a struct tag")

				// field.Tag.Value includes the surrounding backtick delimiters.
				// Strip them so we can use reflect.StructTag for canonical parsing.
				raw := field.Tag.Value
				if len(raw) >= 2 && raw[0] == '`' && raw[len(raw)-1] == '`' {
					raw = raw[1 : len(raw)-1]
				}
				jsonTag := reflect.StructTag(raw).Get("json")
				assert.Equal(t, "data,omitempty", jsonTag,
					`Data field json tag must be "data,omitempty"`)
				found = true
			}
		}
		return true
	})

	assert.True(t, found, "JSONRPCError must have a Data field")
}

// TestU1b_WrapErrorDataFuncDeclared asserts that a top-level function named
// WrapErrorData is declared in jsonrpc.go.
// FAILS before implementation (function absent). PASSES after.
func TestU1b_WrapErrorDataFuncDeclared(t *testing.T) {
	f := parseJSONRPCFile(t)

	var found bool
	for _, decl := range f.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if fd.Name.Name == "WrapErrorData" {
			found = true
			// Verify signature: four parameters (id, code, msg, data).
			require.NotNil(t, fd.Type.Params, "WrapErrorData must have parameters")
			assert.Equal(t, 4, fd.Type.Params.NumFields(),
				"WrapErrorData must accept exactly 4 parameters: id, code, msg, data")
			break
		}
	}

	assert.True(t, found, "WrapErrorData function must be declared in jsonrpc.go")
}
