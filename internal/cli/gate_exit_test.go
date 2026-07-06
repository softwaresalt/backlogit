package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/softwaresalt/backlogit/internal/core"
	corerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// TestGateExitError_BlockedMapsTo6 verifies an exit-1 refusal maps to the
// versioned blocked exit code regardless of the block-family outcome.
func TestGateExitError_BlockedMapsTo6(t *testing.T) {
	for _, outcome := range []string{"blocked", "requeued", "escalated"} {
		be := &corerrors.GateBlockedError{ItemID: "001.001-T", Outcome: outcome, OldStatus: "active"}
		ee := gateExitError(be)
		if ee == nil {
			t.Fatalf("outcome %q: expected *ExitError, got nil", outcome)
		}
		if ee.Code != ExitGateBlocked {
			t.Errorf("outcome %q: exit code = %d, want %d", outcome, ee.Code, ExitGateBlocked)
		}
	}
}

// TestGateExitError_ClassMapping verifies setup/config map to 7 and the
// retryable classes (timeout/in_progress) map to 8.
func TestGateExitError_ClassMapping(t *testing.T) {
	cases := []struct {
		class string
		want  int
	}{
		{"setup", ExitGateConfig},
		{"config", ExitGateConfig},
		{"timeout", ExitGateRetryable},
		{"in_progress", ExitGateRetryable},
	}
	for _, tc := range cases {
		ge := &corerrors.GateError{Class: tc.class, ItemID: "001.001-T", Message: "x"}
		ee := gateExitError(ge)
		if ee == nil {
			t.Fatalf("class %q: expected *ExitError, got nil", tc.class)
		}
		if ee.Code != tc.want {
			t.Errorf("class %q: exit code = %d, want %d", tc.class, ee.Code, tc.want)
		}
	}
}

// TestGateExitError_NonGatePassThrough verifies a non-gate error yields nil so
// the caller returns it unchanged (generic exit 1).
func TestGateExitError_NonGatePassThrough(t *testing.T) {
	if ee := gateExitError(errors.New("boom")); ee != nil {
		t.Fatalf("expected nil for non-gate error, got %+v", ee)
	}
	if ee := gateExitError(nil); ee != nil {
		t.Fatalf("expected nil for nil error, got %+v", ee)
	}
}

// TestRenderGateBlockedJSON verifies the blocked payload carries the outcome,
// retained/redirected status, repeated_failure, and the raw gate report.
func TestRenderGateBlockedJSON(t *testing.T) {
	be := &corerrors.GateBlockedError{
		ItemID:       "001.001-T",
		OldStatus:    "active",
		NewStatus:    "queued",
		Outcome:      "requeued",
		StateChanged: true,
		BaseRef:      "origin/main",
		HeadRef:      "HEAD",
		ExitCode:     1,
		ReportJSON:   []byte(`{"passed":false}`),
		Repeated:     &corerrors.GateRepeatedFailure{Count: 3, Threshold: 3, Reached: true, Action: "block"},
	}
	out, err := renderGateBlockedJSON(be.ItemID, be)
	if err != nil {
		t.Fatalf("renderGateBlockedJSON: %v", err)
	}
	var p gateJSONPayload
	if err := json.Unmarshal([]byte(out), &p); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if p.Outcome != "requeued" || p.NewStatus != "queued" || !p.StateChanged {
		t.Errorf("payload mismatch: %+v", p)
	}
	if p.RepeatedFailure == nil || p.RepeatedFailure.Action != "block" || !p.RepeatedFailure.Reached {
		t.Errorf("repeated_failure mismatch: %+v", p.RepeatedFailure)
	}
	if len(p.GateReport) == 0 {
		t.Errorf("expected gate_report to be preserved")
	}
	// Parity with the MCP surface: a redirect outcome (requeued/escalated) must
	// NOT carry the next-action menu, since the item was already moved to a
	// non-terminal status and move_to_non_terminal is non-actionable.
	if len(p.AllowedNext) != 0 {
		t.Errorf("requeued outcome must omit allowed_next_actions, got %v", p.AllowedNext)
	}
}

