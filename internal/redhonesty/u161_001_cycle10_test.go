package redhonesty

import (
	"os"
	"path/filepath"
	"testing"
)

// Overlay eligibility is narrower than discovery exclusion: hidden or
// underscore production inputs (p/.config) are production state and must stay
// at baseline contents, while real test files and explicit testdata (even
// hidden files inside testdata) are overlaid with head contents.
func TestU161_001_OverlayLeavesHiddenProductionInputsAtBaseline(t *testing.T) {
	dir := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("p/.config", "base")
	write("p/_data.txt", "base")
	write("p/.gone", "base")
	write("p/testdata/.fixture", "base")
	err := applyOverlay(dir, []FileChange{
		{Path: "p/.config", Status: 'M', Base: []byte("base"), Head: []byte("head")},
		{Path: "p/_data.txt", Status: 'M', Base: []byte("base"), Head: []byte("head")},
		{Path: "p/.gone", Status: 'D', Base: []byte("base")},
		{Path: "p/testdata/.fixture", Status: 'M', Base: []byte("base"), Head: []byte("head")},
		{Path: "p/new_test.go", Status: 'A', Head: []byte("package p\n")},
	})
	if err != nil {
		t.Fatal(err)
	}
	read := func(rel string) string {
		b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
		return string(b)
	}
	for _, rel := range []string{"p/.config", "p/_data.txt", "p/.gone"} {
		if got := read(rel); got != "base" {
			t.Fatalf("%s overlaid with %q; production inputs must stay at baseline", rel, got)
		}
	}
	if got := read("p/testdata/.fixture"); got != "head" {
		t.Fatalf("testdata fixture = %q; want head", got)
	}
	if got := read("p/new_test.go"); got != "package p\n" {
		t.Fatalf("test file not overlaid: %q", got)
	}
	if isOverlayPath("p/.config") || !isOverlayPath("p/testdata/.x") || !isOverlayPath("p/a_test.go") {
		t.Fatal("overlay predicate wrong")
	}
}
