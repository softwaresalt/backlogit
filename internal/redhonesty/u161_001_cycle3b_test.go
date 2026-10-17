package redhonesty

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestU161_001_IdentifyNewTests_DotImportedAndAliasedTesting(t *testing.T) {
	got, err := IdentifyNewTests([]FileChange{
		{Path: "p/dot_test.go", Status: 'A', Head: []byte("package p\nimport . \"testing\"\nfunc TestDot(t *T) {}\nfunc TestQualifiedSibling(t *T) {}\n")},
		// `type T = testing.T` alias with a conventional test beside it: the go tool
		// discovers *T by name, so the alias test must not be silently omitted.
		{Path: "p/alias_test.go", Status: 'A', Head: []byte("package p\nimport \"testing\"\ntype T = testing.T\nfunc TestAlias(t *T) {}\nfunc TestPlain(t *testing.T) {}\n")},
		{Path: "p/notatest_test.go", Status: 'A', Head: []byte("package p\nimport \"testing\"\nfunc TestNotATest(b *testing.B) {}\nfunc TestTwoArgs(t *T, x int) {}\n")},
	})
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, n := range got {
		names[n.Name] = true
	}
	if !names["TestDot"] || !names["TestQualifiedSibling"] || !names["TestAlias"] || !names["TestPlain"] || names["TestNotATest"] || names["TestTwoArgs"] {
		t.Fatalf("got %+v", got)
	}
}

func TestU161_002_SharedHelperReachedAtShallowerDepthIsStillFollowed(t *testing.T) {
	// TestX -> a -> b -> c -> shared -> leaf -> Compute puts `shared` at depth 4,
	// where `leaf` is out of bound; the direct TestX -> shared call must still
	// reach Compute within the bound.
	src := `package p
import "testing"
func leaf() int { return Compute(1) }
func shared() int { return leaf() }
func c() int { return shared() }
func b() int { return c() }
func a() int { return b() }
func TestX(t *testing.T) {
	_ = a()
	_ = shared()
}
`
	v := evalOne(t, src, ResultFail, ResultPass, syms([]string{"Compute"}, nil, nil, nil))
	wantRejections(t, v) // exercises the changed code via the shallow path: honest
}

func TestU161_001_CollectChangesCarriesExecutableMode(t *testing.T) {
	b, h := strings.Repeat("a", 40), strings.Repeat("b", 40)
	g := scriptedGit{
		diff: ":000000 100755 0000000 1111111 A\x00p/testdata/run.sh\x00:100644 100755 1111111 2222222 M\x00p/testdata/mode_only.sh\x00:000000 100644 0000000 3333333 A\x00p/testdata/plain.txt\x00",
		blobs: map[string]string{
			h + ":p/testdata/run.sh": "#!/bin/sh\n", b + ":p/testdata/mode_only.sh": "x", h + ":p/testdata/mode_only.sh": "x",
			h + ":p/testdata/plain.txt": "x",
		},
	}
	got, err := CollectChanges(context.Background(), g, ".", b, h)
	if err != nil {
		t.Fatal(err)
	}
	exec := map[string]bool{}
	for _, c := range got {
		exec[c.Path] = c.Exec
	}
	if !exec["p/testdata/run.sh"] || !exec["p/testdata/mode_only.sh"] || exec["p/testdata/plain.txt"] {
		t.Fatalf("exec modes = %v", exec)
	}
}

func TestU161_001_OverlayAppliesExecutableModeIncludingModeOnlyChanges(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits")
	}
	dir := t.TempDir()
	writeFile(t, dir, "p/testdata/mode_only.sh", "x") // exists as 0644
	if err := applyOverlay(dir, []FileChange{
		{Path: "p/testdata/new.sh", Status: 'A', Head: []byte("#!/bin/sh\n"), Exec: true},
		{Path: "p/testdata/mode_only.sh", Status: 'M', Base: []byte("x"), Head: []byte("x"), Exec: true},
	}); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"new.sh", "mode_only.sh"} {
		fi, err := os.Stat(filepath.Join(dir, "p", "testdata", f))
		if err != nil || fi.Mode().Perm()&0o111 == 0 {
			t.Errorf("%s lost its executable bit: %v %v", f, fi, err)
		}
	}
}
