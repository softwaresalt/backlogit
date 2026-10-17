package redhonesty

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/softwaresalt/backlogit/internal/faultline"
)

func TestU161_001_PrepareBaseline_IgnoresHostAutoCRLFAndIsByteFaithful(t *testing.T) {
	root := newRepo(t, true)
	// LF content committed; a host with core.autocrlf=true must not turn the
	// exported baseline into CRLF.
	writeFile(t, root, "p/golden.txt", "line1\nline2\n")
	gitCmd(t, root, "add", "-A")
	gitCmd(t, root, "commit", "-q", "-m", "golden")
	gitCmd(t, root, "config", "core.autocrlf", "true")
	base := gitCmd(t, root, "rev-parse", "HEAD")
	dir, cleanup, err := PrepareBaseline(context.Background(), ExecGit{}, root, base, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	b, err := os.ReadFile(filepath.Join(dir, "p", "golden.txt"))
	if err != nil || string(b) != "line1\nline2\n" {
		t.Fatalf("baseline bytes differ from the committed blob: %q %v", b, err)
	}
}

func TestU161_001_PrepareBaseline_RefusesContentChangingAttributes(t *testing.T) {
	root := newRepo(t, true)
	writeFile(t, root, "p/crlf.txt", "a\nb\n")
	writeFile(t, root, ".gitattributes", "p/crlf.txt text eol=crlf\n")
	gitCmd(t, root, "add", "-A")
	gitCmd(t, root, "commit", "-q", "-m", "eol")
	base := gitCmd(t, root, "rev-parse", "HEAD")
	if _, _, err := PrepareBaseline(context.Background(), ExecGit{}, root, base, nil); err == nil {
		t.Fatal("an export whose bytes differ from the committed blob was accepted as the baseline")
	}
}

func TestU161_002_ExpectedIndexedAndDereferencedBindingsAreCaught(t *testing.T) {
	s := syms([]string{"Compute"}, nil, nil, nil)
	for name, body := range map[string]string{
		"slice element": "want := make([]int, 1)\n\twant[0] = Compute(2)\n\t_ = want",
		"map element":   "want := map[string]int{}\n\twant[\"k\"] = Compute(2)\n\t_ = want",
		"deref":         "var w int\n\twant := &w\n\t*want = Compute(2)",
		"plural field":  "var tc struct{ wants []int }\n\ttc.wants[0] = Compute(2)",
		"paren":         "var want int\n\t(want) = Compute(2)",
	} {
		src := "package p\nimport \"testing\"\nfunc TestX(t *testing.T) {\n\t" + body + "\n}\n"
		v := evalOne(t, src, ResultFail, ResultPass, s)
		found := false
		for _, r := range v.Rejections {
			found = found || r == RejectBuggyPathExpected
		}
		if !found {
			t.Errorf("%s: rejections = %v", name, v.Rejections)
		}
	}
}

func TestU161_003_ValidTestNameMatchesProducerRule(t *testing.T) {
	for _, ok := range []string{"Test", "TestA", "Test_x", "Test1", "TestÄb"} {
		if !faultline.ValidTestName(ok) || !isTestName(ok) {
			t.Errorf("%q should be valid in both", ok)
		}
	}
	for _, bad := range []string{"Testfoo", "Tes", "test", "Test-x", "Test x", "Test²", "Test(x|y)", ""} {
		if faultline.ValidTestName(bad) {
			t.Errorf("%q accepted by the contract", bad)
		}
	}
}
