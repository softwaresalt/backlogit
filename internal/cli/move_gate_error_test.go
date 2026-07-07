package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/spf13/cobra"

	corerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// TestMoveGateError_JSONEmitsGateErrorPayload pins the F7 (083.004-T) routing:
// under --json, moveGateError renders the structured *GateError payload
// (outcome:"error") and returns the versioned *ExitError (7 config / 8 timeout).
func TestMoveGateError_JSONEmitsGateErrorPayload(t *testing.T) {
	cases := []struct {
		name          string
		ge            *corerrors.GateError
		wantExit      int
		wantRetryable bool
	}{
		{
			name:          "config maps to exit 7 without retryable",
			ge:            &corerrors.GateError{Class: "config", ItemID: "001.001-T", Message: "bad config", Err: corerrors.ErrGateConfig},
			wantExit:      ExitGateConfig,
			wantRetryable: false,
		},
		{
			name:          "timeout maps to exit 8 with retryable",
			ge:            &corerrors.GateError{Class: "timeout", ItemID: "001.001-T", Message: "timed out", Err: corerrors.ErrGateTimeout},
			wantExit:      ExitGateRetryable,
			wantRetryable: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := &cobra.Command{}
			var out bytes.Buffer
			cmd.SetOut(&out)

			retErr := moveGateError(cmd, tc.ge.ItemID, tc.ge, true)

			var ee *ExitError
			if !errors.As(retErr, &ee) {
				t.Fatalf("want *ExitError, got %T", retErr)
			}
			if ee.Code != tc.wantExit {
				t.Errorf("exit code = %d, want %d", ee.Code, tc.wantExit)
			}
			var p gateJSONPayload
			if err := json.Unmarshal(out.Bytes(), &p); err != nil {
				t.Fatalf("stdout is not a JSON payload (%q): %v", out.String(), err)
			}
			if p.Outcome != "error" || p.Error == "" {
				t.Errorf("payload mismatch: %+v", p)
			}
			if p.Retryable != tc.wantRetryable {
				t.Errorf("retryable = %v, want %v", p.Retryable, tc.wantRetryable)
			}
		})
	}
}

// TestMoveGateError_BlockedTakesPrecedence confirms the blocked-before-error
// precedence (exit 6 before 7/8) is preserved: a *GateBlockedError under --json
// still renders the blocked payload, never the error payload.
func TestMoveGateError_BlockedTakesPrecedence(t *testing.T) {
	be := &corerrors.GateBlockedError{ItemID: "001.001-T", Outcome: "blocked", OldStatus: "active", NewStatus: "active", ExitCode: 1}
	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)

	retErr := moveGateError(cmd, be.ItemID, be, true)
	var ee *ExitError
	if !errors.As(retErr, &ee) || ee.Code != ExitGateBlocked {
		t.Fatalf("want *ExitError exit %d, got %+v", ExitGateBlocked, retErr)
	}
	var p gateJSONPayload
	if err := json.Unmarshal(out.Bytes(), &p); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if p.Outcome != "blocked" {
		t.Errorf("blocked precedence violated: outcome = %q, want blocked", p.Outcome)
	}
}
