package mcp

// 026.011-T: Comprehensive error mapping tests for domainError.
//
// These tests verify that every sentinel error defined in internal/errors/ maps
// to the correct MCP wire-type and never falls through to the "internal"
// category unexpectedly. The mapping contract is documented in the
// domainError comment in errors.go.

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	corerrors "github.com/softwaresalt/backlogit/internal/errors"

	"github.com/softwaresalt/backlogit/internal/core"
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
