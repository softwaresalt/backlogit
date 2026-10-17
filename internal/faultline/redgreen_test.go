package faultline

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

const (
	rgBase = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	rgHead = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

func rgArtifact(t *testing.T, p RedGreenEvidence, status string) EvidenceArtifact {
	t.Helper()
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	return EvidenceArtifact{
		SchemaVersion:     EvidenceSchemaVersionV2,
		ProducingTask:     "161.003-T",
		ProducingCommit:   rgHead,
		NodeFamily:        NodeFamilyRedGreen,
		Applicability:     []string{"internal"},
		ValidatorIdentity: "backlogit/redhonesty@v1",
		VerifiedEvidence:  raw,
		Status:            status,
	}
}

func rgHonest() RedGreenEvidence {
	return RedGreenEvidence{
		BaseSHA: rgBase, HeadSHA: rgHead,
		Tests: []RedGreenTest{{
			Package: "internal/x", Test: "TestFix", BaselineResult: "fail",
			HeadResult: "pass", Honest: true, RejectionClasses: []string{},
		}},
	}
}

func rgDishonest() RedGreenEvidence {
	p := rgHonest()
	p.Tests[0].BaselineResult = "pass"
	p.Tests[0].Honest = false
	p.Tests[0].RejectionClasses = []string{"already_passes_at_baseline"}
	return p
}

func TestU161_003_RedGreenRoundTripAndCanonical(t *testing.T) {
	a := rgArtifact(t, rgHonest(), StatusPass)
	raw, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	got, fp, err := DecodeAndValidate(raw)
	if err != nil {
		t.Fatalf("DecodeAndValidate: %v", err)
	}
	if _, ok := fp.(*RedGreenEvidence); !ok {
		t.Fatalf("payload type = %T, want *RedGreenEvidence", fp)
	}
	c1, err := got.Canonical()
	if err != nil {
		t.Fatalf("Canonical: %v", err)
	}
	// Test ordering must not change the canonical digest.
	p := rgHonest()
	p.Tests = append(p.Tests, RedGreenTest{Package: "internal/a", Test: "TestA", BaselineResult: "fail", HeadResult: "pass", Honest: true, RejectionClasses: []string{}})
	pr := p
	pr.Tests = []RedGreenTest{p.Tests[1], p.Tests[0]}
	ca, err := rgArtifact(t, p, StatusPass).Canonical()
	if err != nil {
		t.Fatal(err)
	}
	cb, err := rgArtifact(t, pr, StatusPass).Canonical()
	if err != nil {
		t.Fatal(err)
	}
	if string(ca) != string(cb) {
		t.Fatalf("canonical differs by test order:\n%s\n%s", ca, cb)
	}
	if len(c1) == 0 || strings.Contains(string(c1), "null") {
		t.Fatalf("canonical bytes empty or contain null: %s", c1)
	}
}

func TestU161_003_RedGreenDishonestIsFailStatus(t *testing.T) {
	if err := Validate(rgArtifact(t, rgDishonest(), StatusFail)); err != nil {
		t.Fatalf("fail status with a dishonest test must validate: %v", err)
	}
	if err := Validate(rgArtifact(t, rgDishonest(), StatusPass)); !errors.Is(err, ErrValidation) {
		t.Fatalf("pass status with a dishonest test: err = %v, want ErrValidation", err)
	}
	if err := Validate(rgArtifact(t, rgHonest(), StatusFail)); !errors.Is(err, ErrValidation) {
		t.Fatalf("fail status with only honest tests: err = %v, want ErrValidation", err)
	}
	if err := Validate(rgArtifact(t, rgHonest(), StatusReportOnly)); !errors.Is(err, ErrValidation) {
		t.Fatalf("report_only must be rejected for red_green: err = %v", err)
	}
}

func TestU161_003_RedGreenNegativeInvariants(t *testing.T) {
	cases := map[string]func(p *RedGreenEvidence, a *EvidenceArtifact){
		"head sha not bound to producing commit": func(p *RedGreenEvidence, a *EvidenceArtifact) { a.ProducingCommit = rgBase },
		"bad base sha":                           func(p *RedGreenEvidence, _ *EvidenceArtifact) { p.BaseSHA = "main" },
		"base equals head":                       func(p *RedGreenEvidence, _ *EvidenceArtifact) { p.BaseSHA = p.HeadSHA },
		"no tests":                               func(p *RedGreenEvidence, _ *EvidenceArtifact) { p.Tests = []RedGreenTest{} },
		"unknown baseline result":                func(p *RedGreenEvidence, _ *EvidenceArtifact) { p.Tests[0].BaselineResult = "maybe" },
		"honest but baseline pass":               func(p *RedGreenEvidence, _ *EvidenceArtifact) { p.Tests[0].BaselineResult = "pass" },
		"honest with rejection classes":          func(p *RedGreenEvidence, _ *EvidenceArtifact) { p.Tests[0].RejectionClasses = []string{"x_y"} },
		"dishonest without classes": func(p *RedGreenEvidence, _ *EvidenceArtifact) {
			p.Tests[0].Honest = false
		},
		"malformed class token": func(p *RedGreenEvidence, _ *EvidenceArtifact) {
			p.Tests[0].Honest = false
			p.Tests[0].RejectionClasses = []string{"Bad Class\n"}
		},
		"duplicate test":    func(p *RedGreenEvidence, _ *EvidenceArtifact) { p.Tests = append(p.Tests, p.Tests[0]) },
		"empty test name":   func(p *RedGreenEvidence, _ *EvidenceArtifact) { p.Tests[0].Test = "" },
		"package traversal": func(p *RedGreenEvidence, _ *EvidenceArtifact) { p.Tests[0].Package = "../x" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			p := rgHonest()
			a := rgArtifact(t, p, StatusPass)
			mutate(&p, &a)
			raw, _ := json.Marshal(p)
			a.VerifiedEvidence = raw
			if err := Validate(a); !errors.Is(err, ErrValidation) {
				t.Fatalf("err = %v, want ErrValidation", err)
			}
		})
	}
}

