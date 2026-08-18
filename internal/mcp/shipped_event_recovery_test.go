package mcp

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	corerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// 143.010-T (Unit 10): producer-scoped recovery guidance for the governed
// shipped-event append. mutationPartialRecovery is shared by three producers
// (domainError, handleTrackCommit, checkpointDispositionError), so repointing
// the class-only "indeterminate" case would mis-advise all three. The guidance
// branches on FailedStep and CompensationState instead.
//
// These scenarios were written and observed failing before the change: the
// guidance named check_partial_mutations for every class.

func decodeMutationPartial(t *testing.T, result *mcplib.CallToolResult) mutationPartialResponse {
	t.Helper()
	require.NotNil(t, result)
	require.True(t, result.IsError)
	text, ok := result.Content[0].(mcplib.TextContent)
	require.True(t, ok)
	var resp mutationPartialResponse
	require.NoError(t, json.Unmarshal([]byte(text.Text), &resp))
	return resp
}

func TestMutationPartialRecovery_ShippedEventIndeterminateNamesBothSurfaces(t *testing.T) {
	err := &corerrors.MutationPartialError{
		Completed:         []string{"complete-release-scope", "persist-shipment-status"},
		FailedStep:        corerrors.StepShippedEventAppend,
		CompensationState: "not-compensated",
		Class:             "indeterminate",
		Cause:             errors.New("injected append failure"),
	}

	resp := decodeMutationPartial(t, domainError("ship shipment", err))

	assert.Equal(t, "indeterminate", resp.Classification)
	assert.Equal(t, corerrors.StepShippedEventAppend, resp.FailedStep)
	assert.False(t, resp.Retryable)
	assert.Contains(t, resp.Recovery, "check_shipped_event_completeness",
		"the MCP audit parameter must be named")
	assert.Contains(t, resp.Recovery, "--check-shipped-event-completeness",
		"the CLI audit flag must be named")
	assert.Contains(t, strings.ToLower(resp.Recovery), "reconcile before archiving",
		"the reconcile-before-archiving instruction must be carried")
	assert.NotContains(t, resp.Recovery, "check_partial_mutations",
		"check_partial_mutations provably cannot detect this residue")
}

func TestMutationPartialRecovery_ShippedEventPartiallyCompensatedIsNotRetryable(t *testing.T) {
	err := &corerrors.MutationPartialError{
		FailedStep:        corerrors.StepShippedEventAppend,
		CompensationState: "partially-compensated",
		Class:             "not-applied",
		Cause:             errors.New("compensation could not restore: 901.001-T"),
	}

	resp := decodeMutationPartial(t, domainError("ship shipment", err))

	assert.Equal(t, "not-applied", resp.Classification)
	assert.Equal(t, "partially-compensated", resp.CompensationState)
	assert.False(t, resp.Retryable,
		"a partially-compensated result must never advertise itself as safe to retry")
	assert.Contains(t, resp.Message, "901.001-T",
		"the un-restored IDs must reach the structured result, not only the prose")
	assert.Contains(t, resp.Recovery, "check_shipped_event_completeness")
}

func TestMutationPartialRecovery_ShippedEventCompensatedIsRetryable(t *testing.T) {
	err := &corerrors.MutationPartialError{
		FailedStep:        corerrors.StepShippedEventAppend,
		CompensationState: "compensated",
		Class:             "not-applied",
		Cause:             errors.New("lock shipment event log"),
	}

	resp := decodeMutationPartial(t, domainError("ship shipment", err))

	assert.True(t, resp.Retryable, "a fully compensated refusal is safe to retry")
	assert.Contains(t, strings.ToLower(resp.Recovery), "retry")
}

// The guidance change must not leak out of the shipped-event producer. The
// existing track_commit assertion in internal/mcp/error_mapping_test.go pins the
// class-only default text for FailedStep "jsonl-append"; this asserts the same
// invariant directly alongside the new branch.
func TestMutationPartialRecovery_OtherProducersKeepClassOnlyGuidance(t *testing.T) {
	err := &corerrors.MutationPartialError{
		FailedStep:        "jsonl-append",
		CompensationState: "unknown",
		Class:             "indeterminate",
		Cause:             errors.New("injected"),
	}

	resp := decodeMutationPartial(t, domainError("track commit", err))

	assert.Contains(t, resp.Recovery, "check_partial_mutations")
	assert.NotContains(t, resp.Recovery, "check_shipped_event_completeness")
}

// gateErrorResult runs before domainError in handleShipShipment but matches only
// gate types, so it cannot consume a *MutationPartialError. This regression
// guards that dispatch ordering rather than restructuring it.
func TestShipShipment_MutationPartialIsNotConsumedByGateErrorResult(t *testing.T) {
	err := &corerrors.MutationPartialError{
		FailedStep:        corerrors.StepShippedEventAppend,
		CompensationState: "not-compensated",
		Class:             "indeterminate",
		Cause:             errors.New("injected"),
	}
	result, matched := gateErrorResult(err, "shipped")
	assert.False(t, matched, "gateErrorResult must not intercept a mutation-partial result")
	assert.Nil(t, result)
}
