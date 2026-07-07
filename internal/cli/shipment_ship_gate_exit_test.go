package cli

import (
	"bytes"
	"errors"
	"testing"

	"github.com/spf13/cobra"

	corerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// TestShipmentShipGateError_PreservesExitCodes pins F5 (083.003-T) at the CLI
// seam: a shipment-completion gate refusal returned by core.ShipShipment must be
// mapped to the versioned gate exit code (6/7/8) instead of collapsing to the
// generic 1. Before this wiring, `shipment ship` wrapped the typed *GateError
// with fmt.Errorf and main's ExitCodeFor fell through to 1.
func TestShipmentShipGateError_PreservesExitCodes(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"config maps to 7", &corerrors.GateError{Class: "config", ItemID: "001-S", Message: "x"}, ExitGateConfig},
		{"setup maps to 7", &corerrors.GateError{Class: "setup", ItemID: "001-S", Message: "x"}, ExitGateConfig},
		{"timeout maps to 8", &corerrors.GateError{Class: "timeout", ItemID: "001-S", Message: "x"}, ExitGateRetryable},
		{"in_progress maps to 8", &corerrors.GateError{Class: "in_progress", ItemID: "001-S", Message: "x"}, ExitGateRetryable},
		{"blocked maps to 6", &corerrors.GateBlockedError{ItemID: "001-S", Outcome: "blocked", OldStatus: "active"}, ExitGateBlocked},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := &cobra.Command{}
			cmd.SetErr(&bytes.Buffer{})
			ee := shipmentShipGateError(cmd, tc.err)
			if ee == nil {
				t.Fatalf("expected *ExitError, got nil")
			}
			if ee.Code != tc.want {
				t.Errorf("exit code = %d, want %d", ee.Code, tc.want)
			}
			if !cmd.SilenceErrors {
				t.Errorf("expected SilenceErrors set so cobra does not double-print the error")
			}
		})
	}
}

// TestShipmentShipGateError_NonGatePassThrough verifies a non-gate error yields
// nil so the caller wraps it as a generic failure (exit 1).
func TestShipmentShipGateError_NonGatePassThrough(t *testing.T) {
	cmd := &cobra.Command{}
	if ee := shipmentShipGateError(cmd, errors.New("disk full")); ee != nil {
		t.Fatalf("expected nil for non-gate error, got %+v", ee)
	}
}
