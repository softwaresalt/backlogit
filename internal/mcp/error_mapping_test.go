package mcp

// 026.011-T: Comprehensive error mapping tests for domainError.
//
// These tests verify that every sentinel error defined in internal/errors/ maps
// to the correct MCP wire-type and never falls through to the "internal"
// category unexpectedly. The mapping contract is documented in the
// domainError comment in errors.go.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	mcplib "github.com/mark3labs/mcp-go/mcp"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	corerrors "github.com/softwaresalt/backlogit/internal/errors"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/models"
)

// domainErrorType calls domainError and returns the "error" field from the JSON body.
func domainErrorType(t *testing.T, err error) string {
	t.Helper()
	result := domainError("test op", err)
	require.True(t, result.IsError, "domainError must always produce an error result")
	return shipmentErrorField(t, result)
}

// domainErrorMessage parses the "message" field from an error result.
func domainErrorMessage(t *testing.T, result *mcplib.CallToolResult) string {
	t.Helper()
	require.True(t, result.IsError)
	require.NotEmpty(t, result.Content)
	text, ok := result.Content[0].(mcplib.TextContent)
	require.True(t, ok, "expected text content, got %T", result.Content[0])
	var resp struct {
		Message string `json:"message"`
	}
	require.NoError(t, json.Unmarshal([]byte(text.Text), &resp))
	return resp.Message
}

// TestDomainError_NotFound_MapsCorrectly verifies that all not-found sentinels
// produce error="not_found".
func TestDomainError_NotFound_MapsCorrectly(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "ErrNotFound", err: corerrors.ErrNotFound},
		{name: "ErrShipmentNotFound", err: corerrors.ErrShipmentNotFound},
		{name: "wrapped ErrNotFound", err: fmt.Errorf("context: %w", corerrors.ErrNotFound)},
		{name: "wrapped ErrShipmentNotFound", err: fmt.Errorf("context: %w", corerrors.ErrShipmentNotFound)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, "not_found", domainErrorType(t, tt.err),
				"%s must map to not_found", tt.name)
		})
	}
}

// TestDomainError_Conflict_MapsCorrectly verifies that all conflict sentinels
// produce error="conflict".
func TestDomainError_Conflict_MapsCorrectly(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "ErrShipmentConflict", err: corerrors.ErrShipmentConflict},
		{name: "ErrItemAlreadyAssigned", err: corerrors.ErrItemAlreadyAssigned},
		{name: "ErrCannotReturnItem", err: corerrors.ErrCannotReturnItem},
		{name: "ErrChildrenNotTerminal", err: corerrors.ErrChildrenNotTerminal},
		{name: "ErrTaskBusy", err: core.ErrTaskBusy},
		{name: "wrapped ErrTaskBusy", err: fmt.Errorf("set artifact size: %w", core.ErrTaskBusy)},
		{name: "wrapped ErrShipmentConflict", err: fmt.Errorf("wrap: %w", corerrors.ErrShipmentConflict)},
		{name: "wrapped ErrChildrenNotTerminal", err: fmt.Errorf("wrap: %w", corerrors.ErrChildrenNotTerminal)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, "conflict", domainErrorType(t, tt.err),
				"%s must map to conflict", tt.name)
		})
	}
}

func TestDomainError_BlockedShipmentEnvelope_MapsCorrectly(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "direct", err: corerrors.ErrShipmentBlockedRequiresEnvelope},
		{name: "wrapped", err: fmt.Errorf("wrap: %w", corerrors.ErrShipmentBlockedRequiresEnvelope)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, "shipment_blocked_requires_envelope", domainErrorType(t, test.err))
		})
	}
}

