package redhonesty

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/softwaresalt/backlogit/internal/faultline"
)

const (
	u3Base = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	u3Head = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

func honestVerdict(name string) Verdict {
	return Verdict{Test: NewTest{Package: "internal/x", File: "internal/x/x_test.go", Name: name}, Baseline: ResultFail, Head: ResultPass}
}

func TestU161_003_BuildArtifactConformsToSharedContract(t *testing.T) {
	a, err := BuildArtifact("161.003-T", u3Base, u3Head, []Verdict{honestVerdict("TestB"), honestVerdict("TestA")})
	if err != nil {
		t.Fatal(err)
	}
	if a.SchemaVersion != faultline.EvidenceSchemaVersionV2 || a.NodeFamily != faultline.NodeFamilyRedGreen ||
		a.Status != faultline.StatusPass || a.ProducingTask != "161.003-T" || a.ProducingCommit != u3Head {
		t.Fatalf("envelope = %+v", a)
	}
	raw, _ := json.Marshal(a)
	_, fp, err := faultline.DecodeAndValidate(raw)
	if err != nil {
		t.Fatalf("conformance: %v", err)
	}
	rg := fp.(*faultline.RedGreenEvidence)
	if len(rg.Tests) != 2 || rg.Tests[0].Test != "TestA" {
		t.Fatalf("tests not sorted deterministically: %+v", rg.Tests)
	}
}

func TestU161_003_BuildArtifactDishonestIsFailStatus(t *testing.T) {
	bad := honestVerdict("TestBad")
	bad.Baseline = ResultPass
	bad.Rejections = []RejectionClass{RejectAlreadyPasses}
	a, err := BuildArtifact("161.003-T", u3Base, u3Head, []Verdict{honestVerdict("TestOK"), bad})
	if err != nil {
		t.Fatal(err)
	}
	if a.Status != faultline.StatusFail {
		t.Fatalf("status = %s, want fail", a.Status)
	}
	if err := faultline.Validate(a); err != nil {
		t.Fatal(err)
	}
}

func TestU161_003_BuildArtifactRejectsBadInputs(t *testing.T) {
	if _, err := BuildArtifact("161.003-T", u3Base, u3Head, nil); !errors.Is(err, ErrNoNewTests) {
		t.Fatalf("no tests: err = %v", err)
	}
	for name, args := range map[string][3]string{
		"bad task":   {"../x", u3Base, u3Head},
		"short base": {"161.003-T", "abc", u3Head},
		"short head": {"161.003-T", u3Base, "abc"},
	} {
		if _, err := BuildArtifact(args[0], args[1], args[2], []Verdict{honestVerdict("TestA")}); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
}

func TestU161_003_StoreRoundTripPerTaskAndCommit(t *testing.T) {
	dir := t.TempDir()
	st, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := BuildArtifact("161.003-T", u3Base, u3Head, []Verdict{honestVerdict("TestA")})
	path, err := st.Write(a)
	if err != nil {
		t.Fatal(err)
	}
	rel, _ := filepath.Rel(dir, path)
	if filepath.ToSlash(rel) != "161.003-T/"+u3Head+".json" {
		t.Fatalf("path = %s", rel)
	}
	got, err := st.Load("161.003-T", u3Head)
	if err != nil {
		t.Fatal(err)
	}
	c1, _ := a.Canonical()
	c2, _ := got.Canonical()
	if string(c1) != string(c2) {
		t.Fatal("round trip changed the artifact")
	}
	if _, err := st.Load("161.003-T", strings.Repeat("c", 40)); err == nil {
		t.Fatal("missing evidence must error")
	}
}

func TestU161_003_StoreRejectsUnsafeKeysAndSubstitution(t *testing.T) {
	dir := t.TempDir()
	st, _ := NewStore(dir)
	for _, k := range [][2]string{
		{"../x", u3Head}, {"161.003-T", "../../etc"}, {"161.003-T", "HEAD"}, {"", u3Head}, {"161.003-T", strings.ToUpper(u3Head)},
		{"a/b", u3Head}, {`a\b`, u3Head},
	} {
		if _, err := st.Load(k[0], k[1]); err == nil {
			t.Errorf("Load(%q,%q) accepted", k[0], k[1])
		}
	}
	// Substitution: a valid artifact for another task/commit placed at this key.
	a, _ := BuildArtifact("161.001-T", u3Base, u3Head, []Verdict{honestVerdict("TestA")})
	raw, _ := json.Marshal(a)
	if err := os.MkdirAll(filepath.Join(dir, "161.003-T"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "161.003-T", u3Head+".json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Load("161.003-T", u3Head); err == nil {
		t.Fatal("artifact bound to a different task was returned")
	}
	// Oversized file is refused before decode.
	big := filepath.Join(dir, "161.002-T")
	_ = os.MkdirAll(big, 0o755)
	if err := os.WriteFile(filepath.Join(big, u3Head+".json"), make([]byte, faultline.MaxArtifactBytes+10), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Load("161.002-T", u3Head); err == nil {
		t.Fatal("oversized evidence accepted")
	}
	// A non-red_green family is refused.
	par := faultline.EvidenceArtifact{SchemaVersion: 1, ProducingTask: "161.003-T", ProducingCommit: u3Head,
		NodeFamily: "parity", Applicability: []string{"cli"}, ValidatorIdentity: "v", Status: "pass",
		VerifiedEvidence: json.RawMessage(`{"scenario_id":"s","dimension":"d","surfaces":["cli"],"divergent_fields":[],"expected_divergence":false,"tracked_defect":"","detail":""}`)}
	if _, err := st.Write(par); err == nil {
		t.Fatal("Write accepted a non-red_green artifact")
	}
}

func TestU161_003_StoreWriteRefusesInvalidArtifact(t *testing.T) {
	st, _ := NewStore(t.TempDir())
	a, _ := BuildArtifact("161.003-T", u3Base, u3Head, []Verdict{honestVerdict("TestA")})
	a.ProducingCommit = u3Base // head_sha no longer bound to the envelope commit
	if _, err := st.Write(a); err == nil {
		t.Fatal("invalid artifact persisted")
	}
}

// --- end-to-end gate over a seeded repository ---

type e2eRepo struct {
	root, base, head string
}

func seedRepo(t *testing.T, fixed string, testSrc string) e2eRepo {
	t.Helper()
	root := newRepo(t, true)
	base := gitCmd(t, root, "rev-parse", "HEAD")
	writeFile(t, root, "p/p.go", fixed)
	writeFile(t, root, "p/p_test.go", testSrc)
	gitCmd(t, root, "add", "-A")
	gitCmd(t, root, "commit", "-q", "-m", "fix")
	return e2eRepo{root: root, base: base, head: gitCmd(t, root, "rev-parse", "HEAD")}
}

func newGate(t *testing.T, r e2eRepo) *Gate {
	t.Helper()
	st, err := NewStore(filepath.Join(r.root, ".backlogit", "evidence", "redgreen"))
	if err != nil {
		t.Fatal(err)
	}
	return &Gate{Git: ExecGit{}, Exec: GoTestExecutor{}, Root: r.root, Store: st}
}

func TestU161_003_GateAcceptsGenuineReproducerEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("runs go test in a seeded repository")
	}
	r := seedRepo(t,
		"package p\n\nfunc Compute(x int) int { return x + 1 }\n",
		"package p\n\nimport \"testing\"\n\nfunc TestFix(t *testing.T) {\n\tif got := Compute(2); got != 3 {\n\t\tt.Fatalf(\"got %d\", got)\n\t}\n}\n")
	rep, err := newGate(t, r).Check(context.Background(), Request{TaskID: "161.003-T", BaseRef: r.base, HeadRef: r.head})
	if err != nil {
		t.Fatal(err)
	}
	if !rep.Honest || len(rep.Verdicts) != 1 || rep.Verdicts[0].Baseline != ResultFail || rep.Verdicts[0].Head != ResultPass {
		t.Fatalf("report = %+v", rep)
	}
	if rep.Artifact.Status != faultline.StatusPass || rep.BaseSHA != r.base || rep.HeadSHA != r.head {
		t.Fatalf("artifact = %+v", rep.Artifact)
	}
	st, _ := NewStore(filepath.Join(r.root, ".backlogit", "evidence", "redgreen"))
	if _, err := st.Load("161.003-T", r.head); err != nil {
		t.Fatalf("evidence not retrievable per task/commit: %v", err)
	}
	if entries, _ := os.ReadDir(filepath.Join(r.root, ".backlogit", "runtime", "redhonesty")); len(entries) != 0 {
		t.Fatalf("baseline workspace not cleaned up: %v", entries)
	}
}

func TestU161_003_GateRejectsAlreadyGreenTestEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("runs go test in a seeded repository")
	}
	// The test passes at the baseline too: the claimed red never failed.
	r := seedRepo(t,
		"package p\n\nfunc Compute(x int) int { return x + 1 }\n",
		"package p\n\nimport \"testing\"\n\nfunc TestNotRed(t *testing.T) {\n\tif Compute(0) < 0 {\n\t\tt.Fatal(\"neg\")\n\t}\n}\n")
	rep, err := newGate(t, r).Check(context.Background(), Request{TaskID: "161.003-T", BaseRef: r.base, HeadRef: r.head})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Honest || rep.Artifact.Status != faultline.StatusFail || len(rep.Verdicts) != 1 {
		t.Fatalf("report = %+v", rep)
	}
	if rep.Verdicts[0].Rejections[0] != RejectAlreadyPasses {
		t.Fatalf("rejections = %v", rep.Verdicts[0].Rejections)
	}
}

