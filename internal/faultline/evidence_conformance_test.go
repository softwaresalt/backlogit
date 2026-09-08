package faultline

// 156.006-T (U4a-behavior): conformance tests for the fault-line evidence
// contract behavior. These are RED before the function bodies land in
// evidence.go and turn GREEN once Canonical/DecodeAndValidate/Validate plus the
// registry (RegisterFamily/knownFamilies/freeze) and the parity family payload
// are implemented.

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

var updateGolden = flag.Bool("update", false, "update golden files in testdata")

// validParityPayload returns a conformant, status=pass parity payload whose
// surface set matches the pass-status envelope applicability set.
func validParityPayload() ParityEvidence {
	return ParityEvidence{
		ScenarioID:         "list-default",
		Dimension:          "list-output",
		Surfaces:           []string{"cli", "mcp", "internal"},
		DivergentFields:    []string{},
		ExpectedDivergence: false,
		TrackedDefect:      "",
		Detail:             "all surfaces agree",
	}
}

// artifactWith builds an EvidenceArtifact whose verified_evidence carries the
// JSON encoding of p, with a pass-shaped envelope by default.
func artifactWith(t *testing.T, p ParityEvidence, status string, applicability []string) EvidenceArtifact {
	t.Helper()
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal parity payload: %v", err)
	}
	return EvidenceArtifact{
		SchemaVersion:     EvidenceSchemaVersion,
		ProducingTask:     "156.006-T",
		ProducingCommit:   "e97e0263",
		NodeFamily:        NodeFamilyParity,
		Applicability:     applicability,
		ValidatorIdentity: "faultline-conformance",
		VerifiedEvidence:  raw,
		Status:            status,
	}
}

// validPassArtifact returns a fully conformant status=pass artifact.
func validPassArtifact(t *testing.T) EvidenceArtifact {
	t.Helper()
	return artifactWith(t, validParityPayload(), StatusPass, []string{"cli", "mcp", "internal"})
}

func mustMarshal(t *testing.T, a EvidenceArtifact) []byte {
	t.Helper()
	b, err := json.Marshal(a)
	if err != nil {
		t.Fatalf("marshal artifact: %v", err)
	}
	return b
}

func TestU4aBehaviorCanonicalByteStable(t *testing.T) {
	a := validPassArtifact(t)
	got, err := a.Canonical()
	if err != nil {
		t.Fatalf("Canonical: %v", err)
	}

	goldenPath := filepath.Join("testdata", "parity_v1.golden.json")
	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatalf("mkdir testdata: %v", err)
		}
		if err := os.WriteFile(goldenPath, got, 0o644); err != nil { //nolint:gosec // golden fixture.
			t.Fatalf("write golden: %v", err)
		}
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden %s: %v (regenerate with -update)", goldenPath, err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("canonical bytes mismatch\n got: %q\nwant: %q", got, want)
	}

	// Byte-stability: re-canonicalizing yields identical bytes.
	again, err := a.Canonical()
	if err != nil {
		t.Fatalf("Canonical (second): %v", err)
	}
	if !bytes.Equal(got, again) {
		t.Errorf("Canonical not byte-stable across calls")
	}
}