// TestDomainError_WriteDurability_MapsCorrectly verifies that the generic
// durable-write sentinels (surfaced by callers that do not wrap a durable
// write in a MutationEnvelope, e.g. ResolveCheckpoint) map to their own
// codes rather than falling through to "internal" (147-F, found during
// 130-S adversarial review: the fix restoring RewriteCheckpointFile's fsync
// guarantee reintroduced the possibility of these sentinels reaching
// domainError directly).
func TestDomainError_WriteDurability_MapsCorrectly(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "ErrWriteIndeterminate", err: corerrors.ErrWriteIndeterminate, want: "write_indeterminate"},
		{name: "wrapped ErrWriteIndeterminate", err: fmt.Errorf("wrap: %w", corerrors.ErrWriteIndeterminate), want: "write_indeterminate"},
		{name: "ErrWriteNotApplied", err: corerrors.ErrWriteNotApplied, want: "write_not_applied"},
		{name: "wrapped ErrWriteNotApplied", err: fmt.Errorf("wrap: %w", corerrors.ErrWriteNotApplied), want: "write_not_applied"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, domainErrorType(t, tt.err),
				"%s must map to %s, not fall through to internal", tt.name, tt.want)
		})
	}
}

// TestDomainError_Validation_MapsCorrectly verifies that all validation sentinels
// produce error="validation_failed".
func TestDomainError_Validation_MapsCorrectly(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "ErrValidation", err: corerrors.ErrValidation},
		{name: "ErrInvalidLinkType", err: corerrors.ErrInvalidLinkType},
		{name: "ErrTelemetrySourceMissing", err: corerrors.ErrTelemetrySourceMissing},
		{name: "wrapped ErrValidation", err: fmt.Errorf("wrap: %w", corerrors.ErrValidation)},
		{name: "wrapped ErrTelemetrySourceMissing", err: fmt.Errorf("wrap: %w", corerrors.ErrTelemetrySourceMissing)},
		{name: "ValidationError struct", err: corerrors.NewValidationError("field", "val", "too short", nil)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, "validation_failed", domainErrorType(t, tt.err),
				"%s must map to validation_failed", tt.name)
		})
	}
}

// TestDomainError_Unknown_MapsToInternal verifies that unrecognised errors
// produce error="internal" and include the op prefix in the message.
func TestDomainError_Unknown_MapsToInternal(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "plain error", err: errors.New("something broke")},
		{name: "ErrTelemetryParseFailed", err: corerrors.ErrTelemetryParseFailed},
		{name: "ErrQuery", err: corerrors.ErrQuery},
		{name: "ErrConfig", err: corerrors.ErrConfig},
		{name: "ErrRehydration", err: corerrors.ErrRehydration},
		{name: "ErrMigration", err: corerrors.ErrMigration},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, "internal", domainErrorType(t, tt.err),
				"%s must fall through to internal", tt.name)
		})
	}
}

// TestDomainError_StateDumpGuards_MapsToValidation verifies that the new
// 153.003-T (S1 U3) state_dump sentinel errors map to validation_failed
// (not internal_error) so MCP callers distinguish client-input rejections
// from server-side faults (Copilot PRRT_kwDORzozKM6fDzZU).
func TestDomainError_StateDumpGuards_MapsToValidation(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "ErrCheckpointStateDumpTooLarge", err: corerrors.ErrCheckpointStateDumpTooLarge},
		{name: "wrapped ErrCheckpointStateDumpTooLarge", err: fmt.Errorf("wrap: %w", corerrors.ErrCheckpointStateDumpTooLarge)},
		{name: "ErrCheckpointStateDumpSecretDetected", err: corerrors.ErrCheckpointStateDumpSecretDetected},
		{name: "wrapped ErrCheckpointStateDumpSecretDetected", err: fmt.Errorf("wrap: %w", corerrors.ErrCheckpointStateDumpSecretDetected)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, "validation_failed", domainErrorType(t, tt.err),
				"%s must map to validation_failed, not fall through to internal", tt.name)
		})
	}
}

