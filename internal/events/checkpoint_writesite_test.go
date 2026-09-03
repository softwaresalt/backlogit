package events_test

// U2f — Supplemental caller-set regression guard for the guarded seam
// (147-F / 147.021-T, harness-exempt: verification-only). No production
// change: the enumeration this unit asserts lives inside the test file
// itself. This is a supplemental regression guard, not the authoritative I1
// mechanism — that is architectural (U11/U12/U13/U14/U14b make one guarded
// seam the only in-place rewrite path). It proves no direct, statically
// resolvable atomic-write call to the checkpoint directory was added
// outside the seam; it does not prove the absence of an indirect or
// dynamically dispatched writer.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// checkpointWriteCall names one direct-write call site found by the
// enumeration below.
type checkpointWriteCall struct {
	file string
	fn   string
	call string
}

// checkpointDirWriteScanExclusions names source files that declare
// "Checkpoint"-named functions but belong to an unrelated, pre-existing
// subsystem rather than the 147-F session/memory checkpoint disposition
// surface. hook_checkpoint.go implements per-consumer hook ack-position
// tracking under .backlogit/runtime/hooks/ (CheckpointStore.SaveCheckpoint)
// — a naming collision with, not a member of, the checkpoint directory this
// guard is scoped to.
var checkpointDirWriteScanExclusions = map[string]bool{
	"hook_checkpoint.go": true,
}

// enumerateCheckpointDirWriteCalls walks every .go (non-test, non-excluded)
// file in dir and returns one entry per call to syncWriteFileAtomic,
// atomicfile.WriteFileAtomic, atomicfile.WriteFileAtomicWithOptions, or
// os.WriteFile found inside any function whose name contains "Checkpoint"
// — the closed, discoverable proxy this supplemental guard uses for
// "resolves under the checkpoint directory".
func enumerateCheckpointDirWriteCalls(t *testing.T, dir string) []checkpointWriteCall {
	t.Helper()
	var found []checkpointWriteCall

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || filepath.Ext(name) != ".go" || len(name) > 8 && name[len(name)-8:] == "_test.go" {
			continue
		}
		if checkpointDirWriteScanExclusions[name] {
			continue
		}
		fset := token.NewFileSet()
		path := filepath.Join(dir, name)
		file, parseErr := parser.ParseFile(fset, path, nil, parser.AllErrors)
		require.NoError(t, parseErr)

		for _, decl := range file.Decls {
			funcDecl, ok := decl.(*ast.FuncDecl)
			if !ok || funcDecl.Body == nil {
				continue
			}
			if !containsSubstring(funcDecl.Name.Name, "Checkpoint") {
				continue
			}
			ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				switch fn := call.Fun.(type) {
				case *ast.Ident:
					if fn.Name == "syncWriteFileAtomic" || fn.Name == "syncWriteFileAtomicHook" {
						found = append(found, checkpointWriteCall{file: name, fn: funcDecl.Name.Name, call: fn.Name})
					}
				case *ast.SelectorExpr:
					pkgIdent, ok := fn.X.(*ast.Ident)
					if !ok {
						return true
					}
					if (pkgIdent.Name == "atomicfile" && (fn.Sel.Name == "WriteFileAtomic" || fn.Sel.Name == "WriteFileAtomicWithOptions")) ||
						(pkgIdent.Name == "os" && fn.Sel.Name == "WriteFile") {
						found = append(found, checkpointWriteCall{file: name, fn: funcDecl.Name.Name, call: pkgIdent.Name + "." + fn.Sel.Name})
					}
				}
				return true
			})
		}
	}
	return found
}

