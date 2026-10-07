package core

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

func TestU172_BarrierSeamsExistAtBandCAcquisition(t *testing.T) {
	wd, err := os.Getwd()
	require.NoError(t, err)
	coreFile := parseSourceFileForTest(t, filepath.Join(wd, "shipment.go"))
	eventsFile := parseSourceFileForTest(t, filepath.Join(wd, "..", "events", "stream.go"))

	assert.True(t, fileDeclaresVar(coreFile, "artifactMutationLockBarrierHook"), "B acquisition hook variable must be declared")
	assert.True(t, funcReferencesIdentifier(coreFile, "lockArtifactMutation", "artifactMutationLockBarrierHook"), "lockArtifactMutation must invoke the B acquisition hook")
	assert.True(t, funcReferencesIdentifier(coreFile, "lockArtifactMutations", "artifactMutationLockBarrierHook"), "lockArtifactMutations must invoke the B acquisition hook")

	assert.True(t, fileDeclaresVar(eventsFile, "itemLogLockBarrierHook"), "C acquisition hook variable must be declared")
	assert.True(t, funcReferencesIdentifier(eventsFile, "acquireItemLogFileLock", "itemLogLockBarrierHook"), "acquireItemLogFileLock must invoke the C acquisition hook")
}

func parseSourceFileForTest(t *testing.T, path string) *ast.File {
	t.Helper()
	set := token.NewFileSet()
	file, err := parser.ParseFile(set, path, nil, parser.ParseComments)
	require.NoError(t, err)
	return file
}

func fileDeclaresVar(file *ast.File, name string) bool {
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.VAR {
			continue
		}
		for _, spec := range gen.Specs {
			valueSpec, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for _, declaredName := range valueSpec.Names {
				if declaredName.Name == name {
					return true
				}
			}
		}
	}
	return false
}

func funcReferencesIdentifier(file *ast.File, funcName, identName string) bool {
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != funcName || fn.Body == nil {
			continue
		}
		found := false
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			ident, ok := node.(*ast.Ident)
			if ok && ident.Name == identName {
				found = true
				return false
			}
			return true
		})
		return found
	}
	return false
}
