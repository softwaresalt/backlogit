package events

import (
	"go/ast"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestU11_RewriteSeamDeclared is a source-shape harness (147-F / U11,
// cycle-31): checkpoint_rewrite.go does not yet exist, so parsing it fails
// with a file-not-found error rather than a Go build error. It asserts
// RewriteCheckpointFile is declared with the exact signature:
// func RewriteCheckpointFile(ctx context.Context, checkpointDir, filename
// string, mutate func(*CheckpointV1) error) error.
func TestU11_RewriteSeamDeclared(t *testing.T) {
	file, err := tryParseEventsSource(t, "checkpoint_rewrite.go")
	if err != nil {
		t.Fatalf("checkpoint_rewrite.go is not declared yet: %v", err)
		return
	}
	funcDecl := findPackageFuncIn(file, "RewriteCheckpointFile")
	if !assert.NotNil(t, funcDecl, "RewriteCheckpointFile is not declared in checkpoint_rewrite.go") {
		return
	}
	assert.Equal(t, 4, countParams(funcDecl.Type.Params), "RewriteCheckpointFile must take exactly four parameters")
	if assert.Len(t, funcDecl.Type.Results.List, 1, "RewriteCheckpointFile must return exactly one value") {
		resultType, ok := funcDecl.Type.Results.List[0].Type.(*ast.Ident)
		assert.True(t, ok && resultType.Name == "error", "RewriteCheckpointFile must return error")
	}
	// The fourth parameter must be a func(*CheckpointV1) error.
	if len(funcDecl.Type.Params.List) > 0 {
		last := funcDecl.Type.Params.List[len(funcDecl.Type.Params.List)-1]
		_, isFuncType := last.Type.(*ast.FuncType)
		assert.True(t, isFuncType, "RewriteCheckpointFile's mutate parameter must be a func type")
	}
}

// TestU11Guard_RewriteSeamReachableFromCore asserts RewriteCheckpointFile is
// an exported package-level function reachable from internal/core (compile
// assertion via source-shape: exported name, non-nil signature).
func TestU11Guard_RewriteSeamReachableFromCore(t *testing.T) {
	file, err := tryParseEventsSource(t, "checkpoint_rewrite.go")
	if err != nil {
		t.Fatalf("checkpoint_rewrite.go is not declared yet: %v", err)
		return
	}
	funcDecl := findPackageFuncIn(file, "RewriteCheckpointFile")
	if !assert.NotNil(t, funcDecl) {
		return
	}
	assert.True(t, funcDecl.Name.IsExported(), "RewriteCheckpointFile must be exported")
}
