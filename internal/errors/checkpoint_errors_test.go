package errors

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// parseCheckpointErrorsSource parses internal/errors/checkpoint_errors.go and
// returns its AST for source-shape assertions. U1's harness must not
// reference any symbol declared by this unit's own delta, so it inspects the
// file's syntax tree instead of importing the not-yet-declared identifiers.
// That is what lets this test compile against the pre-declaration tree and
// fail on an assertion rather than a build error.
func parseCheckpointErrorsSource(t *testing.T) *ast.File {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "checkpoint_errors.go", nil, parser.AllErrors)
	require.NoError(t, err, "checkpoint_errors.go must parse as valid Go source")
	return file
}

func findPackageVar(file *ast.File, name string) *ast.ValueSpec {
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
			for _, n := range valueSpec.Names {
				if n.Name == name {
					return valueSpec
				}
			}
		}
	}
	return nil
}

func findPackageType(file *ast.File, name string) *ast.TypeSpec {
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}
		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			if typeSpec.Name.Name == name {
				return typeSpec
			}
		}
	}
	return nil
}

func findMethodOn(file *ast.File, receiverType, methodName string) *ast.FuncDecl {
	for _, decl := range file.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok || funcDecl.Recv == nil || len(funcDecl.Recv.List) == 0 {
			continue
		}
		if funcDecl.Name.Name != methodName {
			continue
		}
		recvType := funcDecl.Recv.List[0].Type
		if star, ok := recvType.(*ast.StarExpr); ok {
			if ident, ok := star.X.(*ast.Ident); ok && ident.Name == receiverType {
				return funcDecl
			}
			continue
		}
		if ident, ok := recvType.(*ast.Ident); ok && ident.Name == receiverType {
			return funcDecl
		}
	}
	return nil
}

func findPackageFunc(file *ast.File, name string) *ast.FuncDecl {
	for _, decl := range file.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok || funcDecl.Recv != nil {
			continue
		}
		if funcDecl.Name.Name == name {
			return funcDecl
		}
	}
	return nil
}

// TestU1_NonConformingSentinelDeclared asserts checkpoint_errors.go declares
// the ErrCheckpointNonConforming sentinel (U1, Q1).
func TestU1_NonConformingSentinelDeclared(t *testing.T) {
	file := parseCheckpointErrorsSource(t)
	spec := findPackageVar(file, "ErrCheckpointNonConforming")
	assert.NotNil(t, spec, "ErrCheckpointNonConforming is not declared in checkpoint_errors.go")
}

// TestU1_NonConformingErrorTypeDeclared asserts checkpoint_errors.go declares
// CheckpointNonConformingError{Fields []string} with Error() and Unwrap()
// error methods, mirroring CheckpointUnknownFieldError.
func TestU1_NonConformingErrorTypeDeclared(t *testing.T) {
	file := parseCheckpointErrorsSource(t)
	typeSpec := findPackageType(file, "CheckpointNonConformingError")
	if !assert.NotNil(t, typeSpec, "CheckpointNonConformingError is not declared in checkpoint_errors.go") {
		return
	}
	structType, ok := typeSpec.Type.(*ast.StructType)
	if !assert.True(t, ok, "CheckpointNonConformingError must be a struct type") {
		return
	}
	hasFields := false
	for _, field := range structType.Fields.List {
		arrayType, ok := field.Type.(*ast.ArrayType)
		if !ok || arrayType.Len != nil {
			continue
		}
		ident, ok := arrayType.Elt.(*ast.Ident)
		if !ok || ident.Name != "string" {
			continue
		}
		for _, name := range field.Names {
			if name.Name == "Fields" {
				hasFields = true
			}
		}
	}
	assert.True(t, hasFields, "CheckpointNonConformingError must declare a Fields []string field")
	assert.NotNil(t, findMethodOn(file, "CheckpointNonConformingError", "Error"),
		"CheckpointNonConformingError has no Error() method")
	assert.NotNil(t, findMethodOn(file, "CheckpointNonConformingError", "Unwrap"),
		"CheckpointNonConformingError has no Unwrap() method")
}

// TestU1_QuarantineIsRemedyDeclared asserts checkpoint_errors.go declares the
// exported QuarantineIsRemedy(err error) bool predicate (U1, Q1).
func TestU1_QuarantineIsRemedyDeclared(t *testing.T) {
	file := parseCheckpointErrorsSource(t)
	funcDecl := findPackageFunc(file, "QuarantineIsRemedy")
	if !assert.NotNil(t, funcDecl, "QuarantineIsRemedy is not declared in checkpoint_errors.go") {
		return
	}
	if assert.Len(t, funcDecl.Type.Params.List, 1, "QuarantineIsRemedy must take exactly one parameter") {
		paramType, ok := funcDecl.Type.Params.List[0].Type.(*ast.Ident)
		assert.True(t, ok && paramType.Name == "error", "QuarantineIsRemedy's parameter must be of type error")
	}
	if assert.Len(t, funcDecl.Type.Results.List, 1, "QuarantineIsRemedy must return exactly one value") {
		resultType, ok := funcDecl.Type.Results.List[0].Type.(*ast.Ident)
		assert.True(t, ok && resultType.Name == "bool", "QuarantineIsRemedy must return bool")
	}
}
