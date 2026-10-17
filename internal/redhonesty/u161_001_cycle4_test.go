package redhonesty

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestU161_003_StoreInterruptedWriteLeavesNoTornRecord(t *testing.T) {
	dir := t.TempDir()
	st, _ := NewStore(dir)
	// Simulate a crash after staging but before publication: a stray staging file
	// exists, the final name does not.
	taskDir := filepath.Join(dir, "161.003-T")
	if err := os.MkdirAll(taskDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(taskDir, ".evidence-deadbeefdeadbeef.tmp"), []byte("{trunc"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Load("161.003-T", u3Head); err == nil {
		t.Fatal("no record was published, Load must fail")
	}
	a, _ := BuildArtifact("161.003-T", u3Base, u3Head, []Verdict{honestVerdict("TestA")})
	if _, err := st.Write(a); err != nil {
		t.Fatalf("a stray staging file must not block publication: %v", err)
	}
	if _, err := st.Load("161.003-T", u3Head); err != nil {
		t.Fatal(err)
	}
	// Normal writes leave no staging files behind.
	entries, _ := os.ReadDir(taskDir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".evidence-") && e.Name() != ".evidence-deadbeefdeadbeef.tmp" {
			t.Errorf("staging file leaked: %s", e.Name())
		}
	}
}

func TestU161_001_TestdataGoFixturesAreNotSourcesOrTests(t *testing.T) {
	changes := []FileChange{
		{Path: "p/testdata/bad.go", Status: 'A', Head: []byte("package p\nfunc broken(")},
		{Path: "p/testdata/fixture_test.go", Status: 'A', Head: []byte("package p\nimport \"testing\"\nfunc TestFixture(t *testing.T) {}\n")},
		{Path: "testdata/root.go", Status: 'A', Head: []byte("not go at all")},
	}
	if _, err := ChangedSymbols(changes); err != nil {
		t.Fatalf("malformed Go fixture under testdata aborted symbol analysis: %v", err)
	}
	got, err := IdentifyNewTests(changes)
	if err != nil || len(got) != 0 {
		t.Fatalf("fixture test function scheduled for execution: %v %v", got, err)
	}
	// They are still overlaid so the baseline sees head's fixtures.
	dir := t.TempDir()
	if err := applyOverlay(dir, changes); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"p/testdata/bad.go", "p/testdata/fixture_test.go", "testdata/root.go"} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(f))); err != nil {
			t.Errorf("%s missing from overlay", f)
		}
	}
}

func TestU161_002_SharedHelperDepthIndependentOfCallOrder(t *testing.T) {
	mk := func(first, second string) string {
		return `package p
import "testing"
func leaf() int { return Compute(1) }
func shared() int { return leaf() }
func deep() int { return shared() }
func mid() int { return deep() }
func entry() int { return mid() }
func TestX(t *testing.T) {
	_ = ` + first + `()
	_ = ` + second + `()
}
`
	}
	s := syms([]string{"Compute"}, nil, nil, nil)
	for _, order := range [][2]string{{"entry", "shared"}, {"shared", "entry"}} {
		wantRejections(t, evalOne(t, mk(order[0], order[1]), ResultFail, ResultPass, s))
	}
}
