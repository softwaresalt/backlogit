package canonical

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGovernedSha256Allowlist is a baseline/allowlist structural guard. It
// rejects NEW ad hoc crypto/sha256 hashing sites on governed gate-evidence
// payload paths, while allowlisting the ONE pre-existing legacy seam until F1.
//
// F1 removes internal/core/gate_evidence.go from this allowlist when it
// re-routes gateReportHash through internal/canonical (adding a hash-scheme
// version field). New hashing on governed paths MUST route through
// internal/canonical instead of importing crypto/sha256 directly.
//
// NOTE: internal/cli/self_update.go also imports crypto/sha256 (for release
// binary checksums) but is OUTSIDE governed gate-evidence paths, so it is
// intentionally NOT scanned and is NOT part of this allowlist.
func TestGovernedSha256Allowlist(t *testing.T) {
	pkgDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	root := repoRootFrom(t, pkgDir)

	// Governed directories scanned recursively for direct crypto/sha256 imports.
	governedDirs := []string{
		filepath.Join("internal", "core"),
		filepath.Join("internal", "gateevidence"),
	}

	// Allowlist of governed files permitted to import crypto/sha256 directly
	// (forward-slash relative paths from the repo root). F1 empties this set.
	allowlist := map[string]bool{
		"internal/core/gate_evidence.go": true,
	}

	fset := token.NewFileSet()
	var offenders []string

	for _, gd := range governedDirs {
		absDir := filepath.Join(root, gd)
		if _, err := os.Stat(absDir); err != nil {
			continue
		}
		walkErr := filepath.WalkDir(absDir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			name := d.Name()
			if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				return nil
			}
			file, perr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
			if perr != nil {
				return perr
			}
			for _, imp := range file.Imports {
				if strings.Trim(imp.Path.Value, `"`) != "crypto/sha256" {
					continue
				}
				rel, rerr := filepath.Rel(root, path)
				if rerr != nil {
					return rerr
				}
				relSlash := filepath.ToSlash(rel)
				if !allowlist[relSlash] {
					offenders = append(offenders, relSlash)
				}
			}
			return nil
		})
		if walkErr != nil {
			t.Fatalf("walk %s: %v", gd, walkErr)
		}
	}

	if len(offenders) > 0 {
		t.Errorf("governed files import crypto/sha256 directly and are NOT allowlisted: %v\n"+
			"route new gate-evidence hashing through internal/canonical (Canonicalize/Hash) instead of importing crypto/sha256",
			offenders)
	}
}