func containsSubstring(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// checkpointDirWriteAllowlist is the post-migration allow-list: after U14
// and U14b, the only direct writes touching the checkpoint directory are
// the seam's own atomic replace (RewriteCheckpointFile, routed through
// atomicfile.WriteFileAtomicWithOptions with DurableWrites: true since the
// 130-S adversarial-review mode/durability fix) and the excluded
// verbatim-move / create sites (CreateCheckpoint's new-file write and
// QuarantineCheckpoint's disposition-sidecar write), none of which are gated
// by this seam by design (147-F / U11 scope boundary).
//
// After 153.002-T (S1 U2), QuarantineCheckpoint's disposition-sidecar write
// uses writeDispositionSidecarCreateOnly, whose function name does not contain
// "Checkpoint" and is therefore not enumerated by this scan — the helper is
// explicitly excluded from the seam-guard coverage, matching the existing
// verbatim-move exclusion.
//
// moveNoReplace and CleanupCheckpoints use os.Link/os.Rename rather than any
// of the three enumerated write forms, so they never appear in this set.
var checkpointDirWriteAllowlist = map[string]bool{
	"checkpoint_rewrite.go:RewriteCheckpointFile:atomicfile.WriteFileAtomicWithOptions": true,
	"memory.go:CreateCheckpoint:syncWriteFileAtomicHook":                                true,
}

// TestU2fGuard_EnumeratedCallSiteSetEqualsAllowlist asserts the enumerated
// call-site set across internal/events and internal/core equals the
// post-migration allow-list.
func TestU2fGuard_EnumeratedCallSiteSetEqualsAllowlist(t *testing.T) {
	calls := enumerateCheckpointDirWriteCalls(t, "../../internal/events")
	calls = append(calls, enumerateCheckpointDirWriteCalls(t, "../../internal/core")...)
	got := map[string]bool{}
	for _, c := range calls {
		key := c.file + ":" + c.fn + ":" + c.call
		got[key] = true
	}
	assert.Equal(t, checkpointDirWriteAllowlist, got)
}

// TestU2fGuard_SyntheticUngatedRewriteSiteFailsAssertion proves the
// enumeration is load-bearing: a synthetic ungated rewrite site (simulated
// as an unexpected entry) fails the equality assertion above rather than
// being silently absorbed.
func TestU2fGuard_SyntheticUngatedRewriteSiteFailsAssertion(t *testing.T) {
	synthetic := map[string]bool{
		"checkpoint_rewrite.go:RewriteCheckpointFile:atomicfile.WriteFileAtomicWithOptions": true,
		"memory.go:CreateCheckpoint:syncWriteFileAtomicHook":                                true,
		"checkpoint_evil.go:EvilCheckpointRewrite:syncWriteFileAtomic":                      true,
	}
	assert.NotEqual(t, checkpointDirWriteAllowlist, synthetic,
		"an injected ungated rewrite site must not match the allow-list")
}

// TestRewriteCheckpointFile_RequestsDurableWrites is a regression test
// (found during 130-S adversarial review): switching from syncWriteFileAtomic
// (which always fsyncs before rename) to atomicfile.WriteFileAtomic (the
// durable-off fast path, no fsync) silently regressed the seam's durability
// guarantee — a "successful" resolve/abandon could be lost after a crash or
// power failure before the OS itself flushed the rename to disk.
// atomicfile.WriteFileAtomicWithOptions does not expose an injectable fsync
// seam through its public API (that hook is package-private, used only by
// atomicfile's own tests), so this guard statically parses the seam's source
// and asserts its write call is WriteFileAtomicWithOptions with a
// DurableWrites: true field, not the bare durable-off WriteFileAtomic.
func TestRewriteCheckpointFile_RequestsDurableWrites(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "checkpoint_rewrite.go", nil, parser.AllErrors)
	require.NoError(t, err)

	var found bool
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkgIdent, ok := sel.X.(*ast.Ident)
		if !ok || pkgIdent.Name != "atomicfile" {
			return true
		}
		if sel.Sel.Name != "WriteFileAtomicWithOptions" {
			return true
		}
		require.Len(t, call.Args, 3, "WriteFileAtomicWithOptions must be called with (path, data, opts)")
		lit, ok := call.Args[2].(*ast.CompositeLit)
		require.True(t, ok, "the third argument must be an atomicfile.Options composite literal")
		for _, elt := range lit.Elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			key, ok := kv.Key.(*ast.Ident)
			if !ok || key.Name != "DurableWrites" {
				continue
			}
			value, ok := kv.Value.(*ast.Ident)
			if ok && value.Name == "true" {
				found = true
			}
		}
		return true
	})

	assert.True(t, found,
		"RewriteCheckpointFile must call atomicfile.WriteFileAtomicWithOptions with DurableWrites: true, "+
			"not the durable-off WriteFileAtomic fast path")
}