// TestDomainError_CheckpointTargetUnsafe_MapsCorrectly verifies that
// ErrCheckpointTargetUnsafe maps to checkpoint_target_unsafe in domainError
// (153.001-T / S1 U1 — Copilot PRRT_kwDORzozKM6fDzZU).
func TestDomainError_CheckpointTargetUnsafe_MapsCorrectly(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "ErrCheckpointTargetUnsafe", err: corerrors.ErrCheckpointTargetUnsafe},
		{name: "wrapped ErrCheckpointTargetUnsafe", err: fmt.Errorf("path check failed: %w", corerrors.ErrCheckpointTargetUnsafe)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, "checkpoint_target_unsafe", domainErrorType(t, tt.err),
				"%s must map to checkpoint_target_unsafe, not fall through to internal", tt.name)
		})
	}
}

// TestDomainError_MessageContainsOpOnInternal verifies the op prefix is present
// in the message field for InternalError results.
func TestDomainError_MessageContainsOpOnInternal(t *testing.T) {
	opErr := errors.New("db exploded")
	result := domainError("archive item", opErr)
	require.True(t, result.IsError)
	msg := domainErrorMessage(t, result)
	assert.Contains(t, msg, "archive item",
		"internal error message must contain the op prefix")
}

func TestDomainError_MutationPartial_MapsStructuredPayload(t *testing.T) {
	tests := []struct {
		name         string
		class        string
		retryable    bool
		recoveryHint string
	}{
		{
			name:         "not applied",
			class:        "not-applied",
			retryable:    true,
			recoveryHint: "safe to retry",
		},
		{
			name:         "indeterminate",
			class:        "indeterminate",
			retryable:    false,
			recoveryHint: "do not blindly retry",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := domainError("track commit", fmt.Errorf("wrap: %w", &corerrors.MutationPartialError{
				Completed:         []string{"frontmatter-scalar"},
				FailedStep:        "jsonl-append",
				CompensationState: "compensated",
				Class:             tt.class,
				Cause:             corerrors.ErrWriteNotApplied,
			}))

			require.True(t, result.IsError)
			require.NotEmpty(t, result.Content)
			text, ok := result.Content[0].(mcplib.TextContent)
			require.True(t, ok)

			var resp struct {
				Error             string   `json:"error"`
				Message           string   `json:"message"`
				Classification    string   `json:"classification"`
				CompletedSteps    []string `json:"completed_steps"`
				FailedStep        string   `json:"failed_step"`
				CompensationState string   `json:"compensation_state"`
				Retryable         bool     `json:"retryable"`
				Recovery          string   `json:"recovery"`
			}
			require.NoError(t, json.Unmarshal([]byte(text.Text), &resp))
			assert.Equal(t, "mutation_partial", resp.Error)
			assert.Equal(t, tt.class, resp.Classification)
			assert.Equal(t, []string{"frontmatter-scalar"}, resp.CompletedSteps)
			assert.Equal(t, "jsonl-append", resp.FailedStep)
			assert.Equal(t, "compensated", resp.CompensationState)
			assert.Equal(t, tt.retryable, resp.Retryable)
			assert.Contains(t, resp.Recovery, "check_partial_mutations")
			assert.Contains(t, resp.Recovery, tt.recoveryHint)
			assert.Contains(t, resp.Message, "track commit")
		})
	}
}