func TestU4aBehaviorDecodeAndValidate(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		data := mustMarshal(t, validPassArtifact(t))
		a, fp, err := DecodeAndValidate(data)
		if err != nil {
			t.Fatalf("DecodeAndValidate valid: %v", err)
		}
		if a.NodeFamily != NodeFamilyParity {
			t.Errorf("node family = %q, want %q", a.NodeFamily, NodeFamilyParity)
		}
		if fp == nil {
			t.Fatal("family payload is nil")
		}
		if _, ok := fp.(*ParityEvidence); !ok {
			t.Errorf("family payload type = %T, want *ParityEvidence", fp)
		}
	})

	t.Run("malformed", func(t *testing.T) {
		_, _, err := DecodeAndValidate([]byte("{ this is not json"))
		if !errors.Is(err, ErrMalformed) {
			t.Errorf("err = %v, want ErrMalformed", err)
		}
	})

	t.Run("unknown_version", func(t *testing.T) {
		a := validPassArtifact(t)
		a.SchemaVersion = 99
		_, _, err := DecodeAndValidate(mustMarshal(t, a))
		if !errors.Is(err, ErrUnknownVersion) {
			t.Errorf("err = %v, want ErrUnknownVersion", err)
		}
	})

	t.Run("unknown_family", func(t *testing.T) {
		a := validPassArtifact(t)
		a.NodeFamily = "bogus"
		_, _, err := DecodeAndValidate(mustMarshal(t, a))
		if !errors.Is(err, ErrUnknownFamily) {
			t.Errorf("err = %v, want ErrUnknownFamily", err)
		}
	})

	t.Run("validation_failed", func(t *testing.T) {
		// status=pass but divergent fields present => cross-field violation.
		p := validParityPayload()
		p.DivergentFields = []string{"title"}
		a := artifactWith(t, p, StatusPass, []string{"cli", "mcp", "internal"})
		_, _, err := DecodeAndValidate(mustMarshal(t, a))
		if !errors.Is(err, ErrValidation) {
			t.Errorf("err = %v, want ErrValidation", err)
		}
	})
}

func TestU4aBehaviorValidateStateMatrix(t *testing.T) {
	surfaces := []string{"cli", "mcp", "internal"}

	tests := []struct {
		name    string
		mutate  func(p *ParityEvidence)
		status  string
		applic  []string
		wantErr bool
	}{
		{
			name:    "pass_clean",
			mutate:  func(p *ParityEvidence) {},
			status:  StatusPass,
			applic:  surfaces,
			wantErr: false,
		},
		{
			name: "pass_with_divergent_fields_rejected",
			mutate: func(p *ParityEvidence) {
				p.DivergentFields = []string{"title"}
			},
			status:  StatusPass,
			applic:  surfaces,
			wantErr: true,
		},
		{
			name: "pass_with_expected_divergence_rejected",
			mutate: func(p *ParityEvidence) {
				p.ExpectedDivergence = true
			},
			status:  StatusPass,
			applic:  surfaces,
			wantErr: true,
		},
		{
			name: "report_only_complete_ok",
			mutate: func(p *ParityEvidence) {
				p.ExpectedDivergence = true
				p.TrackedDefect = "166-F"
				p.DivergentFields = []string{"title"}
			},
			status:  StatusReportOnly,
			applic:  surfaces,
			wantErr: false,
		},
		{
			name: "report_only_missing_tracked_defect_rejected",
			mutate: func(p *ParityEvidence) {
				p.ExpectedDivergence = true
				p.DivergentFields = []string{"title"}
			},
			status:  StatusReportOnly,
			applic:  surfaces,
			wantErr: true,
		},
		{
			name: "report_only_missing_divergent_fields_rejected",
			mutate: func(p *ParityEvidence) {
				p.ExpectedDivergence = true
				p.TrackedDefect = "166-F"
			},
			status:  StatusReportOnly,
			applic:  surfaces,
			wantErr: true,
		},
		{
			name: "report_only_expected_false_rejected",
			mutate: func(p *ParityEvidence) {
				p.ExpectedDivergence = false
				p.TrackedDefect = "166-F"
				p.DivergentFields = []string{"title"}
			},
			status:  StatusReportOnly,
			applic:  surfaces,
			wantErr: true,
		},
		{
			name: "fail_with_divergent_fields_ok",
			mutate: func(p *ParityEvidence) {
				p.DivergentFields = []string{"title"}
			},
			status:  StatusFail,
			applic:  surfaces,
			wantErr: false,
		},
		{
			name: "fail_without_divergent_fields_rejected",
			mutate: func(p *ParityEvidence) {
				p.DivergentFields = []string{}
			},
			status:  StatusFail,
			applic:  surfaces,
			wantErr: true,
		},
		{
			name: "fail_with_expected_divergence_rejected",
			mutate: func(p *ParityEvidence) {
				p.DivergentFields = []string{"title"}
				p.ExpectedDivergence = true
			},
			status:  StatusFail,
			applic:  surfaces,
			wantErr: true,
		},
		{
			name: "expected_divergence_requires_report_only",
			mutate: func(p *ParityEvidence) {
				p.ExpectedDivergence = true
				p.DivergentFields = []string{"title"}
				p.TrackedDefect = "166-F"
			},
			status:  StatusFail,
			applic:  surfaces,
			wantErr: true,
		},
		{
			name:    "surface_set_mismatch_rejected",
			mutate:  func(p *ParityEvidence) {},
			status:  StatusPass,
			applic:  []string{"cli", "mcp"},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := validParityPayload()
			tc.mutate(&p)
			a := artifactWith(t, p, tc.status, tc.applic)
			err := Validate(a)
			if tc.wantErr {
				if !errors.Is(err, ErrValidation) {
					t.Errorf("err = %v, want ErrValidation", err)
				}
			} else if err != nil {
				t.Errorf("unexpected err: %v", err)
			}
		})
	}
}