// TestRenderGateBlockedJSONPlainBlockOffersMenu verifies that a plain block
// (item retained its prior status) carries the next-action menu, matching the
// MCP gateBlockedResult contract which offers the menu only for outcome=blocked.
func TestRenderGateBlockedJSONPlainBlockOffersMenu(t *testing.T) {
	be := &corerrors.GateBlockedError{
		ItemID:    "001.001-T",
		OldStatus: "active",
		NewStatus: "active",
		Outcome:   "blocked",
		ExitCode:  1,
	}
	out, err := renderGateBlockedJSON(be.ItemID, be)
	if err != nil {
		t.Fatalf("renderGateBlockedJSON: %v", err)
	}
	var p gateJSONPayload
	if err := json.Unmarshal([]byte(out), &p); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	want := []string{"repair_and_retry", "move_to_non_terminal"}
	if len(p.AllowedNext) != len(want) || p.AllowedNext[0] != want[0] || p.AllowedNext[1] != want[1] {
		t.Errorf("blocked outcome must offer next-action menu, got %v", p.AllowedNext)
	}
}

// TestRenderGatePassJSON verifies the pass payload carries the hash and head SHA.
func TestRenderGatePassJSON(t *testing.T) {
	oc := &core.GateOutcome{
		ItemID:         "001.001-T",
		OldStatus:      "active",
		NewStatus:      "done",
		Outcome:        "passed",
		StateChanged:   true,
		BaseRef:        "origin/main",
		HeadRef:        "HEAD",
		HeadSHA:        "abc123",
		GateReportHash: "sha256:deadbeef",
	}
	out, err := renderGatePassJSON(oc.ItemID, oc)
	if err != nil {
		t.Fatalf("renderGatePassJSON: %v", err)
	}
	var p gateJSONPayload
	if err := json.Unmarshal([]byte(out), &p); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if p.Outcome != "passed" || p.NewStatus != "done" || p.HeadSHA != "abc123" || p.GateReportHash != "sha256:deadbeef" {
		t.Errorf("pass payload mismatch: %+v", p)
	}
}

// TestGateHumanMessage verifies the human summary reports the actual
// retained/post-transition status rather than a hard-coded literal.
func TestGateHumanMessage(t *testing.T) {
	cases := []struct {
		be   *corerrors.GateBlockedError
		want string
	}{
		{&corerrors.GateBlockedError{ItemID: "X", Outcome: "blocked", OldStatus: "review"}, "X: gate blocked — task remains review"},
		{&corerrors.GateBlockedError{ItemID: "X", Outcome: "requeued", OldStatus: "active"}, "X: gate blocked — task moved to queued (repeated gate failure)"},
		{&corerrors.GateBlockedError{ItemID: "X", Outcome: "escalated", OldStatus: "active"}, "X: gate blocked — task moved to blocked (escalated)"},
	}
	for _, tc := range cases {
		if got := gateHumanMessage(tc.be.ItemID, tc.be); got != tc.want {
			t.Errorf("gateHumanMessage(%q) = %q, want %q", tc.be.Outcome, got, tc.want)
		}
	}
}

// TestForceGatesRequiresReason exercises the operator-only force guardrail on the
// move command: --force-gates without --force-reason is rejected before any
// workspace is opened.
func TestForceGatesRequiresReason(t *testing.T) {
	cwd := t.TempDir()
	cmd := newMoveCommand(&cwd)
	cmd.SetArgs([]string{"001.001-T", "--status", "done", "--force-gates"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when --force-gates is set without --force-reason")
	}
	if got := err.Error(); got != "--force-gates requires --force-reason" {
		t.Errorf("error = %q, want the force-reason guardrail message", got)
	}
	_ = fmt.Sprint(err)
}