// TestDomainError_MutationPartial_QuarantineRequired is a regression test
// (found during 130-S adversarial review): a between-read race can surface a
// malformed/non-conforming checkpoint verdict through the mutation_partial
// shape (e.g. AbandonCheckpoint's envelope-wrapped rewrite step) rather than
// through the dedicated checkpointDispositionError shape. QuarantineIsRemedy
// traverses through MutationPartialError.Unwrap() to the Cause, so
// mutationPartialError must surface quarantine_required: true and append
// quarantine guidance to Recovery, without discarding the existing
// classification/completed-steps/compensation-state context.
func TestDomainError_MutationPartial_QuarantineRequired(t *testing.T) {
	tests := []struct {
		name  string
		cause error
	}{
		{name: "wrapped ErrCheckpointUseQuarantine", cause: corerrors.ErrCheckpointUseQuarantine},
		{name: "normalized ErrCheckpointCorrupt", cause: fmt.Errorf("%w: %w", corerrors.ErrCheckpointUseQuarantine, corerrors.ErrCheckpointCorrupt)},
		{name: "ErrCheckpointNonConforming", cause: corerrors.ErrCheckpointNonConforming},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := domainError("abandon checkpoint", fmt.Errorf("wrap: %w", &corerrors.MutationPartialError{
				Completed:         []string{},
				FailedStep:        "rewrite-checkpoint",
				CompensationState: "compensated",
				Class:             "not-applied",
				Cause:             tt.cause,
			}))

			require.True(t, result.IsError)
			require.NotEmpty(t, result.Content)
			text, ok := result.Content[0].(mcplib.TextContent)
			require.True(t, ok)

			var resp struct {
				Error              string `json:"error"`
				Classification     string `json:"classification"`
				Recovery           string `json:"recovery"`
				Retryable          bool   `json:"retryable"`
				QuarantineRequired bool   `json:"quarantine_required"`
			}
			require.NoError(t, json.Unmarshal([]byte(text.Text), &resp))
			assert.Equal(t, "mutation_partial", resp.Error, "the mutation-partial context must be preserved")
			assert.Equal(t, "not-applied", resp.Classification)
			assert.True(t, resp.QuarantineRequired)
			assert.Contains(t, resp.Recovery, "quarantine")
			assert.False(t, resp.Retryable,
				"retryable must be false when quarantine is required, even though class=not-applied and "+
					"compensation_state=compensated would otherwise mark it retryable — retrying the same "+
					"resolve/abandon call cannot succeed against quarantine-only bytes")
		})
	}
}

// TestDomainError_MutationPartial_QuarantineNotRequired asserts an ordinary
// partial-mutation cause (not quarantine-related) omits quarantine_required
// from the marshaled JSON (via omitempty) rather than emitting false.
func TestDomainError_MutationPartial_QuarantineNotRequired(t *testing.T) {
	result := domainError("track commit", fmt.Errorf("wrap: %w", &corerrors.MutationPartialError{
		FailedStep:        "jsonl-append",
		CompensationState: "compensated",
		Class:             "not-applied",
		Cause:             corerrors.ErrWriteNotApplied,
	}))
	require.True(t, result.IsError)
	text, ok := result.Content[0].(mcplib.TextContent)
	require.True(t, ok)
	assert.NotContains(t, text.Text, "quarantine_required",
		"quarantine_required must be omitted (omitempty) when the cause is unrelated to quarantine")
}

// TestUCYA1_WriteIndeterminateWinsOverSiblingSentinels pins the precedence rule
// that an indeterminate write outranks not-found, conflict, and validation
// sentinels, because callers must not retry an operation whose outcome is unknown.
func TestUCYA1_WriteIndeterminateWinsOverSiblingSentinels(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{
			name: "ErrWriteIndeterminate with ErrShipmentConflict",
			err:  fmt.Errorf("%w: %w", corerrors.ErrWriteIndeterminate, corerrors.ErrShipmentConflict),
		},
		{
			name: "ErrWriteIndeterminate with ErrNotFound",
			err:  fmt.Errorf("%w: %w", corerrors.ErrWriteIndeterminate, corerrors.ErrNotFound),
		},
		{
			name: "ErrWriteIndeterminate with ErrValidation",
			err:  fmt.Errorf("%w: %w", corerrors.ErrWriteIndeterminate, corerrors.ErrValidation),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, "write_indeterminate", domainErrorType(t, tt.err),
				"%s must map to write_indeterminate ahead of its sibling sentinel", tt.name)
		})
	}
}

