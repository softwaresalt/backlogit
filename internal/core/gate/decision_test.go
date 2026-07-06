package gate

import (
	"fmt"
	"testing"

	bkerrors "github.com/softwaresalt/backlogit/internal/errors"
)

func TestDecide(t *testing.T) {
	rfBlockReached := &RepeatedFailure{Count: 3, Threshold: 3, Reached: true, Action: "block"}
	rfEscalateReached := &RepeatedFailure{Count: 5, Threshold: 3, Reached: true, Action: "escalate"}
	rfBelow := &RepeatedFailure{Count: 1, Threshold: 3, Reached: false, Action: "block"}
	rfUnknownAction := &RepeatedFailure{Count: 3, Threshold: 3, Reached: true, Action: "quarantine"}

	tests := []struct {
		name      string
		enabled   EnabledMode
		res       GateResult
		runErr    error
		rf        *RepeatedFailure
		wantKind  DecisionKind
		wantClass ErrorClass
	}{
		{
			name:    "binary not found under auto fails open",
			enabled: EnabledAuto, runErr: bkerrors.ErrGateBinaryNotFound,
			wantKind: DecisionProceed,
		},
		{
			name:    "binary not found under true is setup error",
			enabled: EnabledTrue, runErr: bkerrors.ErrGateBinaryNotFound,
			wantKind: DecisionError, wantClass: ErrorClassSetup,
		},
		{
			name:    "timeout is timeout error (auto)",
			enabled: EnabledAuto, runErr: bkerrors.ErrGateTimeout,
			wantKind: DecisionError, wantClass: ErrorClassTimeout,
		},
		{
			name:    "timeout is timeout error (true)",
			enabled: EnabledTrue, runErr: bkerrors.ErrGateTimeout,
			wantKind: DecisionError, wantClass: ErrorClassTimeout,
		},
		{
			name:    "generic run error refuses config-class even under auto",
			enabled: EnabledAuto, runErr: fmt.Errorf("boom"),
			wantKind: DecisionError, wantClass: ErrorClassConfig,
		},
		{
			name:    "exit 0 proceeds (auto)",
			enabled: EnabledAuto, res: GateResult{ExitCode: 0, Stdout: []byte("not json")},
			wantKind: DecisionProceed,
		},
		{
			name:    "exit 0 proceeds (true)",
			enabled: EnabledTrue, res: GateResult{ExitCode: 0},
			wantKind: DecisionProceed,
		},
		{
			name:    "exit 2 is config error",
			enabled: EnabledAuto, res: GateResult{ExitCode: 2},
			wantKind: DecisionError, wantClass: ErrorClassConfig,
		},
		{
			name:    "exit 1 reached+block -> redirect queued",
			enabled: EnabledTrue, res: GateResult{ExitCode: 1}, rf: rfBlockReached,
			wantKind: DecisionRedirectQueued,
		},
		{
			name:    "exit 1 reached+escalate -> redirect blocked",
			enabled: EnabledTrue, res: GateResult{ExitCode: 1}, rf: rfEscalateReached,
			wantKind: DecisionRedirectBlocked,
		},
		{
			name:    "exit 1 below threshold -> block",
			enabled: EnabledTrue, res: GateResult{ExitCode: 1}, rf: rfBelow,
			wantKind: DecisionBlock,
		},
		{
			name:    "exit 1 missing rf under true -> config error",
			enabled: EnabledTrue, res: GateResult{ExitCode: 1}, rf: nil,
			wantKind: DecisionError, wantClass: ErrorClassConfig,
		},
		{
			name:    "exit 1 missing rf under auto -> block",
			enabled: EnabledAuto, res: GateResult{ExitCode: 1}, rf: nil,
			wantKind: DecisionBlock,
		},
		{
			name:    "exit 1 reached+unknown action under true -> config error",
			enabled: EnabledTrue, res: GateResult{ExitCode: 1}, rf: rfUnknownAction,
			wantKind: DecisionError, wantClass: ErrorClassConfig,
		},
		{
			name:    "exit 1 reached+unknown action under auto -> block",
			enabled: EnabledAuto, res: GateResult{ExitCode: 1}, rf: rfUnknownAction,
			wantKind: DecisionBlock,
		},
		{
			name:    "unexpected exit code -> config error (never block)",
			enabled: EnabledAuto, res: GateResult{ExitCode: -1},
			wantKind: DecisionError, wantClass: ErrorClassConfig,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Decide(tt.enabled, tt.res, tt.runErr, tt.rf)
			if got.Kind != tt.wantKind {
				t.Fatalf("Kind = %v, want %v", got.Kind, tt.wantKind)
			}
			if tt.wantClass != "" && got.ErrorClass != tt.wantClass {
				t.Fatalf("ErrorClass = %q, want %q", got.ErrorClass, tt.wantClass)
			}
		})
	}
}

func TestParseReport(t *testing.T) {
	report, ok := ParseReport([]byte(`{"repeated_failure":{"count":2,"threshold":3,"reached":false,"action":"block"}}`))
	if !ok {
		t.Fatal("expected ok for valid JSON")
	}
	if report.RepeatedFailure == nil || report.RepeatedFailure.Count != 2 {
		t.Fatalf("unexpected report: %+v", report)
	}

	if _, ok := ParseReport([]byte("not json")); ok {
		t.Fatal("expected ok=false for invalid JSON")
	}
	if _, ok := ParseReport(nil); ok {
		t.Fatal("expected ok=false for empty stdout")
	}
}
