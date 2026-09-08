package faultline

// TestU4aDeclSignatureShape is the permanent regression guard for the
// internal/faultline contract function signatures (156.004-T + 156.006-T).
// Originally a red-deliverable harness, it is now GREEN: all six required
// contract functions (Canonical, DecodeAndValidate, Validate, RegisterFamily,
// knownFamilies, freeze) exist in evidence.go with full implementations.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
)

// TestU4aDeclSignatureShape parses evidence.go and asserts the presence of the
// six contract functions/methods by name. It is GREEN: all six functions exist
// with full implementations as delivered by 156.006-T.
func TestU4aDeclSignatureShape(t *testing.T) {
	src := evidenceSourcePath(t)

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, src, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse %s: %v", src, err)
	}

	found := map[string]bool{}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		found[fn.Name.Name] = true
	}

	required := []string{
		"Canonical",
		"DecodeAndValidate",
		"Validate",
		"RegisterFamily",
		"knownFamilies",
		"freeze",
	}

	var missing []string
	for _, name := range required {
		if !found[name] {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)

	if len(missing) > 0 {
		t.Errorf("evidence.go is missing required contract function declarations %v; "+
			"these are implemented by 156.006-T (this harness is a red deliverable)", missing)
	}
}

// evidenceSourcePath resolves the absolute path to evidence.go relative to this
// test file so the harness is independent of the working directory.
func evidenceSourcePath(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed to locate the test file")
	}
	return filepath.Join(filepath.Dir(thisFile), "evidence.go")
}