func TestU4aBehaviorTrackedDefectFormat(t *testing.T) {
	valid := []string{"166-F", "156.006-T", "123.456.789-AB", "138-S"}
	invalid := []string{"free form text", "12-F", "166", "166-abc", "166-", "-F", "166.-F"}

	for _, td := range valid {
		p := validParityPayload()
		p.ExpectedDivergence = true
		p.TrackedDefect = td
		p.DivergentFields = []string{"title"}
		a := artifactWith(t, p, StatusReportOnly, []string{"cli", "mcp", "internal"})
		if err := Validate(a); err != nil {
			t.Errorf("tracked_defect %q: unexpected err %v", td, err)
		}
	}

	for _, td := range invalid {
		p := validParityPayload()
		p.ExpectedDivergence = true
		p.TrackedDefect = td
		p.DivergentFields = []string{"title"}
		a := artifactWith(t, p, StatusReportOnly, []string{"cli", "mcp", "internal"})
		if err := Validate(a); !errors.Is(err, ErrValidation) {
			t.Errorf("tracked_defect %q: err = %v, want ErrValidation", td, err)
		}
	}
}

func TestU4aBehaviorRegisterFamily(t *testing.T) {
	newReg := func() *registry {
		return &registry{factories: make(map[int]map[string]func() FamilyPayload)}
	}
	factory := func() FamilyPayload { return &ParityEvidence{} }

	t.Run("nil_factory_rejected", func(t *testing.T) {
		r := newReg()
		if err := r.register(1, "parity", nil); err == nil {
			t.Error("nil factory: want error, got nil")
		}
	})

	t.Run("duplicate_rejected", func(t *testing.T) {
		r := newReg()
		if err := r.register(1, "parity", factory); err != nil {
			t.Fatalf("first register: %v", err)
		}
		if err := r.register(1, "parity", factory); err == nil {
			t.Error("duplicate register: want error, got nil")
		}
	})

	t.Run("post_freeze_rejected", func(t *testing.T) {
		r := newReg()
		if err := r.register(1, "parity", factory); err != nil {
			t.Fatalf("register: %v", err)
		}
		if err := r.freezeAgainst(map[int][]string{1: {"parity"}}); err != nil {
			t.Fatalf("freeze: %v", err)
		}
		if err := r.register(1, "parity", factory); err == nil {
			t.Error("post-freeze register: want error, got nil")
		}
	})

	t.Run("global_duplicate_parity_rejected", func(t *testing.T) {
		// The global registry already registered parity in init().
		if err := RegisterFamily(EvidenceSchemaVersion, NodeFamilyParity, factory); err == nil {
			t.Error("duplicate global register: want error, got nil")
		}
	})
}

