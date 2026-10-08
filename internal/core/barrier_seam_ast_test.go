package core

import (
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

// TestU172_ItemLogBarrierSetterUnreachableFromProduction keeps the C barrier
// seam nil in production: SetItemLogLockBarrierHookForTest is exported only so
// tests in other packages can install it, so no non-test file may call it.
func TestU172_ItemLogBarrierSetterUnreachableFromProduction(t *testing.T) {
	const setter = "SetItemLogLockBarrierHookForTest"
	wd, err := os.Getwd()
	require.NoError(t, err)
	moduleRoot := filepath.Join(wd, "..", "..")
	_, err = os.Stat(filepath.Join(moduleRoot, "go.mod"))
	require.NoError(t, err, "module root must contain go.mod")

	scanned := 0
	var offenders []string
	walkErr := filepath.WalkDir(moduleRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			name := entry.Name()
			if path != moduleRoot && (strings.HasPrefix(name, ".") || name == "vendor" || name == "testdata" || name == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		scanned++
		file := parseSourceFileForTest(t, path)
		ast.Inspect(file, func(node ast.Node) bool {
			if fn, ok := node.(*ast.FuncDecl); ok && fn.Name.Name == setter && fn.Recv == nil {
				// The declaration itself is allowed; still inspect its body.
				if fn.Body != nil {
					ast.Inspect(fn.Body, func(inner ast.Node) bool {
						if ident, ok := inner.(*ast.Ident); ok && ident.Name == setter {
							offenders = append(offenders, path)
						}
						return true
					})
				}
				return false
			}
			if ident, ok := node.(*ast.Ident); ok && ident.Name == setter {
				offenders = append(offenders, path)
			}
			return true
		})
		return nil
	})
	require.NoError(t, walkErr)
	require.Positive(t, scanned, "scan must cover production Go files")
	assert.Empty(t, offenders, "production code must never reference %s", setter)
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