// TestUCYA1_ShipmentLifecycleStepErrorsKeepSentinelChain drives the real U20C3
// compensation-event failure and the U20C5 commit-journal failure through core
// and through the backlogit_block_shipment handler. The core error must keep the
// failed step's sentinel wrapped with %w, and the wire code must stay
// write_indeterminate.
func TestUCYA1_ShipmentLifecycleStepErrorsKeepSentinelChain(t *testing.T) {
	tests := []struct {
		name         string
		arm          func(*testing.T, string, string) func()
		stepLabel    string
		wantSentinel error
		sentinelName string
	}{
		{
			name:         "U20C3 compensation event append",
			arm:          armUCYA1LogDirectory,
			stepLabel:    "append compensation status evidence",
			wantSentinel: corerrors.ErrWriteNotApplied,
			sentinelName: "ErrWriteNotApplied",
		},
		{
			name:         "U20C5 commit journal",
			arm:          armUCYA1OpsRegularFile,
			stepLabel:    "persist block shipment commit",
			wantSentinel: corerrors.ErrValidation,
			sentinelName: "ErrValidation",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, shipmentID := newUCYA1ActiveShipmentRoot(t)
			err := blockUCYA1ThroughCore(t, root, shipmentID, tt.arm)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.stepLabel, "the fault must reach the intended lifecycle step")
			assert.True(t, errors.Is(err, corerrors.ErrWriteIndeterminate))
			assert.True(t, errors.Is(err, tt.wantSentinel),
				"the %s step error must wrap %s, got: %v", tt.stepLabel, tt.sentinelName, err)
			assert.False(t, errors.Is(err, corerrors.ErrShipmentConflict),
				"the pre-compensation cause must stay isolated behind %%v")
			assert.False(t, errors.Is(err, corerrors.ErrNotFound),
				"the pre-compensation cause must stay isolated behind %%v")

			root, shipmentID = newUCYA1ActiveShipmentRoot(t)
			assert.Equal(t, "write_indeterminate", blockUCYA1ThroughHandler(t, root, shipmentID, tt.arm))
		})
	}
}

// newUCYA1ActiveShipmentRoot creates a workspace holding one active shipment with
// one member, closes it, and returns the root and shipment ID so the caller can
// reopen the workspace through core or the MCP server.
func newUCYA1ActiveShipmentRoot(t *testing.T) (string, string) {
	t.Helper()

	ctx := context.Background()
	root := t.TempDir()
	storageRoot := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(storageRoot, 0o755))
	require.NoError(t, config.WriteDefaults(storageRoot))

	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)
	feature, err := core.CreateArtifact(ctx, ws, "UCYA1 feature", "feature")
	require.NoError(t, err)
	member, err := core.CreateArtifact(
		ctx,
		ws,
		"UCYA1 member",
		"task",
		core.WithParent(feature.ID),
		core.WithStatus(string(models.StatusActive)),
	)
	require.NoError(t, err)
	shipment, err := core.CreateShipment(ctx, ws, "UCYA1 shipment", []string{member.ID})
	require.NoError(t, err)
	_, err = core.ClaimShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)
	require.NoError(t, ws.Close())

	return root, shipment.ID
}

// armUCYA1LogDirectory replaces the shipment's item log with a directory when the
// first append for the shipment begins. The lifecycle intent append and the
// compensation evidence append both fail before any byte is written.
func armUCYA1LogDirectory(t *testing.T, root, shipmentID string) func() {
	t.Helper()

	logPath := events.LogPathForItem(core.WorkspaceLogsRoot(root), shipmentID)
	armed := false
	restore := events.SetItemLogLockBarrierHookForTest(func(itemID string) {
		if armed || itemID != shipmentID {
			return
		}
		armed = true
		require.NoError(t, os.RemoveAll(logPath))
		require.NoError(t, os.Mkdir(logPath, 0o755))
	})
	t.Cleanup(restore)
	return restore
}

// armUCYA1OpsRegularFile replaces the shipment operations directory with a regular
// file just before the third lifecycle append for the shipment (the committed
// evidence). The commit journal write then fails after committed evidence is
// durable, which is the U20C5 failure shape.
func armUCYA1OpsRegularFile(t *testing.T, root, shipmentID string) func() {
	t.Helper()

	opsRoot := filepath.Join(root, ".backlogit", "ops")
	appends := 0
	restore := events.SetItemLogLockBarrierHookForTest(func(itemID string) {
		if itemID != shipmentID {
			return
		}
		appends++
		if appends != 3 {
			return
		}
		require.NoError(t, os.RemoveAll(opsRoot))
		require.NoError(t, os.WriteFile(opsRoot, []byte("ucya1 obstruction"), 0o644))
	})
	t.Cleanup(restore)
	return restore
}