func TestU4aBehaviorFreezeManifest(t *testing.T) {
	factory := func() FamilyPayload { return &ParityEvidence{} }
	manifest := map[int][]string{1: {"parity"}}

	t.Run("exact_match_ok", func(t *testing.T) {
		r := &registry{factories: make(map[int]map[string]func() FamilyPayload)}
		if err := r.register(1, "parity", factory); err != nil {
			t.Fatalf("register: %v", err)
		}
		if err := r.freezeAgainst(manifest); err != nil {
			t.Errorf("freeze exact: %v", err)
		}
	})

	t.Run("missing_factory_rejected", func(t *testing.T) {
		r := &registry{factories: make(map[int]map[string]func() FamilyPayload)}
		if err := r.freezeAgainst(manifest); err == nil {
			t.Error("freeze missing: want error, got nil")
		}
	})

	t.Run("extra_factory_rejected", func(t *testing.T) {
		r := &registry{factories: make(map[int]map[string]func() FamilyPayload)}
		if err := r.register(1, "parity", factory); err != nil {
			t.Fatalf("register parity: %v", err)
		}
		if err := r.register(1, "extra", factory); err != nil {
			t.Fatalf("register extra: %v", err)
		}
		if err := r.freezeAgainst(manifest); err == nil {
			t.Error("freeze extra: want error, got nil")
		}
	})

	t.Run("global_freeze_and_read_race_clean", func(t *testing.T) {
		var wg sync.WaitGroup
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				freeze()
				_ = knownFamilies(EvidenceSchemaVersion)
			}()
		}
		wg.Wait()
		fams := knownFamilies(EvidenceSchemaVersion)
		if len(fams) != 1 || fams[0] != NodeFamilyParity {
			t.Errorf("knownFamilies = %v, want [parity]", fams)
		}
	})
}

func TestU4aBehaviorBoundedDecode(t *testing.T) {
	oversized := bytes.Repeat([]byte("a"), MaxArtifactBytes+1)
	_, _, err := DecodeAndValidate(oversized)
	if !errors.Is(err, ErrMalformed) {
		t.Errorf("oversized: err = %v, want ErrMalformed", err)
	}
}

func TestU4aBehaviorDuplicateKeys(t *testing.T) {
	t.Run("exact_duplicate_key", func(t *testing.T) {
		base := string(mustMarshal(t, validPassArtifact(t)))
		// Inject a duplicate top-level status key.
		dup := strings.Replace(base, `"status":"pass"`, `"status":"pass","status":"fail"`, 1)
		_, _, err := DecodeAndValidate([]byte(dup))
		if !errors.Is(err, ErrMalformed) {
			t.Errorf("exact duplicate key: err = %v, want ErrMalformed", err)
		}
	})

	t.Run("case_fold_duplicate_key", func(t *testing.T) {
		raw := `{"schema_version":1,"Schema_Version":1,"producing_task":"x",` +
			`"producing_commit":"y","node_family":"parity","applicability":["cli"],` +
			`"validator_identity":"z","verified_evidence":{},"status":"pass"}`
		_, _, err := DecodeAndValidate([]byte(raw))
		if !errors.Is(err, ErrMalformed) {
			t.Errorf("case-fold duplicate key: err = %v, want ErrMalformed", err)
		}
	})
}

// TestU4aBehaviorValidateMiscasedPayloadKey is the SEC-01 conformance test for
// F-C1: Validate() called with a VerifiedEvidence payload that contains a
// miscased field key (e.g. "Scenario_ID" instead of "scenario_id") must return
// ErrMalformed. This proves that decodePayload applies the same exact-casing
// enforcement as DecodeAndValidate (via preScan + structJSONTags), closing the
// gap where plain json.Unmarshal would silently accept case-variant keys via
// encoding/json's case-insensitive fallback matching.
func TestU4aBehaviorValidateMiscasedPayloadKey(t *testing.T) {
	// Hand-craft a VerifiedEvidence object with "Scenario_ID" (capital S + ID)
	// instead of the correct "scenario_id" tag.
	miscasedPayload := json.RawMessage(
		`{"Scenario_ID":"list-default","dimension":"list-output",` +
			`"surfaces":["cli","mcp","internal"],"divergent_fields":[],` +
			`"expected_divergence":false,"tracked_defect":"","detail":"all surfaces agree"}`,
	)
	a := EvidenceArtifact{
		SchemaVersion:     EvidenceSchemaVersion,
		ProducingTask:     "156.006-T",
		ProducingCommit:   "e97e0263",
		NodeFamily:        NodeFamilyParity,
		Applicability:     []string{"cli", "mcp", "internal"},
		ValidatorIdentity: "faultline-conformance",
		VerifiedEvidence:  miscasedPayload,
		Status:            StatusPass,
	}
	err := Validate(a)
	if !errors.Is(err, ErrMalformed) {
		t.Errorf("miscased payload key: err = %v, want ErrMalformed (SEC-01 exact-casing must reject Scenario_ID)", err)
	}
}
