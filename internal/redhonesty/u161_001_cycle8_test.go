package redhonesty

import (
	"strings"
	"testing"
)

// A new test in a file excluded or selected by build constraints cannot be
// attributed by package/name: an unchanged same-named test in another variant
// file could be the one that actually runs. Such a new test is unverifiable.
func TestU161_001_IdentifyNewTests_BuildConstrainedNewTestIsUnverifiable(t *testing.T) {
	body := "import \"testing\"\nfunc TestFix(t *testing.T) {}\n"
	cases := map[string]string{
		"p/fix_windows_test.go":     "package p\n" + body,
		"p/fix_linux_arm64_test.go": "package p\n" + body,
		"p/fix_tagged_test.go":      "//go:build integration\n\npackage p\n" + body,
		"p/fix_oldtag_test.go":      "// +build integration\n\npackage p\n" + body,
		"p/fix_other_test.go":       "// Copyright\n//go:build !linux\n\npackage p\n" + body,
	}
	for file, src := range cases {
		_, err := IdentifyNewTests([]FileChange{{Path: file, Status: 'A', Head: []byte(src)}})
		if err == nil || !strings.Contains(err.Error(), "build constraint") {
			t.Fatalf("%s: err = %v, want build-constraint refusal", file, err)
		}
	}
	// Unconstrained names that merely resemble constraints stay discoverable.
	for _, file := range []string{"p/fix_test.go", "p/fix_unix_test.go", "p/windows_fix_test.go", "p/fix_amd64x_test.go"} {
		got, err := IdentifyNewTests([]FileChange{{Path: file, Status: 'A', Head: []byte("package p\n" + body)}})
		if err != nil || len(got) != 1 {
			t.Fatalf("%s: got %v, %v; want one test", file, got, err)
		}
	}
	// A constrained file whose tests already existed at base is not new.
	got, err := IdentifyNewTests([]FileChange{{Path: "p/fix_windows_test.go", Status: 'M',
		Base: []byte("package p\n" + body), Head: []byte("package p\n" + body)}})
	if err != nil || len(got) != 0 {
		t.Fatalf("existing constrained test: got %v, %v", got, err)
	}
}