func TestU161_003_GateFailsClosedWhenBaselineDoesNotBuild(t *testing.T) {
	if testing.Short() {
		t.Skip("runs go test in a seeded repository")
	}
	// The new test references a symbol that only exists at head, so it cannot
	// build at baseline: unverifiable, therefore rejected.
	r := seedRepo(t,
		"package p\n\nfunc Compute(x int) int { return x }\n\nfunc Added() int { return 1 }\n",
		"package p\n\nimport \"testing\"\n\nfunc TestNeedsAdded(t *testing.T) {\n\tif Added() != 1 {\n\t\tt.Fatal(\"x\")\n\t}\n}\n")
	rep, err := newGate(t, r).Check(context.Background(), Request{TaskID: "161.003-T", BaseRef: r.base, HeadRef: r.head})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Honest || rep.Verdicts[0].Baseline != ResultUnrunnable || rep.Verdicts[0].Rejections[0] != RejectUnverifiableBaseline {
		t.Fatalf("report = %+v", rep)
	}
}

func TestU161_003_GateNoNewTests(t *testing.T) {
	r := seedRepo(t, "package p\n\nfunc Compute(x int) int { return x + 1 }\n", "package p\n")
	// p_test.go with no Test functions adds no tests.
	if _, err := newGate(t, r).Check(context.Background(), Request{TaskID: "161.003-T", BaseRef: r.base, HeadRef: r.head}); !errors.Is(err, ErrNoNewTests) {
		t.Fatalf("err = %v, want ErrNoNewTests", err)
	}
}