func TestU161_003_VersionManifestsStayClosed(t *testing.T) {
	// red_green is NOT a v1 family: v1 stays {parity} forever.
	a := rgArtifact(t, rgHonest(), StatusPass)
	a.SchemaVersion = EvidenceSchemaVersion
	if err := Validate(a); !errors.Is(err, ErrUnknownFamily) {
		t.Fatalf("red_green at v1: err = %v, want ErrUnknownFamily", err)
	}
	if got := knownFamilies(EvidenceSchemaVersion); len(got) != 1 || got[0] != NodeFamilyParity {
		t.Fatalf("v1 families = %v, want [parity]", got)
	}
	got := knownFamilies(EvidenceSchemaVersionV2)
	if len(got) != 2 || got[0] != NodeFamilyParity || got[1] != NodeFamilyRedGreen {
		t.Fatalf("v2 families = %v, want [parity red_green]", got)
	}
	// Unknown version stays rejected.
	a.SchemaVersion = 3
	if err := Validate(a); !errors.Is(err, ErrUnknownVersion) {
		t.Fatalf("v3: err = %v, want ErrUnknownVersion", err)
	}
}

func TestU161_003_RedGreenUnknownFieldRejected(t *testing.T) {
	a := rgArtifact(t, rgHonest(), StatusPass)
	a.VerifiedEvidence = json.RawMessage(`{"base_sha":"` + rgBase + `","head_sha":"` + rgHead + `","tests":[],"extra":1}`)
	raw, _ := json.Marshal(a)
	if _, _, err := DecodeAndValidate(raw); !errors.Is(err, ErrMalformed) {
		t.Fatalf("err = %v, want ErrMalformed", err)
	}
}
