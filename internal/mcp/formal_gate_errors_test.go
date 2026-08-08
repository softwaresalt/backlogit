package mcp

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	corerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// TestGateErrorResult_FormalGateKeyAbsent verifies a refusal caused by an
// absent BACKLOGIT_GATE_EVIDENCE_KEY maps to a structured error naming the
// exact missing environment variable, the minimum key requirement, and
// retryable:false (106-F F1/U8).
func TestGateErrorResult_FormalGateKeyAbsent(t *testing.T) {
	err := fmt.Errorf("%w: %w", corerrors.ErrFormalGateRequired, corerrors.ErrGateKeyAbsent)
	result, handled := gateErrorResult(err, "done")
	require.True(t, handled, "ErrFormalGateRequired must be handled")
	require.True(t, result.IsError)
	body := resultBody(t, result)
	require.Equal(t, "formal_gate_key_absent", body["error"])
	require.Equal(t, "BACKLOGIT_GATE_EVIDENCE_KEY", body["missing_env_var"])
	require.Equal(t, false, body["retryable"])
	require.Contains(t, body["remediation"], "BACKLOGIT_GATE_EVIDENCE_KEY")
	require.EqualValues(t, 32, body["min_key_bytes"])
}

// TestGateErrorResult_FormalGateKeyInvalid verifies a refusal caused by an
// invalid (present but malformed/too-short) key maps to a distinct error_type
// from the absent-key case, still naming the same environment variable and
// minimum requirement.
func TestGateErrorResult_FormalGateKeyInvalid(t *testing.T) {
	err := fmt.Errorf("%w: %w", corerrors.ErrFormalGateRequired, corerrors.ErrGateKeyInvalid)
	result, handled := gateErrorResult(err, "done")
	require.True(t, handled)
	body := resultBody(t, result)
	require.Equal(t, "formal_gate_key_invalid", body["error"])
	require.Equal(t, "BACKLOGIT_GATE_EVIDENCE_KEY", body["missing_env_var"])
	require.Equal(t, false, body["retryable"])
	require.EqualValues(t, 32, body["min_key_bytes"])
}

// TestGateErrorResult_FormalReportInvalid verifies a refusal caused by a
// report failing the schema-validated formal report contract maps to its own
// error_type with actionable remediation.
func TestGateErrorResult_FormalReportInvalid(t *testing.T) {
	err := fmt.Errorf("%w: formal report: %w", corerrors.ErrFormalGateRequired, corerrors.ErrFormalReportInvalid)
	result, handled := gateErrorResult(err, "done")
	require.True(t, handled)
	body := resultBody(t, result)
	require.Equal(t, "formal_gate_report_invalid", body["error"])
	require.Equal(t, false, body["retryable"])
}

// TestGateErrorResult_FormalGateGeneric verifies an ErrFormalGateRequired
// without a more specific recognized cause still gets a distinct, handled
// error_type rather than falling through to a generic internal error.
func TestGateErrorResult_FormalGateGeneric(t *testing.T) {
	err := fmt.Errorf("%w: shipment gate broker unavailable", corerrors.ErrFormalGateRequired)
	result, handled := gateErrorResult(err, "")
	require.True(t, handled)
	body := resultBody(t, result)
	require.Equal(t, "formal_gate_required", body["error"])
	require.Equal(t, false, body["retryable"])
}

// TestGateErrorResult_ProofInvalid verifies a formal-admission MAC/manifest
// verification failure maps to a distinct, non-retryable error_type.
func TestGateErrorResult_ProofInvalid(t *testing.T) {
	err := fmt.Errorf("manifest changed: %w", corerrors.ErrProofInvalid)
	result, handled := gateErrorResult(err, "")
	require.True(t, handled)
	body := resultBody(t, result)
	require.Equal(t, "formal_gate_proof_invalid", body["error"])
	require.Equal(t, false, body["retryable"])
}

// TestGateErrorResult_ProofUnverifiable verifies a proof that could not be
// evaluated at all maps to its own distinct error_type from ProofInvalid.
func TestGateErrorResult_ProofUnverifiable(t *testing.T) {
	err := fmt.Errorf("no proof present: %w", corerrors.ErrProofUnverifiable)
	result, handled := gateErrorResult(err, "")
	require.True(t, handled)
	body := resultBody(t, result)
	require.Equal(t, "formal_gate_proof_unverifiable", body["error"])
	require.Equal(t, false, body["retryable"])
}
