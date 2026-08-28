package events

import (
	"context"
	"go/ast"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// U14 — Resolve-verb caller migration onto the guarded seam (147-F /
// 147.037-T). ResolveCheckpoint must stop calling syncWriteFileAtomic
// directly and instead call RewriteCheckpointFile with its existing
// mutation closure. This also turns U3c's (147.042-T) already-red
// TestU3c_ResolveRefusesValidNonConformingDocument green.

// TestU14_ResolveCallsRewriteSeamNotDirectAtomicWrite is a structural
// assertion: ResolveCheckpoint's function body must contain no
// syncWriteFileAtomic call and must contain a RewriteCheckpointFile call.
func TestU14_ResolveCallsRewriteSeamNotDirectAtomicWrite(t *testing.T) {
	file := parseEventsSource(t, "checkpoint_lifecycle.go")
	funcDecl := findPackageFuncIn(file, "ResolveCheckpoint")
	require.NotNil(t, funcDecl, "ResolveCheckpoint must be declared in checkpoint_lifecycle.go")

	var callsDirectWrite, callsSeam bool
	ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if ident, ok := call.Fun.(*ast.Ident); ok {
			if ident.Name == "syncWriteFileAtomic" {
				callsDirectWrite = true
			}
			if ident.Name == "RewriteCheckpointFile" {
				callsSeam = true
			}
		}
		return true
	})
	assert.False(t, callsDirectWrite, "ResolveCheckpoint must not call syncWriteFileAtomic directly")
	assert.True(t, callsSeam, "ResolveCheckpoint must call RewriteCheckpointFile")
}

// TestU14Guard_ResolveAcceptPathUnchanged is the shipped-accept-path guard:
// ResolveCheckpoint on a conforming active document still resolves.
func TestU14Guard_ResolveAcceptPathUnchanged(t *testing.T) {
	dir := t.TempDir()
	cp := validCheckpointV1()
	cp.Status = "active"
	name := "checkpoint-u14-accept.json"
	writeTestCheckpointNamed(t, dir, name, cp)

	err := ResolveCheckpoint(context.Background(), dir, name)
	require.NoError(t, err)

	result, err := GetCheckpoint(context.Background(), dir, name)
	require.NoError(t, err)
	assert.Equal(t, "resolved", result.Status)
	assert.WithinDuration(t, time.Now().UTC(), result.UpdatedAt, 10*time.Second)
}
