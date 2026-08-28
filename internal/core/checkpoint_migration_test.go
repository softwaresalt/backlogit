package core

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// U14b — Abandon-verb caller migration onto the guarded seam (147-F /
// 147.044-T). AbandonCheckpoint must stop calling atomicfile.WriteFileAtomic
// for its checkpoint-rewrite step and instead call
// events.RewriteCheckpointFile with its existing mutation closure.

// TestU14b_AbandonCallsRewriteSeamNotDirectAtomicWrite is a structural
// assertion: AbandonCheckpoint's function body must contain no
// atomicfile.WriteFileAtomic call and must contain an
// events.RewriteCheckpointFile call.
func TestU14b_AbandonCallsRewriteSeamNotDirectAtomicWrite(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "checkpoint_disposition.go", nil, parser.AllErrors)
	require.NoError(t, err)

	var funcDecl *ast.FuncDecl
	for _, decl := range file.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if ok && fd.Name.Name == "AbandonCheckpoint" {
			funcDecl = fd
			break
		}
	}
	require.NotNil(t, funcDecl, "AbandonCheckpoint must be declared in checkpoint_disposition.go")

	var callsDirectWrite, callsSeam bool
	ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkgIdent, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}
		if pkgIdent.Name == "atomicfile" && sel.Sel.Name == "WriteFileAtomic" {
			callsDirectWrite = true
		}
		if pkgIdent.Name == "events" && sel.Sel.Name == "RewriteCheckpointFile" {
			callsSeam = true
		}
		return true
	})
	assert.False(t, callsDirectWrite, "AbandonCheckpoint must not call atomicfile.WriteFileAtomic directly")
	assert.True(t, callsSeam, "AbandonCheckpoint must call events.RewriteCheckpointFile")
}

// TestU14bGuard_AbandonAcceptPathUnchanged is the shipped-accept-path guard:
// AbandonCheckpoint on a conforming active document still abandons and still
// appends exactly one audit event.
func TestU14bGuard_AbandonAcceptPathUnchanged(t *testing.T) {
	ws := newCheckpointTargetTestWorkspace(t)
	dir := filepath.Join(ws.RootPath, ".backlogit", checkpointsSubdir)
	writeDispositionCheckpoint(t, dir, "checkpoint-u14b-accept.json", validDispositionTestCheckpoint())

	ew := newDispositionEventWriter(t, ws)
	require.NoError(t, AbandonCheckpoint(context.Background(), ws, ew, "checkpoint-u14b-accept.json", "test reason", "operator@example.com"))

	logPath := filepath.Join(ws.RootPath, ".backlogit", "logs", "checkpoint-disposition-audit.jsonl")
	data, err := os.ReadFile(logPath)
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	assert.Len(t, lines, 1, "exactly one audit event must be appended")
}
