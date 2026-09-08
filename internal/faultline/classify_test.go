package faultline

// 156.005-T (U4b): four-outcome classifier test suite plus SEC-04 safe
// diagnostics coverage. These tests live in the producer/consumer obligation
// owner package and exercise Classify against the four typed outcome
// categories, the contradictory-input rules, and the bounded/sanitized
// diagnostic guarantees.

import (
	"errors"
	"strings"
	"testing"
)

// TestU4bClassifyFourOutcomes covers the four-outcome contract plus the two
// contradictory-input rules against the production Classify function.
func TestU4bClassifyFourOutcomes(t *testing.T) {
	valid := mustMarshal(t, validPassArtifact(t))

	unknownVersion := validPassArtifact(t)
	unknownVersion.SchemaVersion = 99
	unknownVersionRaw := mustMarshal(t, unknownVersion)

	validationFailed := artifactWith(t, func() ParityEvidence {
		p := validParityPayload()
		p.DivergentFields = []string{"title"} // status=pass + divergent => cross-field violation
		return p
	}(), StatusPass, []string{"cli", "mcp", "internal"})
	validationFailedRaw := mustMarshal(t, validationFailed)

	tests := []struct {
		name        string
		present     bool
		raw         []byte
		wantOutcome Outcome
		wantErrIs   error // nil means no error expected
	}{
		{
			name:        "absent_empty_raw_not_present",
			present:     false,
			raw:         nil,
			wantOutcome: OutcomeAbsent,
			wantErrIs:   nil,
		},
		{
			name:        "valid_artifact",
			present:     true,
			raw:         valid,
			wantOutcome: OutcomeValid,
			wantErrIs:   nil,
		},
		{
			name:        "malformed_raw",
			present:     true,
			raw:         []byte("{ this is not json"),
			wantOutcome: OutcomeUnknownOrMalformed,
			wantErrIs:   ErrMalformed,
		},
		{
			name:        "unknown_version",
			present:     true,
			raw:         unknownVersionRaw,
			wantOutcome: OutcomeUnknownOrMalformed,
			wantErrIs:   ErrUnknownVersion,
		},
		{
			name:        "validation_failed",
			present:     true,
			raw:         validationFailedRaw,
			wantOutcome: OutcomeValidationFailed,
			wantErrIs:   ErrValidation,
		},
		{
			name:        "contradictory_absent_with_nonempty_raw",
			present:     false,
			raw:         []byte("{ not valid json at all"),
			wantOutcome: OutcomeUnknownOrMalformed,
			wantErrIs:   ErrMalformed,
		},
		{
			name:        "contradictory_present_with_empty_raw",
			present:     true,
			raw:         nil,
			wantOutcome: OutcomeAbsent,
			wantErrIs:   ErrMalformed,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			outcome, _, _, err := Classify(tc.present, tc.raw)
			if outcome != tc.wantOutcome {
				t.Errorf("outcome = %v, want %v", outcome, tc.wantOutcome)
			}
			if tc.wantErrIs == nil {
				if err != nil {
					t.Errorf("err = %v, want nil", err)
				}
				return
			}
			if !errors.Is(err, tc.wantErrIs) {
				t.Errorf("err = %v, want errors.Is(%v)", err, tc.wantErrIs)
			}
		})
	}
}

// TestU4bClassifyValidRoundTrip asserts a valid artifact yields the decoded
// envelope and concrete family payload.
func TestU4bClassifyValidRoundTrip(t *testing.T) {
	raw := mustMarshal(t, validPassArtifact(t))
	outcome, art, fp, err := Classify(true, raw)
	if err != nil {
		t.Fatalf("Classify valid: %v", err)
	}
	if outcome != OutcomeValid {
		t.Fatalf("outcome = %v, want OutcomeValid", outcome)
	}
	if art.NodeFamily != NodeFamilyParity {
		t.Errorf("node family = %q, want %q", art.NodeFamily, NodeFamilyParity)
	}
	if _, ok := fp.(*ParityEvidence); !ok {
		t.Errorf("family payload type = %T, want *ParityEvidence", fp)
	}
}

// TestU4bSafeDiagnosticsControlChars proves SEC-04: a control-char-laden
// malformed artifact yields a bounded, escaped diagnostic that never surfaces
// raw control bytes.
func TestU4bSafeDiagnosticsControlChars(t *testing.T) {
	// Embed raw control characters (NUL, BEL, ESC) in an otherwise-JSON-ish blob.
	raw := []byte("{\"schema_version\":1,\x00\x07\x1b\"node_family\":\"parity\" oops}")
	outcome, _, _, err := Classify(true, raw)
	if outcome != OutcomeUnknownOrMalformed {
		t.Fatalf("outcome = %v, want OutcomeUnknownOrMalformed", outcome)
	}
	if err == nil {
		t.Fatal("expected a diagnostic error, got nil")
	}
	msg := err.Error()
	for i, r := range msg {
		if (r < 0x20 && r != '\t') || r == 0x7f {
			t.Errorf("diagnostic contains raw control char %q at %d: %q", r, i, msg)
		}
	}
	if len(msg) > MaxDiagnosticRunes+64 {
		t.Errorf("diagnostic length %d exceeds bounded budget", len(msg))
	}
}

// TestU4bSafeDiagnosticsOversized proves an oversized input never produces an
// unbounded diagnostic string.
func TestU4bSafeDiagnosticsOversized(t *testing.T) {
	// A JSON string value that exceeds MaxStringBytes trips the bounded decoder.
	big := strings.Repeat("A", MaxStringBytes+16)
	raw := []byte("{\"schema_version\":1,\"validator_identity\":\"" + big + "\"}")
	outcome, _, _, err := Classify(true, raw)
	if outcome != OutcomeUnknownOrMalformed {
		t.Fatalf("outcome = %v, want OutcomeUnknownOrMalformed", outcome)
	}
	if err == nil {
		t.Fatal("expected a diagnostic error, got nil")
	}
	if !errors.Is(err, ErrMalformed) {
		t.Errorf("err = %v, want ErrMalformed", err)
	}
	if len(err.Error()) > MaxDiagnosticRunes+64 {
		t.Errorf("oversized-input diagnostic length %d exceeds bounded budget", len(err.Error()))
	}
}

// TestU4bSanitizeDiagnostic directly exercises the sanitizer used by both
// Classify and the cross-surface comparator.
func TestU4bSanitizeDiagnostic(t *testing.T) {
	got := SanitizeDiagnostic("line1\nline2\ttab\x00nul\x1besc")
	if strings.ContainsRune(got, 0x00) || strings.ContainsRune(got, 0x1b) {
		t.Errorf("sanitized output retained control chars: %q", got)
	}
	if !strings.Contains(got, `\n`) {
		t.Errorf("sanitized output did not escape newline: %q", got)
	}

	long := strings.Repeat("x", MaxDiagnosticRunes*4)
	trimmed := SanitizeDiagnostic(long)
	if len([]rune(trimmed)) > MaxDiagnosticRunes+len("...[truncated]") {
		t.Errorf("sanitized output not truncated: %d runes", len([]rune(trimmed)))
	}
}
