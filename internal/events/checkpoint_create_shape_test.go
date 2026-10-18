package events

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/require"
)

// Source-shape harness (171.001-T). It parses the files that carry the
// create-time legacy-import vocabulary and asserts the declared shapes via
// go/ast only, so it compiles against a tree that lacks the declarations and
// fails on an assertion instead of a build error. Shape only: no behavior.

func parseOptionalSource(t *testing.T, path string) *ast.File {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.AllErrors)
	if err != nil {
		return nil
	}
	return file
}

func TestCheckpointCreateShape_OptionType(t *testing.T) {
	file := parseOptionalSource(t, "checkpoint_create_options.go")
	require.NotNil(t, file, "checkpoint_create_options.go must exist and parse")
	spec := findPackageTypeIn(file, "CreateCheckpointOption")
	require.NotNil(t, spec, "CreateCheckpointOption type must be declared")
}

func TestCheckpointCreateShape_WithAllowLegacyImport(t *testing.T) {
	file := parseOptionalSource(t, "checkpoint_create_options.go")
	require.NotNil(t, file, "checkpoint_create_options.go must exist and parse")
	var fn *ast.FuncDecl
	for _, d := range file.Decls {
		if f, ok := d.(*ast.FuncDecl); ok && f.Recv == nil && f.Name.Name == "WithAllowLegacyImport" {
			fn = f
		}
	}
	require.NotNil(t, fn, "WithAllowLegacyImport constructor must be declared")
	require.Zero(t, fn.Type.Params.NumFields(), "WithAllowLegacyImport takes no parameters")
	require.NotNil(t, fn.Type.Results)
	require.Equal(t, 1, fn.Type.Results.NumFields())
	id, ok := fn.Type.Results.List[0].Type.(*ast.Ident)
	require.True(t, ok, "result must be a plain identifier")
	require.Equal(t, "CreateCheckpointOption", id.Name)
}

func TestCheckpointCreateShape_VariadicCreateCheckpoint(t *testing.T) {
	file := parseEventsSource(t, "memory.go")
	var fn *ast.FuncDecl
	for _, d := range file.Decls {
		if f, ok := d.(*ast.FuncDecl); ok && f.Recv == nil && f.Name.Name == "CreateCheckpoint" {
			fn = f
		}
	}
	require.NotNil(t, fn, "CreateCheckpoint must exist in memory.go")
	params := fn.Type.Params.List
	require.NotEmpty(t, params)
	last := params[len(params)-1]
	ell, ok := last.Type.(*ast.Ellipsis)
	require.True(t, ok, "CreateCheckpoint must end with a variadic parameter")
	id, ok := ell.Elt.(*ast.Ident)
	require.True(t, ok)
	require.Equal(t, "CreateCheckpointOption", id.Name)
	total := 0
	for _, p := range params {
		n := len(p.Names)
		if n == 0 {
			n = 1
		}
		total += n
	}
	require.Equal(t, 4, total, "signature is (ctx, dir, dump, opts...)")
}

func TestCheckpointCreateShape_Sentinels(t *testing.T) {
	file := parseOptionalSource(t, "../errors/checkpoint_errors.go")
	require.NotNil(t, file, "checkpoint_errors.go must parse")
	for _, name := range []string{"ErrCheckpointSchemaRejected", "ErrCheckpointSchemaUnsupported"} {
		require.NotNil(t, findPackageVarIn(file, name), "%s sentinel must be declared", name)
	}
}
