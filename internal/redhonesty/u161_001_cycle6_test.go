package redhonesty

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestU161_001_PrepareBaseline_IgnoresGitReplacementObjects(t *testing.T) {
	root := newRepo(t, true)
	base := gitCmd(t, root, "rev-parse", "HEAD")
	writeFile(t, root, "p/p.go", "package p\n\nfunc Compute(x int) int { return 999 } // evil\n")
	gitCmd(t, root, "add", "-A")
	gitCmd(t, root, "commit", "-q", "-m", "evil")
	evil := gitCmd(t, root, "rev-parse", "HEAD")
	gitCmd(t, root, "replace", base, evil) // base now READS as the evil commit
	dir, cleanup, err := PrepareBaseline(context.Background(), ExecGit{}, root, base, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cleanup() }()
	b, err := os.ReadFile(filepath.Join(dir, "p", "p.go"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "evil") {
		t.Fatalf("a local replace ref changed the exported baseline: %q", b)
	}
}

type treeGit struct{ tree string }

func (g treeGit) Run(_ context.Context, _ string, w io.Writer, args ...string) error {
	if args[0] == "ls-tree" {
		_, err := w.Write([]byte(g.tree))
		return err
	}
	return nil
}

func TestU161_001_TreeBlobsRefusesSubmodules(t *testing.T) {
	sha := strings.Repeat("a", 40)
	ok := "100644 blob " + strings.Repeat("b", 40) + "\tp/p.go\x00"
	if _, err := treeBlobs(context.Background(), treeGit{ok}, ".", sha); err != nil {
		t.Fatalf("plain tree rejected: %v", err)
	}
	withSub := ok + "160000 commit " + strings.Repeat("c", 40) + "\tvendor/sub\x00"
	if _, err := treeBlobs(context.Background(), treeGit{withSub}, ".", sha); err == nil {
		t.Fatal("a submodule entry was silently excluded from export verification")
	}
}

func TestU161_003_HarnessTimeoutRequiresFailedRunAndHarnessDiagnostic(t *testing.T) {
	loud := []byte(`{"Action":"output","Test":"TestTimeoutDiagnostics","Output":"test timed out after 5s\n"}` + "\n")
	if harnessTimedOut(loud, false) {
		t.Fatal("a passing run that logs the phrase was treated as a timeout")
	}
	if harnessTimedOut(loud, true) {
		t.Fatal("logged phrase without the harness panic diagnostic was treated as a timeout")
	}
	real := []byte(`{"Action":"output","Output":"panic: test timed out after 9m55s\n"}` + "\n")
	if !harnessTimedOut(real, true) {
		t.Fatal("a genuine harness timeout was not detected")
	}
	if harnessTimedOut(real, false) {
		t.Fatal("a successful exit cannot be a harness timeout")
	}
}
