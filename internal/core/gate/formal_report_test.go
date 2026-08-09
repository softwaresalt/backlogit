package gate

import (
	stderrors "errors"
	"testing"

	bkerrors "github.com/softwaresalt/backlogit/internal/errors"
)

func TestValidateFormalReport_Valid(t *testing.T) {
	stdout := []byte(`{"reviewers":[{"persona":"Constitution Reviewer","decision":"pass"},{"persona":"Go Reviewer","decision":"pass"}],"repeated_failure":null}`)
	report, err := ValidateFormalReport(stdout)
	if err != nil {
		t.Fatalf("ValidateFormalReport() unexpected error: %v", err)
	}
	if len(report.Reviewers) != 2 {
		t.Fatalf("Reviewers len = %d, want 2", len(report.Reviewers))
	}
	digest, err := FormalReportDigest(*report)
	if err != nil {
		t.Fatalf("FormalReportDigest() unexpected error: %v", err)
	}
	if digest == "" {
		t.Fatal("FormalReportDigest() returned empty digest")
	}

	// Digest must be over the VALIDATED report, not raw bytes: a
	// byte-different-but-semantically-identical input (extra whitespace,
	// different field order) produces the SAME digest.
	stdout2 := []byte(`{
		"repeated_failure": null,
		"reviewers": [
			{"decision":"pass","persona":"Constitution Reviewer"},
			{"decision":"pass","persona":"Go Reviewer"}
		]
	}`)
	report2, err := ValidateFormalReport(stdout2)
	if err != nil {
		t.Fatalf("ValidateFormalReport() (variant) unexpected error: %v", err)
	}
	digest2, err := FormalReportDigest(*report2)
	if err != nil {
		t.Fatalf("FormalReportDigest() (variant) unexpected error: %v", err)
	}
	if digest != digest2 {
		t.Fatalf("digest = %q, variant digest = %q, want equal (validated-report digest, not raw bytes)", digest, digest2)
	}
}

func TestValidateFormalReport_EmptyStdoutRejected(t *testing.T) {
	_, err := ValidateFormalReport(nil)
	if !stderrors.Is(err, bkerrors.ErrFormalReportInvalid) {
		t.Fatalf("ValidateFormalReport(nil) error = %v, want ErrFormalReportInvalid", err)
	}
	_, err = ValidateFormalReport([]byte{})
	if !stderrors.Is(err, bkerrors.ErrFormalReportInvalid) {
		t.Fatalf("ValidateFormalReport([]byte{}) error = %v, want ErrFormalReportInvalid", err)
	}
}

func TestValidateFormalReport_NonJSONRejected(t *testing.T) {
	_, err := ValidateFormalReport([]byte("not json at all"))
	if !stderrors.Is(err, bkerrors.ErrFormalReportInvalid) {
		t.Fatalf("ValidateFormalReport(non-JSON) error = %v, want ErrFormalReportInvalid", err)
	}
}

func TestValidateFormalReport_MissingReviewersRejected(t *testing.T) {
	_, err := ValidateFormalReport([]byte(`{"repeated_failure":null}`))
	if !stderrors.Is(err, bkerrors.ErrFormalReportInvalid) {
		t.Fatalf("ValidateFormalReport(no reviewers) error = %v, want ErrFormalReportInvalid", err)
	}
	_, err = ValidateFormalReport([]byte(`{"reviewers":[]}`))
	if !stderrors.Is(err, bkerrors.ErrFormalReportInvalid) {
		t.Fatalf("ValidateFormalReport(empty reviewers) error = %v, want ErrFormalReportInvalid", err)
	}
}

func TestValidateFormalReport_MissingRequiredFieldRejected(t *testing.T) {
	tests := []struct {
		name   string
		stdout string
	}{
		{"missing persona", `{"reviewers":[{"decision":"pass"}]}`},
		{"missing decision", `{"reviewers":[{"persona":"Go Reviewer"}]}`},
		{"blank persona", `{"reviewers":[{"persona":"  ","decision":"pass"}]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ValidateFormalReport([]byte(tt.stdout))
			if !stderrors.Is(err, bkerrors.ErrFormalReportInvalid) {
				t.Fatalf("ValidateFormalReport(%s) error = %v, want ErrFormalReportInvalid", tt.name, err)
			}
		})
	}
}

// TestValidateFormalReport_ExitZeroEmptyOutputStillRejected characterizes the
// exact gap the spike identified: today's broker maps exit 0 to
// DecisionProceed even when stdout is empty or non-JSON
// (internal/core/gate/decision.go:56-60), so authenticating that event alone
// would prove only that the broker ran, not that a complete attributed
// review occurred. The formal validator must reject exactly this case.
func TestValidateFormalReport_ExitZeroEmptyOutputStillRejected(t *testing.T) {
	res := GateResult{ExitCode: 0, Stdout: []byte{}}
	decision := Decide(EnabledAuto, res, nil, nil)
	if decision.Kind != DecisionProceed {
		t.Fatalf("Decide() kind = %v, want DecisionProceed (today's permissive behavior, unchanged)", decision.Kind)
	}
	// The formal validator, applied separately to the same empty stdout,
	// must still refuse.
	_, err := ValidateFormalReport(decision.ReportJSON)
	if !stderrors.Is(err, bkerrors.ErrFormalReportInvalid) {
		t.Fatalf("ValidateFormalReport(exit-0 empty stdout) error = %v, want ErrFormalReportInvalid", err)
	}
}
