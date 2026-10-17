package redhonesty

import "testing"

// The go tool ignores files and directories whose names begin with "_" or ".".
// They are neither tests nor production sources, so they must not enter the
// base inventory (hiding an ignored-to-visible rename) or the new-test set
// (inheriting another declaration's results).
func TestU161_001_IgnoredNamesAreNeitherTestsNorSources(t *testing.T) {
	src := []byte("package p\nimport \"testing\"\nfunc TestFix(t *testing.T) {}\n")
	for _, f := range []string{"p/_fix_test.go", "p/.fix_test.go", "p/_x/fix_test.go", "p/.x/fix_test.go", "_p/fix_test.go"} {
		if isTestFile(f) || isGoSource(f) {
			t.Fatalf("%s must be ignored", f)
		}
		got, err := IdentifyNewTests([]FileChange{{Path: f, Status: 'A', Head: src}})
		if err != nil || len(got) != 0 {
			t.Fatalf("%s: got %v, %v; want no tests", f, got, err)
		}
	}
	// Ignored base declaration renamed to a visible file is a NEW test.
	got, err := IdentifyNewTests([]FileChange{
		{Path: "p/_fix_test.go", Status: 'D', Base: src},
		{Path: "p/fix_test.go", Status: 'A', Head: src},
	})
	if err != nil || len(got) != 1 || got[0].Name != "TestFix" {
		t.Fatalf("ignored-to-visible rename: got %v, %v", got, err)
	}
	// Ignored production file contributes no symbol delta.
	s, err := ChangedSymbols([]FileChange{{Path: "p/_gen.go", Status: 'A', Head: []byte("package p\nfunc Hidden() {}\n")}})
	if err != nil || len(s.Funcs) != 0 {
		t.Fatalf("ignored source leaked symbols: %v, %v", s.Funcs, err)
	}
	if !isTestFile("p/fix_test.go") || !isGoSource("p/x.go") {
		t.Fatal("visible files must still count")
	}
}
