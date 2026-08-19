package cli

// 144.007-T (U7): CLI governance exit-code parity tests for guard 1 and
// guard 2. These confirm the stable per-sentinel CLI exit contract:
// ErrShipmentShippedRequiresEnvelope → exit 9 (ExitShipmentGovernance).
// ErrArchiveShippedRequiresEvent      → exit 9 (ExitShipmentGovernance).

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	corerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// TestShipmentGovernanceExitError_MapsEnvelopeSentinel pins exit code 9 for
// ErrShipmentShippedRequiresEnvelope (guard 1).
func TestShipmentGovernanceExitError_MapsEnvelopeSentinel(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{"bare sentinel", corerrors.ErrShipmentShippedRequiresEnvelope},
		{"wrapped sentinel", fmt.Errorf("move shipment 001-S: %w", corerrors.ErrShipmentShippedRequiresEnvelope)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ee := shipmentGovernanceExitError(tc.err)
			require.NotNil(t, ee, "shipmentGovernanceExitError must map ErrShipmentShippedRequiresEnvelope to *ExitError")
			assert.Equal(t, ExitShipmentGovernance, ee.Code,
				"ErrShipmentShippedRequiresEnvelope must map to exit code %d", ExitShipmentGovernance)
		})
	}
}

// TestShipmentGovernanceExitError_MapsArchiveSentinel pins exit code 9 for
// ErrArchiveShippedRequiresEvent (guard 2).
func TestShipmentGovernanceExitError_MapsArchiveSentinel(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{"bare sentinel", corerrors.ErrArchiveShippedRequiresEvent},
		{"wrapped sentinel", fmt.Errorf("archive shipment 001-S: %w", corerrors.ErrArchiveShippedRequiresEvent)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ee := shipmentGovernanceExitError(tc.err)
			require.NotNil(t, ee, "shipmentGovernanceExitError must map ErrArchiveShippedRequiresEvent to *ExitError")
			assert.Equal(t, ExitShipmentGovernance, ee.Code,
				"ErrArchiveShippedRequiresEvent must map to exit code %d", ExitShipmentGovernance)
		})
	}
}

// TestShipmentGovernanceExitError_NilForNonGovernanceSentinels verifies that
// shipmentGovernanceExitError returns nil for unrelated errors.
func TestShipmentGovernanceExitError_NilForNonGovernanceSentinels(t *testing.T) {
	unrelated := []error{
		errors.New("some other error"),
		corerrors.ErrShipmentConflict,
		corerrors.ErrNotFound,
		corerrors.ErrValidation,
	}
	for _, err := range unrelated {
		ee := shipmentGovernanceExitError(err)
		assert.Nil(t, ee, "shipmentGovernanceExitError must return nil for unrelated error: %v", err)
	}
}

// TestExitCodeFor_GovernanceSentinelIsNine verifies that ExitCodeFor returns 9
// for an ExitError carrying ExitShipmentGovernance.
func TestExitCodeFor_GovernanceSentinelIsNine(t *testing.T) {
	ee := &ExitError{Code: ExitShipmentGovernance, Msg: "governance test"}
	assert.Equal(t, ExitShipmentGovernance, ExitCodeFor(ee),
		"ExitCodeFor must honor ExitShipmentGovernance = %d", ExitShipmentGovernance)
}
