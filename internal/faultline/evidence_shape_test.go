package faultline

// 156.004-T (U4a-decl): RED AST source-shape harness.
//
// This harness is a RED DELIVERABLE. It parses the SOURCE TEXT of evidence.go
// with go/parser and go/ast and asserts that the fault-line evidence contract
// WILL declare the functions/methods Canonical, DecodeAndValidate, Validate,
// RegisterFamily, knownFamilies, and freeze.
//
// It deliberately does NOT reference any symbol from this package: the 156.004-T
// declarations task lands only body-free-compilable declarations, so those six
// functions do not exist yet. Parsing the source (instead of importing/using the
// symbols) lets the harness compile RIGHT NOW while staying RED until the
// behavior task (156.006-T) adds the function bodies, at which point it turns
// GREEN. This preserves the harness-architect declaration gate without stubs.

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
// six contract functions/methods by name. It is expected to FAIL (RED) until
// 156.006-T implements the function bodies.
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
