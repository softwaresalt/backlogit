package canonical

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPackageImportsZeroInternal asserts that every production (non-_test.go)
// .go file in internal/canonical imports ZERO internal backlogit packages. This
// pins the leaf-package invariant so the canonical hash seam can be shared
// across the one-way core->db boundary without creating an import cycle.
func TestPackageImportsZeroInternal(t *testing.T) {
	const internalPrefix = "github.com/softwaresalt/backlogit/"

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}

	fset := token.NewFileSet()
	var scanned int
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		scanned++
		file, err := parser.ParseFile(fset, name, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, imp := range file.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			if strings.HasPrefix(path, internalPrefix) {
				t.Errorf("%s imports internal package %q; internal/canonical must remain a stdlib-only leaf (zero internal imports)", name, path)
			}
		}
	}

	if scanned == 0 {
		t.Fatalf("no production .go files scanned; expected at least canonical.go")
	}
}

// repoRootFrom walks up from dir until it finds a directory containing go.mod.
func repoRootFrom(t *testing.T, dir string) string {
	t.Helper()
	cur := dir
	for {
		if _, err := os.Stat(filepath.Join(cur, "go.mod")); err == nil {
			return cur
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			t.Fatalf("go.mod not found walking up from %s", dir)
		}
		cur = parent
	}
}
