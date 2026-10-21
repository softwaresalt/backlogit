package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
	corerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// The dispose tool is Stage-only (G-B). These tests call the handler in
// process on temp-dir fixtures and never touch a real workspace.

func setupDisposeMCPFixture(t *testing.T) (*Server, string, string) {
	t.Helper()
	ctx := context.Background()
	s, ws := setupBugFixServer(t)
	feature, err := core.CreateArtifact(ctx, ws, "Dispose feature", "feature")
	require.NoError(t, err)
	member, err := core.CreateArtifact(ctx, ws, "Dispose member", "task", core.WithParent(feature.ID))
	require.NoError(t, err)
	shipment, err := core.CreateShipment(ctx, ws, "Dispose target", []string{member.ID})
	require.NoError(t, err)
	delib, err := core.CreateArtifact(ctx, ws, "Disposition authorization", "deliberation",
		core.WithStatus("done"),
		core.WithFields(map[string]any{"authorizes_disposition": shipment.ID}))
	require.NoError(t, err)
	return s, shipment.ID, delib.ID
}

func disposeArgs(id, delibID string, confirm bool) map[string]any {
	return map[string]any{
		"id":                id,
		"reason":            "superseded by a newer plan",
		"by":                "operator",
		"authorization_ref": delibID,
		"confirm":           confirm,
	}
}

func TestShipmentDispose_MCPSuccessReturnsAbandonedShipment(t *testing.T) {
	s, shipID, delibID := setupDisposeMCPFixture(t)

	result, err := s.handleDisposeShipment(context.Background(), contractRequest(disposeArgs(shipID, delibID, true)))
	require.NoError(t, err)
	text := resultTextForHarness(t, result)
	require.False(t, result.IsError, text)

	var m map[string]any
	require.NoError(t, json.Unmarshal([]byte(text), &m), text)
	assert.Equal(t, shipID, m["id"])
	assert.Equal(t, "abandoned", m["status"])
}

func TestShipmentDispose_MCPErrorCodes(t *testing.T) {
	s, shipID, delibID := setupDisposeMCPFixture(t)

	tests := []struct {
		name string
		args map[string]any
		want string
	}{
		{"confirm false", disposeArgs(shipID, delibID, false), "confirmation_required"},
		{"empty reason", func() map[string]any {
			a := disposeArgs(shipID, delibID, true)
			a["reason"] = ""
			return a
		}(), "validation_failed"},
		{"unbound authorization", disposeArgs(shipID, "999-DL", true), "validation_failed"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := s.handleDisposeShipment(context.Background(), contractRequest(tc.args))
			require.NoError(t, err)
			assert.Equal(t, tc.want, contractErrorType(t, result))
		})
	}
}

func TestShipmentDispose_MCPStateRefusal(t *testing.T) {
	s, shipID, delibID := setupDisposeMCPFixture(t)
	ctx := context.Background()

	first, err := s.handleDisposeShipment(ctx, contractRequest(disposeArgs(shipID, delibID, true)))
	require.NoError(t, err)
	require.False(t, first.IsError, resultTextForHarness(t, first))

	second, err := s.handleDisposeShipment(ctx, contractRequest(disposeArgs(shipID, delibID, true)))
	require.NoError(t, err)
	assert.Equal(t, "shipment_disposition_refused", contractErrorType(t, second))
}

// TestShipmentDispose_MCPWriteIndeterminate checks the G-D classification: a
// disposition error wrapping ErrWriteIndeterminate surfaces as
// write_indeterminate, never as a refusal or validation error.
func TestShipmentDispose_MCPWriteIndeterminate(t *testing.T) {
	err := fmt.Errorf("dispose shipment 001-S: %w", corerrors.ErrWriteIndeterminate)
	assert.Equal(t, "write_indeterminate", contractErrorType(t, domainError("dispose shipment", err)))
}
