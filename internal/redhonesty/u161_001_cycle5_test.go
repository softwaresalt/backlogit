package redhonesty

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestU161_001_IdentifyNewTests_InternalAndExternalPackagesAreAmbiguous(t *testing.T) {
	internal := "package p\nimport \"testing\"\nfunc TestX(t *testing.T) {}\n"
	external := "package p_test\nimport \"testing\"\nfunc TestX(t *testing.T) {}\n"

	// Existing internal test in a MODIFIED file; new external test of the same name.
	_, err := IdentifyNewTests([]FileChange{
		{Path: "p/a_test.go", Status: 'M', Base: []byte(internal), Head: []byte(internal + "// touched\n")},
		{Path: "p/b_test.go", Status: 'A', Head: []byte(external)},
	})
	if err == nil {
		t.Fatal("same test name in internal and external packages (modified file) was silently accepted/omitted")
	}
	// Two new declarations in different packages of one directory.
	_, err = IdentifyNewTests([]FileChange{
		{Path: "p/a_test.go", Status: 'A', Head: []byte(internal)},
		{Path: "p/b_test.go", Status: 'A', Head: []byte(external)},
	})
	if err == nil {
		t.Fatal("two new same-name tests in different packages accepted")
	}
	// Same name in different DIRECTORIES is fine.
	got, err := IdentifyNewTests([]FileChange{
		{Path: "p/a_test.go", Status: 'A', Head: []byte(internal)},
		{Path: "q/b_test.go", Status: 'A', Head: []byte(external)},
	})
	if err != nil || len(got) != 2 {
		t.Fatalf("got %+v err %v", got, err)
	}
}

func TestU161_001_ParseGoTestJSON_DuplicateTerminalEventsAreUnverifiable(t *testing.T) {
	// An unchanged internal TestX plus a new external TestX both run: the second
	// terminal event must not overwrite or merge with the first.
	in := strings.Join([]string{
		`{"Action":"fail","Package":"m/p","Test":"TestX"}`,
		`{"Action":"pass","Package":"m/p","Test":"TestX"}`,
		`{"Action":"pass","Package":"m/p","Test":"TestY"}`,
	}, "\n")
	got, err := parseGoTestJSON(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	if got["TestX"] != ResultUnrunnable || got["TestY"] != ResultPass {
		t.Fatalf("got %v", got)
	}
}

func TestU161_001_PrepareBaseline_ChecksTheActualAllocatedDirectory(t *testing.T) {
	root := newRepo(t, false)
	// Ignores only the placeholder probe name, not the allocated base-* directory.
	writeFile(t, root, ".gitignore", ".backlogit/runtime/redhonesty/baseline-probe/\n")
	gitCmd(t, root, "add", "-A")
	gitCmd(t, root, "commit", "-q", "-m", "selective ignore")
	base := gitCmd(t, root, "rev-parse", "HEAD")
	if _, _, err := PrepareBaseline(context.Background(), ExecGit{}, root, base, nil); err == nil {
		t.Fatal("baseline accepted although its allocated directory is not git-ignored")
	}
	entries, _ := os.ReadDir(filepath.Join(root, ".backlogit", "runtime", "redhonesty"))
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "base-") {
			t.Fatalf("refused baseline left %s behind", e.Name())
		}
	}
}

func TestU161_001_GoEnvFileCannotInjectFlags(t *testing.T) {
	if testing.Short() {
		t.Skip("runs go test")
	}
	envFile := filepath.Join(t.TempDir(), "goenv")
	if err := os.WriteFile(envFile, []byte("GOFLAGS=-tags=sentinel\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOENV", envFile)
	mod := t.TempDir()
	writeFile(t, mod, "go.mod", "module example.com/m\n\ngo 1.24\n")
	writeFile(t, mod, "p/on.go", "//go:build sentinel\n\npackage p\n\nconst tagOn = true\n")
	writeFile(t, mod, "p/off.go", "//go:build !sentinel\n\npackage p\n\nconst tagOn = false\n")
	writeFile(t, mod, "p/p_test.go", "package p\n\nimport \"testing\"\n\nfunc TestTag(t *testing.T) {\n\tif tagOn {\n\t\tt.Fatal(\"saved GOFLAGS leaked into the run\")\n\t}\n}\n")
	res, err := GoTestExecutor{}.Run(context.Background(), mod, "p", []string{"TestTag"})
	if err != nil {
		t.Fatal(err)
	}
	if res["TestTag"] != ResultPass {
		t.Fatalf("result = %v; GOENV-saved flags changed what ran", res)
	}
}
