package redhonesty

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/softwaresalt/backlogit/internal/faultline"
)

// Regression tests for Copilot review cycle 2 (S9).

func TestU161_002_MultiResultAndGenericBindingsAreCaught(t *testing.T) {
	s := syms([]string{"Compute"}, nil, nil, nil)
	for name, body := range map[string]string{
		"multi-result define":   "got, want := Compute(2)\n\t_ = got\n\tif got != want { t.Fatal(\"x\") }",
		"multi-result var":      "var got, expected = Compute(2)\n\tif got != expected { t.Fatal(\"x\") }",
		"explicit generic":      "want := Compute[int](2)\n\tif Compute(2) != want { t.Fatal(\"x\") }",
		"multi type args":       "want := Compute[int, string](2)\n\tif Compute(2) != want { t.Fatal(\"x\") }",
		"parenthesized callee":  "want := (Compute)(2)\n\tif Compute(2) != want { t.Fatal(\"x\") }",
		"exported Want field":   "var tc struct{ Want int }\n\ttc.Want = Compute(2)\n\tif Compute(2) != tc.Want { t.Fatal(\"x\") }",
		"paired multi-value":    "a, want := 1, Compute(2)\n\t_ = a\n\tif Compute(2) != want { t.Fatal(\"x\") }",
		"blank-first multi-res": "_, want := Compute(2)\n\tif Compute(2) != want { t.Fatal(\"x\") }",
	} {
		src := "package p\nimport \"testing\"\nfunc TestX(t *testing.T) {\n\t" + body + "\n}\n"
		v := evalOne(t, src, ResultFail, ResultPass, s)
		found := false
		for _, r := range v.Rejections {
			found = found || r == RejectBuggyPathExpected
		}
		if !found {
			t.Errorf("%s: rejections = %v, want %s", name, v.Rejections, RejectBuggyPathExpected)
		}
	}
	// A literal expected value next to a multi-result call stays honest.
	honest := "package p\nimport \"testing\"\nfunc TestX(t *testing.T) {\n\tgot, err := Compute(2)\n\twant := 3\n\t_ = err\n\tif got != want { t.Fatal(\"x\") }\n}\n"
	wantRejections(t, evalOne(t, honest, ResultFail, ResultPass, s))
}

func TestU161_003_StoreExactLimitArtifactRoundTrips(t *testing.T) {
	build := func(pad int) faultline.EvidenceArtifact {
		p := faultline.RedGreenEvidence{BaseSHA: u3Base, HeadSHA: u3Head}
		for i := 0; i < 17; i++ {
			p.Tests = append(p.Tests, faultline.RedGreenTest{
				Package: "internal/x", Test: "TestPad" + string(rune('A'+i)) + strings.Repeat("a", 60000),
				BaselineResult: "fail", HeadResult: "pass", Honest: true, RejectionClasses: []string{},
			})
		}
		p.Tests = append(p.Tests, faultline.RedGreenTest{
			Package: "internal/x", Test: "TestTail" + strings.Repeat("b", pad),
			BaselineResult: "fail", HeadResult: "pass", Honest: true, RejectionClasses: []string{},
		})
		raw, _ := json.Marshal(p)
		return faultline.EvidenceArtifact{SchemaVersion: faultline.EvidenceSchemaVersionV2, ProducingTask: "161.003-T",
			ProducingCommit: u3Head, NodeFamily: faultline.NodeFamilyRedGreen, Applicability: []string{"internal"},
			ValidatorIdentity: ValidatorIdentity, VerifiedEvidence: raw, Status: faultline.StatusPass}
	}
	pad := 100
	for i := 0; i < 4; i++ { // converge on exactly MaxArtifactBytes
		raw, _ := json.Marshal(build(pad))
		pad += faultline.MaxArtifactBytes - len(raw)
	}
	a := build(pad)
	raw, _ := json.Marshal(a)
	if len(raw) != faultline.MaxArtifactBytes {
		t.Skipf("could not size artifact exactly (got %d)", len(raw))
	}
	st, _ := NewStore(t.TempDir())
	if _, err := st.Write(a); err != nil {
		t.Fatalf("exact-limit write: %v", err)
	}
	if _, err := st.Load("161.003-T", u3Head); err != nil {
		t.Fatalf("exact-limit artifact must load back: %v", err)
	}
	if _, err := st.Write(a); err != nil {
		t.Fatalf("exact-limit idempotent re-write: %v", err)
	}
}

func TestU161_003_StoreRefusesSymlinkedTaskDirectory(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(dir, "161.003-T")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	st, _ := NewStore(dir)
	a, _ := BuildArtifact("161.003-T", u3Base, u3Head, []Verdict{honestVerdict("TestA")})
	if _, err := st.Write(a); err == nil {
		t.Fatal("Write followed a symlinked task directory out of the store")
	}
	if entries, _ := os.ReadDir(outside); len(entries) != 0 {
		t.Fatalf("evidence escaped the store: %v", entries)
	}
	if _, err := st.Load("161.003-T", u3Head); err == nil {
		t.Fatal("Load followed a symlinked task directory")
	}
}

func TestU161_003_StoreRefusesSymlinkedRecord(t *testing.T) {
	dir := t.TempDir()
	st, _ := NewStore(dir)
	a, _ := BuildArtifact("161.003-T", u3Base, u3Head, []Verdict{honestVerdict("TestA")})
	if _, err := st.Write(a); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "elsewhere.json")
	raw, _ := os.ReadFile(filepath.Join(dir, "161.003-T", u3Head+".json"))
	if err := os.WriteFile(outside, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "161.003-T", u3Head+".json")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "161.003-T", u3Head+".json")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := st.Load("161.003-T", u3Head); err == nil {
		t.Fatal("Load followed a symlinked record")
	}
	_ = context.Background
}
