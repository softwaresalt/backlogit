package events

import (
	"go/ast"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestU15_CheckpointReadResultDeclared asserts checkpoint_lifecycle.go
// declares CheckpointReadResult with the six fields U6b's production delta
// will populate (147-F / U15). This unit adds no conformance evaluation, no
// intent population, and no offender projection of its own.
func TestU15_CheckpointReadResultDeclared(t *testing.T) {
	file := parseEventsSource(t, "checkpoint_lifecycle.go")
	typeSpec := findPackageTypeIn(file, "CheckpointReadResult")
	if !assert.NotNil(t, typeSpec, "CheckpointReadResult struct is not declared in checkpoint_lifecycle.go") {
		return
	}
	structType, ok := typeSpec.Type.(*ast.StructType)
	if !assert.True(t, ok, "CheckpointReadResult must be a struct type") {
		return
	}
	want := map[string]bool{
		"Checkpoint": false, "Valid": false, "Conforming": false,
		"NeedsQuarantine": false, "RemediationIntent": false, "NonConformingFields": false,
	}
	for _, field := range structType.Fields.List {
		for _, name := range field.Names {
			if _, ok := want[name.Name]; ok {
				want[name.Name] = true
			}
		}
	}
	for name, found := range want {
		assert.True(t, found, "CheckpointReadResult is missing declared field %q", name)
	}
}

// TestU15_GetCheckpointResultDeclared asserts checkpoint_lifecycle.go
// countParams returns the total number of parameter names across a
// FieldList, accounting for grouped parameters of the same type
// (e.g. "checkpointDir, filename string" is one *ast.Field with two names).
func countParams(list *ast.FieldList) int {
	if list == nil {
		return 0
	}
	count := 0
	for _, field := range list.List {
		if len(field.Names) == 0 {
			count++ // unnamed parameter counts as one
			continue
		}
		count += len(field.Names)
	}
	return count
}

// declares GetCheckpointResult(ctx, checkpointDir, filename) (*CheckpointReadResult, error).
func TestU15_GetCheckpointResultDeclared(t *testing.T) {
	file := parseEventsSource(t, "checkpoint_lifecycle.go")
	funcDecl := findPackageFuncIn(file, "GetCheckpointResult")
	if !assert.NotNil(t, funcDecl, "GetCheckpointResult is not declared in checkpoint_lifecycle.go") {
		return
	}
	assert.Equal(t, 3, countParams(funcDecl.Type.Params), "GetCheckpointResult must take exactly three parameters")
	if assert.Len(t, funcDecl.Type.Results.List, 2, "GetCheckpointResult must return exactly two values") {
		resultType, ok := funcDecl.Type.Results.List[0].Type.(*ast.StarExpr)
		if assert.True(t, ok, "GetCheckpointResult's first result must be a pointer") {
			ident, ok := resultType.X.(*ast.Ident)
			assert.True(t, ok && ident.Name == "CheckpointReadResult",
				"GetCheckpointResult must return *CheckpointReadResult")
		}
	}
}

// TestU15Guard_GetCheckpointRetainedAsWrapper pins GetCheckpoint's existing
// signature unchanged: (context.Context, string, string) (*CheckpointV1,
// error). GetCheckpointResult wraps it; GetCheckpoint itself must not change
// so every existing caller compiles untouched. This assertion already holds
// today (GetCheckpoint is untouched by this unit), so it is a guard/pin
// rather than a red harness function.
func TestU15Guard_GetCheckpointRetainedAsWrapper(t *testing.T) {
	file := parseEventsSource(t, "checkpoint_lifecycle.go")
	funcDecl := findPackageFuncIn(file, "GetCheckpoint")
	if !assert.NotNil(t, funcDecl, "GetCheckpoint must still be declared in checkpoint_lifecycle.go") {
		return
	}
	if assert.Len(t, funcDecl.Type.Results.List, 2, "GetCheckpoint must still return exactly two values") {
		resultType, ok := funcDecl.Type.Results.List[0].Type.(*ast.StarExpr)
		if assert.True(t, ok, "GetCheckpoint's first result must be a pointer") {
			ident, ok := resultType.X.(*ast.Ident)
			assert.True(t, ok && ident.Name == "CheckpointV1", "GetCheckpoint must still return *CheckpointV1")
		}
	}
}