// blockUCYA1ThroughCore runs core.BlockShipment on a reopened workspace with the
// fault armed and returns the real error value.
func blockUCYA1ThroughCore(t *testing.T, root, shipmentID string, arm func(*testing.T, string, string) func()) error {
	t.Helper()

	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)
	defer func() { require.NoError(t, ws.Close()) }()
	restore := arm(t, root, shipmentID)
	defer restore()

	_, blockErr := core.BlockShipment(ctx, ws, shipmentID, core.BlockOptions{
		Reason:    "ucya1 lifecycle fault",
		BlockedBy: "UCYA1",
	})
	return blockErr
}

// blockUCYA1ThroughHandler drives backlogit_block_shipment through the MCP server
// with the fault armed and returns the wire error code.
func blockUCYA1ThroughHandler(t *testing.T, root, shipmentID string, arm func(*testing.T, string, string) func()) string {
	t.Helper()

	server := NewServerForRoot(root)
	t.Cleanup(func() {
		if server.Workspace != nil {
			assert.NoError(t, server.Workspace.Close())
		}
	})
	restore := arm(t, root, shipmentID)
	defer restore()

	request := mcplib.CallToolRequest{}
	request.Params.Arguments = map[string]any{
		"id":     shipmentID,
		"reason": "ucya1 lifecycle fault",
		"by":     "UCYA1",
	}
	result, err := server.InvokeTool(context.Background(), "backlogit_block_shipment", request)
	require.NoError(t, err)
	require.True(t, result.IsError, "the injected lifecycle fault must surface as a tool error")
	return shipmentErrorField(t, result)
}

// schemaErrorBody decodes the structured body of a checkpoint-schema error
// result so tests assert on the serialized CallToolResult, not a Go error.
func schemaErrorBody(t *testing.T, result *mcplib.CallToolResult) map[string]any {
	t.Helper()
	require.NotNil(t, result)
	require.True(t, result.IsError)
	require.NotEmpty(t, result.Content)
	text, ok := result.Content[0].(mcplib.TextContent)
	require.True(t, ok, "expected text content, got %T", result.Content[0])
	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte(text.Text), &body))
	return body
}

// TestDomainError_CheckpointSchema_SentinelsAreStructurallyDistinct (171.004-T)
// asserts the two schema sentinels are validation-class and distinguishable by
// machine-readable fields (not message wording), with the opt-in remediation
// present only for the legacy-eligible sentinel and spelled as the MCP
// parameter, not the CLI flag.
func TestDomainError_CheckpointSchema_SentinelsAreStructurallyDistinct(t *testing.T) {
	rejected := schemaErrorBody(t, domainError("create checkpoint",
		fmt.Errorf("create: %w: schema_version is missing or 0", corerrors.ErrCheckpointSchemaRejected)))
	unsupported := schemaErrorBody(t, domainError("create checkpoint",
		fmt.Errorf("create: %w: %w", corerrors.ErrCheckpointSchemaUnsupported, corerrors.ErrCheckpointUnknownField)))

	assert.Equal(t, "validation_failed", rejected["error"])
	assert.Equal(t, "validation_failed", unsupported["error"],
		"unsupported wraps the unknown-field cause but must not be shadowed by that mapping")

	assert.Equal(t, "checkpoint_schema_rejected", rejected["code"])
	assert.Equal(t, "checkpoint_schema_unsupported", unsupported["code"])
	assert.NotEqual(t, rejected["code"], unsupported["code"])

	assert.Equal(t, true, rejected["retry_with_opt_in"])
	assert.Equal(t, false, unsupported["retry_with_opt_in"])

	assert.Equal(t, "allow_legacy_import", rejected["remediation"])
	assert.NotContains(t, rejected["message"], "--allow-legacy-import", "MCP must not use the CLI flag spelling")
	assert.NotContains(t, unsupported, "remediation", "no opt-in remediation when enabling it cannot help")
	assert.NotContains(t, unsupported["message"], "allow_legacy_import")
	assert.NotContains(t, unsupported["message"], "allow-legacy-import")

	assert.NotContains(t, unsupported, "unknown_fields", "unsupported must not adopt the unknown-fields shape")
}

