package core

import (
	"go/ast"
	"go/token"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestShipmentDispositionDeclarations pins the permanent disposition API
// declarations by signature only (B-U1 / 192.001-T). It reads source text via
// go/parser, so it compiles before the declarations exist and fails on an
// assertion rather than on a build error. Function bodies are not pinned.
func TestShipmentDispositionDeclarations(t *testing.T) {
	t.Run("function", func(t *testing.T) {
		var found *ast.FuncDecl
		for _, f := range readinessParseDir(t, ".") {
			for _, d := range f.Decls {
				if fd, ok := d.(*ast.FuncDecl); ok && fd.Recv == nil && fd.Name.Name == "DisposeQueuedShipment" {
					found = fd
				}
			}
		}
		require.NotNil(t, found, "func DisposeQueuedShipment is not declared in internal/core")
		require.Equal(t,
			[]string{"context.Context", "*Workspace", "string", "ShipmentDispositionOptions"},
			readinessFieldTypes(found.Type.Params))
		require.Equal(t, []string{"*models.Artifact", "error"}, readinessFieldTypes(found.Type.Results))
	})

	t.Run("options", func(t *testing.T) {
		var found *ast.StructType
		for _, f := range readinessParseDir(t, ".") {
			for _, d := range f.Decls {
				gd, ok := d.(*ast.GenDecl)
				if !ok || gd.Tok != token.TYPE {
					continue
				}
				for _, s := range gd.Specs {
					ts, ok := s.(*ast.TypeSpec)
					if !ok || ts.Name.Name != "ShipmentDispositionOptions" {
						continue
					}
					if st, isStruct := ts.Type.(*ast.StructType); isStruct {
						found = st
					}
				}
			}
		}
		require.NotNil(t, found, "type ShipmentDispositionOptions struct is not declared in internal/core")
		fields := map[string]string{}
		for _, field := range found.Fields.List {
			for _, name := range field.Names {
				fields[name.Name] = readinessExprString(field.Type)
			}
		}
		require.Equal(t, map[string]string{
			"Reason":           "string",
			"SupersededBy":     "string",
			"By":               "string",
			"AuthorizationRef": "string",
			"Confirm":          "bool",
		}, fields)
	})

	t.Run("sentinel", func(t *testing.T) {
		var found bool
		for _, f := range readinessParseDir(t, "../errors") {
			for _, d := range f.Decls {
				gd, ok := d.(*ast.GenDecl)
				if !ok || gd.Tok != token.VAR {
					continue
				}
				for _, s := range gd.Specs {
					if vs, ok := s.(*ast.ValueSpec); ok {
						for _, n := range vs.Names {
							if n.Name == "ErrShipmentDispositionRefused" {
								found = true
							}
						}
					}
				}
			}
		}
		require.True(t, found, "ErrShipmentDispositionRefused is not declared in internal/errors")
	})
}
