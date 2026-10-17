package redhonesty

import (
	"archive/tar"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Regression tests for the adversarial-review remediation batch (S9).

func TestU161_001_OverlayRemovesDeletedTestFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "p/old_test.go", "package p\n")
	writeFile(t, dir, "p/testdata/old.txt", "x")
	writeFile(t, dir, "p/prod.go", "package p\n")
	if err := applyOverlay(dir, []FileChange{
		{Path: "p/old_test.go", Status: 'D', Base: []byte("package p\n")},
		{Path: "p/testdata/old.txt", Status: 'D', Base: []byte("x")},
		{Path: "p/prod.go", Status: 'D', Base: []byte("package p\n")}, // production deletion is NOT applied
		{Path: "p/never_there_test.go", Status: 'D'},
	}); err != nil {
		t.Fatal(err)
	}
	for _, gone := range []string{"p/old_test.go", "p/testdata/old.txt"} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(gone))); err == nil {
			t.Errorf("%s survived the overlay; baseline would see a duplicate/stale test", gone)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "p", "prod.go")); err != nil {
		t.Error("production file was removed by the overlay")
	}
}

func TestU161_001_IdentifyNewTests_MovedAndEmptyFiles(t *testing.T) {
	got, err := IdentifyNewTests([]FileChange{
		{Path: "p/a_test.go", Status: 'D', Base: []byte("package p\nimport \"testing\"\nfunc TestMoved(t *testing.T) {}\n")},
		{Path: "p/b_test.go", Status: 'A', Head: []byte("package p\nimport \"testing\"\nfunc TestMoved(t *testing.T) {}\nfunc TestReallyNew(t *testing.T) {}\n")},
		{Path: "p/empty_test.go", Status: 'M', Base: []byte{}, Head: []byte{}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "TestReallyNew" {
		t.Fatalf("got %+v, want only TestReallyNew", got)
	}
}

func TestU161_001_ChangedSymbols_MovedAcrossFilesIsNotAChange(t *testing.T) {
	s, err := ChangedSymbols([]FileChange{
		{Path: "p/a.go", Status: 'M', Base: []byte("package p\nfunc Moved() int { return 1 }\nfunc Stay() {}\n"), Head: []byte("package p\nfunc Stay() {}\n")},
		{Path: "p/b.go", Status: 'A', Head: []byte("package p\nfunc Moved() int { return 1 }\n")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !s.Empty() {
		t.Fatalf("a move within one package reported as change: %v", s.Funcs)
	}
}

func TestU161_001_ChangedSymbols_MultipleInitsEachTracked(t *testing.T) {
	base := "package p\nvar x int\nfunc init() { x = 1 }\nfunc init() { x = 2 }\n"
	head := "package p\nvar x int\nfunc init() { x = 1 }\nfunc init() { x = 3 }\n"
	s, err := ChangedSymbols([]FileChange{{Path: "p/p.go", Status: 'M', Base: []byte(base), Head: []byte(head)}})
	if err != nil {
		t.Fatal(err)
	}
	if !has(s.Funcs, "init") {
		t.Fatalf("change to the second init() was lost: %v", s.Funcs)
	}
}

func TestU161_001_ChangedSymbols_EmptyBaseFile(t *testing.T) {
	if _, err := ChangedSymbols([]FileChange{{Path: "p/p.go", Status: 'M', Base: []byte{}, Head: []byte("package p\nfunc F() {}\n")}}); err != nil {
		t.Fatalf("empty base file must be treated as absent: %v", err)
	}
}

func TestU161_001_PackageWildcardAndDeviceNamesRejected(t *testing.T) {
	ex := &fakeExec{}
	for _, pkg := range []string{"internal/...", "a/.../b", "NUL", "internal/CON"} {
		if _, err := RunTests(context.Background(), ex, t.TempDir(), []NewTest{{Package: pkg, Name: "TestA"}}); err == nil {
			t.Errorf("package %q accepted", pkg)
		}
	}
}

// scriptedGit answers git commands from fixed data.
type scriptedGit struct {
	diff  string
	blobs map[string]string
}

func (g scriptedGit) Run(_ context.Context, _ string, w io.Writer, args ...string) error {
	switch args[0] {
	case "diff":
		_, err := io.WriteString(w, g.diff)
		return err
	case "cat-file":
		_, err := io.WriteString(w, g.blobs[args[2]])
		return err
	}
	return nil
}

func TestU161_001_CollectChanges_FiltersBeforeValidatingAndBoundsContent(t *testing.T) {
	b, h := strings.Repeat("a", 40), strings.Repeat("b", 40)
	// An unrelated file with an exotic name must not abort the gate.
	g := scriptedGit{
		diff:  "A\x00docs/we:ird\\name.md\x00M\x00p/p.go\x00",
		blobs: map[string]string{b + ":p/p.go": "package p\n", h + ":p/p.go": "package p\nfunc F(){}\n"},
	}
	got, err := CollectChanges(context.Background(), g, ".", b, h)
	if err != nil || len(got) != 1 || got[0].Path != "p/p.go" {
		t.Fatalf("got %+v err %v", got, err)
	}
	// A relevant file with an unsafe name is refused.
	bad := scriptedGit{diff: "A\x00p/NUL.go\x00"}
	if _, err := CollectChanges(context.Background(), bad, ".", b, h); err == nil {
		t.Fatal("reserved device name accepted")
	}
	// Cumulative content budget.
	big := strings.Repeat("x", maxSourceBytes-1)
	var diff strings.Builder
	blobs := map[string]string{}
	for i := 0; i < 20; i++ { // 20 files x 2 blobs x ~4 MiB exceeds 64 MiB
		name := "p/f" + string(rune('a'+i)) + ".go"
		diff.WriteString("M\x00" + name + "\x00")
		blobs[b+":"+name], blobs[h+":"+name] = big, big
	}
	if _, err := CollectChanges(context.Background(), scriptedGit{diff: diff.String(), blobs: blobs}, ".", b, h); err == nil {
		t.Fatal("cumulative change content budget not enforced")
	}
}

func TestU161_001_ChildEnvironmentIsScrubbed(t *testing.T) {
	t.Setenv("GIT_DIR", "/elsewhere")
	t.Setenv("MY_API_TOKEN", "s3cr3t")
	t.Setenv("GOFLAGS", "-mod=vendor")
	t.Setenv("PLAIN_VAR", "ok")
	for _, kv := range gitEnv() {
		if strings.HasPrefix(kv, "GIT_DIR=") {
			t.Error("GIT_DIR leaked into git child")
		}
	}
	env := goTestEnv()
	joined := "\n" + strings.Join(env, "\n") + "\n"
	for _, bad := range []string{"MY_API_TOKEN=", "GOFLAGS=-mod=vendor"} {
		if strings.Contains(joined, "\n"+bad) {
			t.Errorf("%s leaked into go test child", bad)
		}
	}
	for _, want := range []string{"GOTOOLCHAIN=local", "GOWORK=off", "PLAIN_VAR=ok"} {
		if !strings.Contains(joined, "\n"+want+"\n") {
			t.Errorf("go test env missing %s", want)
		}
	}
}

func TestU161_001_LimitedBufferOverflowDoesNotBlockWriter(t *testing.T) {
	lb := &limitedBuffer{max: 8}
	if n, err := lb.Write([]byte("0123456789")); err != nil || n != 10 {
		t.Fatalf("Write = %d, %v; must accept and discard so the child never blocks", n, err)
	}
	if !lb.exceeded || lb.buf.Len() != 0 {
		t.Fatalf("exceeded=%v len=%d", lb.exceeded, lb.buf.Len())
	}
	g := scriptedGit{diff: strings.Repeat("x", 32)}
	if _, err := gitOutput(context.Background(), g, ".", 8, "diff"); err == nil {
		t.Fatal("overflowing git output must be an error")
	}
}

func TestU161_001_ChangedSymbols_BuildTagVariantsAreBothTracked(t *testing.T) {
	linuxBase := "package p\nfunc Impl() int { return 1 }\n"
	winBase := "package p\nfunc Impl() int { return 2 }\n"
	// Only the earlier-sorted variant changes.
	s, err := ChangedSymbols([]FileChange{
		{Path: "p/impl_linux.go", Status: 'M', Base: []byte(linuxBase), Head: []byte("package p\nfunc Impl() int { return 10 }\n")},
		{Path: "p/impl_windows.go", Status: 'M', Base: []byte(winBase), Head: []byte(winBase)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !has(s.Funcs, "Impl") {
		t.Fatal("change to one build-tag variant was hidden by the other")
	}
}

func TestU161_001_ExtractArchiveToleratesNamesLegalElsewhere(t *testing.T) {
	// Reserved device names are a host concern for files the gate writes itself;
	// the baseline archive check is containment-only.
	if err := extractArchive(tarOf(t, tar.Header{Name: "docs/notes: v2.md", Typeflag: tar.TypeReg, Mode: 0o644}), t.TempDir()); err == nil {
		t.Log("colon accepted on this host")
	}
	if err := validArchivePath("docs/prn/readme.md"); err != nil {
		t.Fatalf("legal-elsewhere path rejected: %v", err)
	}
	for _, bad := range []string{"../x", "a/../../x", "/abs", "C:/x", `a\b`, ""} {
		if err := validArchivePath(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestU161_003_GateRequiresBaseToBeAncestorOfHead(t *testing.T) {
	r := seedRepo(t,
		"package p\n\nfunc Compute(x int) int { return x + 1 }\n",
		"package p\n\nimport \"testing\"\n\nfunc TestFix(t *testing.T) { if Compute(2) != 3 { t.Fatal(\"x\") } }\n")
	// Swapped: head (the fix) as the "base" and the old commit as head is not an ancestor relation.
	_, err := newGate(t, r).Check(context.Background(), Request{TaskID: "161.003-T", BaseRef: r.head, HeadRef: r.base})
	if err == nil {
		t.Fatal("non-ancestor base accepted")
	}
}

func TestU161_003_GateHeadRunIgnoresWorkingTreeState(t *testing.T) {
	if testing.Short() {
		t.Skip("runs go test in a seeded repository")
	}
	r := seedRepo(t,
		"package p\n\nfunc Compute(x int) int { return x + 1 }\n",
		"package p\n\nimport \"testing\"\n\nfunc TestFix(t *testing.T) { if Compute(2) != 3 { t.Fatal(\"x\") } }\n")
	// Untracked and ignored files that would break or alter a working-tree run
	// must not influence evidence bound to the committed head.
	writeFile(t, r.root, "p/sneaky.go", "package p\n\nfunc Compute(x int) int { return 0 }\n")
	writeFile(t, r.root, ".backlogit/runtime/ignored.go", "package broken\n")
	rep, err := newGate(t, r).Check(context.Background(), Request{TaskID: "161.003-T", BaseRef: r.base, HeadRef: r.head})
	if err != nil {
		t.Fatal(err)
	}
	if !rep.Honest {
		t.Fatalf("working-tree state leaked into the head run: %+v", rep.Verdicts)
	}
}

func TestU161_003_ReconcileExitDistrustsPassesOfAFailedRun(t *testing.T) {
	// TestMain exiting non-zero after m.Run: tests printed pass but the run failed.
	got := reconcileExit(map[string]Result{"TestA": ResultPass, "TestB": ResultPass}, true)
	if got["TestA"] != ResultUnrunnable || got["TestB"] != ResultUnrunnable {
		t.Fatalf("passes of a failed run must be unverifiable: %v", got)
	}
	// An ordinary failing test explains the non-zero exit; results are kept.
	got = reconcileExit(map[string]Result{"TestA": ResultPass, "TestB": ResultFail}, true)
	if got["TestA"] != ResultPass || got["TestB"] != ResultFail {
		t.Fatalf("ordinary failure results must be preserved: %v", got)
	}
	got = reconcileExit(map[string]Result{"TestA": ResultPass}, false)
	if got["TestA"] != ResultPass {
		t.Fatalf("clean exit must not alter results: %v", got)
	}
}

func TestU161_002_ExpectedFieldAssignmentIsCaught(t *testing.T) {
	src := `package p
import "testing"
func TestX(t *testing.T) {
	var tc struct{ want int }
	tc.want = Compute(2)
	if Compute(2) != tc.want { t.Fatal("x") }
}
`
	v := evalOne(t, src, ResultFail, ResultPass, syms([]string{"Compute"}, nil, nil, nil))
	wantRejections(t, v, RejectBuggyPathExpected)
	tableSrc := `package p
import "testing"
func TestX(t *testing.T) {
	cases := []struct{ in, want int }{{in: 2, want: Compute(2)}}
	for _, c := range cases { if Compute(c.in) != c.want { t.Fatal("x") } }
}
`
	wantRejections(t, evalOne(t, tableSrc, ResultFail, ResultPass, syms([]string{"Compute"}, nil, nil, nil)), RejectBuggyPathExpected)
}

func TestU161_003_StoreConcurrentWritersCannotReplaceEachOther(t *testing.T) {
	st, _ := NewStore(t.TempDir())
	good, _ := BuildArtifact("161.003-T", u3Base, u3Head, []Verdict{honestVerdict("TestA")})
	bad := honestVerdict("TestA")
	bad.Baseline = ResultPass
	bad.Rejections = []RejectionClass{RejectAlreadyPasses}
	other, _ := BuildArtifact("161.003-T", u3Base, u3Head, []Verdict{bad})

	const n = 16
	results := make(chan error, n)
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		a := good
		if i%2 == 1 {
			a = other
		}
		go func() {
			<-start
			_, err := st.Write(a)
			results <- err
		}()
	}
	close(start)
	okGood := 0
	for i := 0; i < n; i++ {
		if err := <-results; err == nil {
			okGood++ // compared below with the number of writers of the winning content
		}
	}
	loaded, err := st.Load("161.003-T", u3Head)
	if err != nil {
		t.Fatal(err)
	}
	// Exactly one artifact content won; every successful writer wrote that content.
	winner := loaded.Status
	wantOK := 0
	for i := 0; i < n; i++ {
		a := good
		if i%2 == 1 {
			a = other
		}
		if a.Status == winner {
			wantOK++
		}
	}
	if okGood != wantOK {
		t.Fatalf("successful writers = %d, want exactly the %d writers of the winning content (%s)", okGood, wantOK, winner)
	}
}

func TestU161_003_StoreIsIdempotentButNotReplaceable(t *testing.T) {
	st, _ := NewStore(t.TempDir())
	a, _ := BuildArtifact("161.003-T", u3Base, u3Head, []Verdict{honestVerdict("TestA")})
	if _, err := st.Write(a); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Write(a); err != nil {
		t.Fatalf("identical re-record must be idempotent: %v", err)
	}
	bad := honestVerdict("TestA")
	bad.Baseline = ResultPass
	bad.Rejections = []RejectionClass{RejectAlreadyPasses}
	b, _ := BuildArtifact("161.003-T", u3Base, u3Head, []Verdict{bad})
	if _, err := st.Write(b); err == nil {
		t.Fatal("a different artifact silently replaced recorded evidence")
	}
}
