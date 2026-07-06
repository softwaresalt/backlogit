package gate

import (
	"encoding/json"
	stderrors "errors"

	bkerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// ParseReport unmarshals the autoharness --json correction report. It returns
// ok=false when stdout is not valid JSON (exit 0 still counts as a pass even when
// this is false; a missing/malformed report only matters for exit 1 under
// enabled:true).
func ParseReport(stdout []byte) (report *GateReport, ok bool) {
	var r GateReport
	if len(stdout) == 0 {
		return nil, false
	}
	if err := json.Unmarshal(stdout, &r); err != nil {
		return nil, false
	}
	return &r, true
}

// Decide maps the outcome of a single gate run to a typed GateDecision. It is a
// pure function (no I/O) so it is exhaustively table-testable without a
// workspace.
//
//   - runErr is the error returned by GateRunner.Run: nil when the process ran and
//     exited; a wrapped ErrGateBinaryNotFound when the binary is unresolvable; a
//     wrapped ErrGateTimeout on a context-deadline kill; any other error for a
//     genuine IO/exec fault.
//   - res carries the process outcome (exit code, stdout, stderr) when runErr is nil.
//   - rf is the parsed repeated_failure object (nil when absent/unparseable).
//
// The retained status on a plain DecisionBlock is intentionally NOT decided here:
// the mapper has no view of the artifact's current status, so the caller resolves
// the retained status from the reread old_status.
func Decide(enabled EnabledMode, res GateResult, runErr error, rf *RepeatedFailure) GateDecision {
	// Failure-to-run classes first (runErr non-nil).
	switch {
	case stderrors.Is(runErr, bkerrors.ErrGateBinaryNotFound):
		if enabled == EnabledTrue {
			return GateDecision{Kind: DecisionError, ErrorClass: ErrorClassSetup}
		}
		// enabled:auto — fail open.
		return GateDecision{Kind: DecisionProceed}
	case stderrors.Is(runErr, bkerrors.ErrGateTimeout):
		return GateDecision{Kind: DecisionError, ErrorClass: ErrorClassTimeout, Stderr: res.Stderr}
	case runErr != nil:
		// Any other run error (IO/exec fault) refuses fail-closed even under auto:
		// we could not determine the gate outcome, so completing would be unsafe.
		return GateDecision{Kind: DecisionError, ErrorClass: ErrorClassConfig, Stderr: res.Stderr}
	}

	// Process ran and exited; classify by exit code.
	switch res.ExitCode {
	case 0:
		// exit 0 is a pass even if stdout is not JSON.
		return GateDecision{Kind: DecisionProceed, ReportJSON: res.Stdout, RepeatedFailure: rf}
	case 2:
		return GateDecision{
			Kind: DecisionError, ErrorClass: ErrorClassConfig,
			ExitCode: 2, ReportJSON: res.Stdout, Stderr: res.Stderr,
		}
	case 1:
		return decideBlocked(enabled, res, rf)
	default:
		// Any other exit code (including a platform-dependent -1 that slipped
		// past the runner's timeout detection) is a config error, never a block.
		return GateDecision{
			Kind: DecisionError, ErrorClass: ErrorClassConfig,
			ExitCode: res.ExitCode, ReportJSON: res.Stdout, Stderr: res.Stderr,
		}
	}
}

// decideBlocked handles the exit-1 (blocked) branch, consulting repeated_failure.
func decideBlocked(enabled EnabledMode, res GateResult, rf *RepeatedFailure) GateDecision {
	// A missing/malformed repeated_failure is a contract violation under strict
	// enforcement; under auto it degrades to a safe below-threshold block.
	if rf == nil {
		if enabled == EnabledTrue {
			return GateDecision{
				Kind: DecisionError, ErrorClass: ErrorClassConfig,
				ExitCode: 1, ReportJSON: res.Stdout, Stderr: res.Stderr,
			}
		}
		return GateDecision{Kind: DecisionBlock, ExitCode: 1, ReportJSON: res.Stdout, Stderr: res.Stderr}
	}

	if rf.Reached {
		switch rf.Action {
		case "block":
			return GateDecision{
				Kind: DecisionRedirectQueued, ExitCode: 1,
				ReportJSON: res.Stdout, Stderr: res.Stderr, RepeatedFailure: rf,
			}
		case "escalate":
			return GateDecision{
				Kind: DecisionRedirectBlocked, ExitCode: 1,
				ReportJSON: res.Stdout, Stderr: res.Stderr, RepeatedFailure: rf,
			}
		default:
			// Unknown action while reached is a contract violation under strict
			// enforcement; auto degrades to a plain block.
			if enabled == EnabledTrue {
				return GateDecision{
					Kind: DecisionError, ErrorClass: ErrorClassConfig,
					ExitCode: 1, ReportJSON: res.Stdout, Stderr: res.Stderr,
				}
			}
			return GateDecision{Kind: DecisionBlock, ExitCode: 1, ReportJSON: res.Stdout, Stderr: res.Stderr, RepeatedFailure: rf}
		}
	}

	// Below threshold: plain block (retained status resolved by the caller).
	return GateDecision{Kind: DecisionBlock, ExitCode: 1, ReportJSON: res.Stdout, Stderr: res.Stderr, RepeatedFailure: rf}
}
