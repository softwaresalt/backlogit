package redhonesty

import (
	"archive/tar"
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func has(s map[string]struct{}, k string) bool { _, ok := s[k]; return ok }

func TestU161_001_IdentifyNewTests(t *testing.T) {
	changes := []FileChange{
		{Path: "internal/a/a_test.go", Status: 'A', Head: []byte(`package a
import "testing"
func TestA(t *testing.T) {}
func TestB(t *testing.T) {}
func helper(t *testing.T) {}
func TestMain(m *testing.M) {}
func Testlower(t *testing.T) {}
func TestWrongSig(x int) {}
func BenchmarkX(b *testing.B) {}
`)},
		{Path: "internal/b/b_test.go", Status: 'M',
			Base: []byte("package b\nimport \"testing\"\nfunc TestOld(t *testing.T) {}\n"),
			Head: []byte("package b\nimport \"testing\"\nfunc TestOld(t *testing.T) { t.Log(1) }\nfunc TestNew(t *testing.T) {}\n")},
		{Path: "internal/b/b.go", Status: 'M', Base: []byte("package b\n"), Head: []byte("package b\nfunc TestNotATestFile() {}\n")},
		{Path: "internal/c/c_test.go", Status: 'D', Base: []byte("package c\nfunc TestGone(t *testing.T) {}\n")},
		{Path: "root_test.go", Status: 'A', Head: []byte("package root\nimport \"testing\"\nfunc TestRoot(t *testing.T) {}\n")},
	}
	got, err := IdentifyNewTests(changes)
	if err != nil {
		t.Fatal(err)
	}
	want := []NewTest{
		{Package: ".", File: "root_test.go", Name: "TestRoot"},
		{Package: "internal/a", File: "internal/a/a_test.go", Name: "TestA"},
		{Package: "internal/a", File: "internal/a/a_test.go", Name: "TestB"},
		{Package: "internal/b", File: "internal/b/b_test.go", Name: "TestNew"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

func TestU161_001_IdentifyNewTests_FailsClosedOnUnparseable(t *testing.T) {
	_, err := IdentifyNewTests([]FileChange{{Path: "x/x_test.go", Status: 'A', Head: []byte("package x\nfunc TestA(")}})
	if err == nil {
		t.Fatal("expected parse error for unparseable test file")
	}
}

func TestU161_001_ChangedSymbols(t *testing.T) {
	base := `package p
type Row struct {
	A int
	B string
}
func Compute(x int) int { return x }
func Other() int { return 1 }
`
	head := `package p
type Row struct {
	A int
	B string
	C bool
}
func Compute(x int) int { return x + 1 }
func Other() int { return 1 }
func Added() {}
`
	s, err := ChangedSymbols([]FileChange{
		{Path: "p/p.go", Status: 'M', Base: []byte(base), Head: []byte(head)},
		{Path: "p/p_test.go", Status: 'M', Base: []byte("package p\nfunc TestX() {}\n"), Head: []byte("package p\nfunc TestY() {}\n")},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"Compute", "Added"} {
		if !has(s.Funcs, f) {
			t.Errorf("Funcs missing %s: %v", f, s.Funcs)
		}
	}
	for _, f := range []string{"Other", "TestX", "TestY"} {
		if has(s.Funcs, f) {
			t.Errorf("Funcs must not contain %s", f)
		}
	}
	if !has(s.Types, "Row") || !has(s.Fields, "C") {
		t.Errorf("Types/Fields wrong: %v %v", s.Types, s.Fields)
	}
	if has(s.Fields, "A") || !has(s.Siblings, "A") || !has(s.Siblings, "B") || has(s.Siblings, "C") {
		t.Errorf("Siblings wrong: fields=%v siblings=%v", s.Fields, s.Siblings)
	}
	if s.Empty() {
		t.Error("Empty() = true")
	}
}

func TestU161_001_ChangedSymbols_AddedAndDeletedFiles(t *testing.T) {
	s, err := ChangedSymbols([]FileChange{
		{Path: "n/n.go", Status: 'A', Head: []byte("package n\nfunc New() {}\n")},
		{Path: "n/gone.go", Status: 'D', Base: []byte("package n\nfunc Gone() {}\n")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !has(s.Funcs, "New") || !has(s.Funcs, "Gone") {
		t.Fatalf("Funcs = %v", s.Funcs)
	}
}

type countingGit struct {
	calls int
	out   string
}

func (g *countingGit) Run(_ context.Context, _ string, w io.Writer, _ ...string) error {
	g.calls++
	_, err := w.Write([]byte(g.out))
	return err
}

func TestU161_001_ResolveCommit(t *testing.T) {
	good := strings.Repeat("a1", 20)
	g := &countingGit{out: good + "\n"}
	sha, err := ResolveCommit(context.Background(), g, ".", "main")
	if err != nil || sha != good {
		t.Fatalf("sha=%q err=%v", sha, err)
	}
	for _, ref := range []string{"", "--output=x", "-h", "a b", "x\ny", "main\x00"} {
		g.calls = 0
		if _, err := ResolveCommit(context.Background(), g, ".", ref); err == nil {
			t.Errorf("ref %q accepted", ref)
		}
		if g.calls != 0 {
			t.Errorf("ref %q reached git", ref)
		}
	}
	bad := &countingGit{out: "not-a-sha\n"}
	if _, err := ResolveCommit(context.Background(), bad, ".", "main"); err == nil {
		t.Error("non-hex git output accepted")
	}
	short := &countingGit{out: "abc123\n"}
	if _, err := ResolveCommit(context.Background(), short, ".", "main"); err == nil {
		t.Error("abbreviated sha accepted; must be a full immutable SHA")
	}
}

func tarOf(t *testing.T, hdrs ...tar.Header) *bytes.Reader {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for i := range hdrs {
		h := hdrs[i]
		body := ""
		if h.Typeflag == tar.TypeReg {
			body = "x"
			h.Size = 1
		}
		if err := tw.WriteHeader(&h); err != nil {
			t.Fatal(err)
		}
		if body != "" {
			_, _ = tw.Write([]byte(body))
		}
	}
	_ = tw.Close()
	return bytes.NewReader(buf.Bytes())
}

func TestU161_001_ExtractArchiveContainment(t *testing.T) {
	for _, h := range []tar.Header{
		{Name: "../evil.go", Typeflag: tar.TypeReg, Mode: 0o644},
		{Name: "a/../../evil.go", Typeflag: tar.TypeReg, Mode: 0o644},
		{Name: "/abs.go", Typeflag: tar.TypeReg, Mode: 0o644},
		{Name: `C:\abs.go`, Typeflag: tar.TypeReg, Mode: 0o644},
		{Name: "C:/abs2.go", Typeflag: tar.TypeReg, Mode: 0o644},
		{Name: "pkg/dev", Typeflag: tar.TypeChar, Mode: 0o644},
	} {
		dir := t.TempDir()
		if err := extractArchive(tarOf(t, h), dir); err == nil {
			t.Errorf("entry %q (%c) accepted", h.Name, h.Typeflag)
		}
	}
	// Links are refused, not silently dropped: a baseline missing a linked
	// fixture would manufacture a failure the real base never had.
	for _, h := range []tar.Header{
		{Name: "link", Typeflag: tar.TypeSymlink, Linkname: "../outside", Mode: 0o777},
		{Name: "hard", Typeflag: tar.TypeLink, Linkname: "a.go", Mode: 0o644},
	} {
		ldir := t.TempDir()
		if err := extractArchive(tarOf(t, h), ldir); err == nil {
			t.Errorf("link entry %q accepted", h.Name)
		}
		if _, err := os.Lstat(filepath.Join(ldir, h.Name)); err == nil {
			t.Errorf("%s was materialized", h.Name)
		}
	}
	dir := t.TempDir()
	if err := extractArchive(tarOf(t,
		tar.Header{Name: "pkg/", Typeflag: tar.TypeDir, Mode: 0o755},
		tar.Header{Name: "pkg/a.go", Typeflag: tar.TypeReg, Mode: 0o644},
	), dir); err != nil {
		t.Fatalf("benign archive: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "pkg", "a.go")); err != nil {
		t.Fatal(err)
	}
}

func TestU161_001_ApplyOverlayOnlyTestFiles(t *testing.T) {
	dir := t.TempDir()
	if err := applyOverlay(dir, []FileChange{
		{Path: "p/new_test.go", Status: 'A', Head: []byte("package p\n")},
		{Path: "p/testdata/golden.txt", Status: 'A', Head: []byte("g")},
		{Path: "p/prod.go", Status: 'M', Head: []byte("package p\nfunc Fixed(){}\n")}, // production: must be skipped
		{Path: "p/gone_test.go", Status: 'D', Base: []byte("package p\n")},            // deletion: skipped
	}); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"p/new_test.go", "p/testdata/golden.txt"} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(p))); err != nil {
			t.Errorf("overlay missing %s", p)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "p", "prod.go")); err == nil {
		t.Error("production file leaked into the overlay")
	}
	for _, bad := range []string{"../x_test.go", "p/../../x_test.go", "/abs_test.go", `p\..\x_test.go`, ""} {
		if err := applyOverlay(t.TempDir(), []FileChange{{Path: bad, Status: 'A', Head: []byte("x")}}); err == nil {
			t.Errorf("overlay path %q accepted", bad)
		}
	}
}

func TestU161_001_ParseGoTestJSON(t *testing.T) {
	in := strings.Join([]string{
		`{"Action":"run","Package":"m/p","Test":"TestA"}`,
		`{"Action":"pass","Package":"m/p","Test":"TestA"}`,
		`{"Action":"run","Package":"m/p","Test":"TestB"}`,
		`{"Action":"fail","Package":"m/p","Test":"TestB"}`,
		`{"Action":"skip","Package":"m/p","Test":"TestC"}`,
		`{"Action":"pass","Package":"m/p","Test":"TestB/sub"}`,
		`{"Action":"fail","Package":"m/p"}`,
		`not json at all`,
	}, "\n")
	got, err := parseGoTestJSON(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]Result{"TestA": ResultPass, "TestB": ResultFail, "TestC": ResultUnrunnable}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestU161_001_ParseGoTestJSON_Bounded(t *testing.T) {
	huge := `{"Action":"output","Output":"` + strings.Repeat("x", maxJSONLineBytes+10) + `"}`
	if _, err := parseGoTestJSON(strings.NewReader(huge)); err == nil {
		t.Fatal("oversized line accepted")
	}
}

type fakeExec struct {
	res map[string]Result
	got map[string][]string
}

func (f *fakeExec) Run(_ context.Context, _ string, pkg string, names []string) (map[string]Result, error) {
	if f.got == nil {
		f.got = map[string][]string{}
	}
	f.got[pkg] = append([]string(nil), names...)
	out := map[string]Result{}
	for _, n := range names {
		if r, ok := f.res[n]; ok {
			out[n] = r
		}
	}
	return out, nil
}

func TestU161_001_RunTestsGroupsByPackageAndMarksMissingUnrunnable(t *testing.T) {
	ex := &fakeExec{res: map[string]Result{"TestA": ResultFail, "TestB": ResultPass}}
	tests := []NewTest{
		{Package: "p", File: "p/a_test.go", Name: "TestA"},
		{Package: "p", File: "p/a_test.go", Name: "TestB"},
		{Package: "q", File: "q/q_test.go", Name: "TestMissing"},
	}
	got, err := RunTests(context.Background(), ex, t.TempDir(), tests)
	if err != nil {
		t.Fatal(err)
	}
	if got["p::TestA"] != ResultFail || got["p::TestB"] != ResultPass || got["q::TestMissing"] != ResultUnrunnable {
		t.Fatalf("got %v", got)
	}
	if !reflect.DeepEqual(ex.got["p"], []string{"TestA", "TestB"}) {
		t.Fatalf("package p names = %v", ex.got["p"])
	}
}

func TestU161_001_RunTestsRejectsUnsafeNames(t *testing.T) {
	ex := &fakeExec{}
	for _, nt := range []NewTest{
		{Package: "p", Name: "Test(x|y)"},
		{Package: "../p", Name: "TestA"},
		{Package: "-flag", Name: "TestA"},
		{Package: "/abs", Name: "TestA"},
	} {
		if _, err := RunTests(context.Background(), ex, t.TempDir(), []NewTest{nt}); err == nil {
			t.Errorf("unsafe test %+v accepted", nt)
		}
	}
}

// --- real-git integration (workspace-contained, git-ignored baseline) ---

func gitCmd(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func newRepo(t *testing.T, ignore bool) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	gitCmd(t, root, "init", "-q")
	if ignore {
		writeFile(t, root, ".gitignore", ".backlogit/runtime/\n")
	}
	writeFile(t, root, "go.mod", "module example.com/m\n\ngo 1.24\n")
	writeFile(t, root, "p/p.go", "package p\n\nfunc Compute(x int) int { return x }\n")
	gitCmd(t, root, "add", "-A")
	gitCmd(t, root, "commit", "-q", "-m", "base")
	return root
}

func TestU161_001_PrepareBaseline_WorkspaceContainedAndIgnored(t *testing.T) {
	root := newRepo(t, true)
	base := gitCmd(t, root, "rev-parse", "HEAD")
	overlay := []FileChange{{Path: "p/p_test.go", Status: 'A', Head: []byte("package p\n")}}
	dir, cleanup, err := PrepareBaseline(context.Background(), ExecGit{}, root, base, overlay)
	if err != nil {
		t.Fatal(err)
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	rel, err := filepath.Rel(realRoot, dir)
	if err != nil || strings.HasPrefix(rel, "..") {
		t.Fatalf("baseline dir %q escapes workspace %q", dir, root)
	}
	if !strings.HasPrefix(filepath.ToSlash(rel), ".backlogit/runtime/redhonesty/") {
		t.Fatalf("baseline dir %q not under the sanctioned ignored path", rel)
	}
	b, err := os.ReadFile(filepath.Join(dir, "p", "p.go"))
	if err != nil || !strings.Contains(string(b), "return x }") {
		t.Fatalf("baseline production source missing: %v %q", err, b)
	}
	if _, err := os.Stat(filepath.Join(dir, "p", "p_test.go")); err != nil {
		t.Fatalf("overlay test missing: %v", err)
	}
	if err := cleanup(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); err == nil {
		t.Fatal("cleanup left the baseline directory behind")
	}
}

func TestU161_001_PrepareBaseline_RefusesNonIgnoredPath(t *testing.T) {
	root := newRepo(t, false)
	base := gitCmd(t, root, "rev-parse", "HEAD")
	if _, _, err := PrepareBaseline(context.Background(), ExecGit{}, root, base, nil); err == nil {
		t.Fatal("baseline directory outside a git-ignored path was accepted")
	}
	if _, err := os.Stat(filepath.Join(root, ".backlogit", "runtime", "redhonesty")); err == nil {
		t.Fatal("refusal must not leave a directory behind")
	}
}

func TestU161_001_PrepareBaseline_RequiresFullSHA(t *testing.T) {
	root := newRepo(t, true)
	if _, _, err := PrepareBaseline(context.Background(), ExecGit{}, root, "HEAD", nil); err == nil {
		t.Fatal("symbolic base accepted; the baseline must be an immutable SHA")
	}
}

func TestU161_001_CollectChanges(t *testing.T) {
	root := newRepo(t, true)
	base := gitCmd(t, root, "rev-parse", "HEAD")
	writeFile(t, root, "p/p.go", "package p\n\nfunc Compute(x int) int { return x + 1 }\n")
	writeFile(t, root, "p/p_test.go", "package p\n")
	gitCmd(t, root, "add", "-A")
	gitCmd(t, root, "commit", "-q", "-m", "fix")
	head := gitCmd(t, root, "rev-parse", "HEAD")
	got, err := CollectChanges(context.Background(), ExecGit{}, root, base, head)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Path != "p/p.go" || got[0].Status != 'M' || got[1].Path != "p/p_test.go" || got[1].Status != 'A' {
		t.Fatalf("got %+v", got)
	}
	if !bytes.Contains(got[0].Base, []byte("return x }")) || !bytes.Contains(got[0].Head, []byte("x + 1")) || got[1].Base != nil {
		t.Fatalf("base/head content wrong: %+v", got)
	}
}