// TestHandleCreateCheckpoint_SchemaSentinels_SerializedResult drives the real
// handler (which returns a nil Go error) and asserts the structured distinction
// on the serialized result.
func TestHandleCreateCheckpoint_SchemaSentinels_SerializedResult(t *testing.T) {
	s, _ := setupBugFixServer(t)
	tests := []struct {
		name          string
		dump          string
		wantCode      string
		wantRetryable bool
	}{
		{"in-window legacy", `{"agent":"ship","session_id":"s1","phase":"build"}`, "checkpoint_schema_rejected", true},
		{"out-of-window legacy", `{"consumer":"ship","phase":"build"}`, "checkpoint_schema_unsupported", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			request := mcplib.CallToolRequest{}
			request.Params.Name = "backlogit_create_checkpoint"
			request.Params.Arguments = map[string]any{"state_dump": tc.dump}
			result, err := s.handleCreateCheckpoint(context.Background(), request)
			require.NoError(t, err, "handler converts domain errors into results")
			body := schemaErrorBody(t, result)
			assert.Equal(t, "validation_failed", body["error"])
			assert.Equal(t, tc.wantCode, body["code"])
			assert.Equal(t, tc.wantRetryable, body["retry_with_opt_in"])
		})
	}
}

// 171.004-T: the serialized schema error must stay bounded even when the
// wrapped cause echoes caller-controlled input that fits the 64-KiB input cap.
func TestHandleCreateCheckpoint_SchemaError_DiagnosticIsBounded(t *testing.T) {
	s, _ := setupBugFixServer(t)
	const maxSerialized = 2048
	tests := []struct {
		name string
		dump string
	}{
		{"invalid created_at 60000 chars",
			`{"agent":"ship","session_id":"s1","phase":"build","created_at":"` + strings.Repeat("x", 60000) + `"}`},
		{"duplicated 30000 byte context key",
			`{"agent":"ship","session_id":"s1","phase":"build","context":{"` +
				strings.Repeat("k", 30000) + `":1,"` + strings.Repeat("k", 30000) + `":2}}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			request := mcplib.CallToolRequest{}
			request.Params.Name = "backlogit_create_checkpoint"
			request.Params.Arguments = map[string]any{"state_dump": tc.dump}
			result, err := s.handleCreateCheckpoint(context.Background(), request)
			require.NoError(t, err)
			body := schemaErrorBody(t, result)
			raw, mErr := json.Marshal(result)
			require.NoError(t, mErr)
			assert.Less(t, len(raw), maxSerialized, "serialized CallToolResult must be bounded")
			assert.Equal(t, "validation_failed", body["error"])
			assert.Equal(t, "checkpoint_schema_unsupported", body["code"])
			assert.Equal(t, false, body["retry_with_opt_in"])
			assert.Contains(t, body["message"], "[truncated]")
			assert.True(t, utf8.ValidString(body["message"].(string)))
		})
	}
}

func TestCheckpointSchemaError_BoundedPreservesOptInFieldsAndRuneBoundary(t *testing.T) {
	cause := fmt.Errorf("%w: %s", corerrors.ErrCheckpointSchemaRejected, strings.Repeat("é", 5000))
	body := schemaErrorBody(t, domainError("create checkpoint", cause))
	msg, _ := body["message"].(string)
	assert.True(t, utf8.ValidString(msg), "truncation must not split a rune")
	assert.Contains(t, msg, "[truncated]")
	assert.Contains(t, msg, "allow_legacy_import", "opt-in advice must survive truncation")
	assert.Less(t, len(msg), 1024)
	assert.Equal(t, "checkpoint_schema_rejected", body["code"])
	assert.Equal(t, true, body["retry_with_opt_in"])
	assert.Equal(t, "allow_legacy_import", body["remediation"])
}
