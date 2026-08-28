package mcp

// 134.001-T: Failing tests for MCP create_shipment priority param (U1 red harness).
//
// These tests verify that:
//  1. The MCP backlogit_create_shipment tool accepts a "priority" param and
//     passes it through to core.CreateShipment.
//  2. The tool definition includes "priority" in its InputSchema properties.
//
// Red harness: both tests fail until U2 wires the priority param on the MCP
// tool definition and handler.

import (
	"context"
	"encoding/json"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCreateShipmentMCP_AcceptsPriorityParam verifies that the MCP handler
// backlogit_create_shipment accepts a "priority" argument and creates a
// shipment with that priority reflected in the response.
//
// Red harness: fails until U2 adds the "priority" parameter to the
// backlogit_create_shipment tool and wires it through handleCreateShipment
// → core.CreateShipment(... WithPriority(priority)).
func TestCreateShipmentMCP_AcceptsPriorityParam(t *testing.T) {
	s, _ := setupBugFixServer(t)
	ctx := context.Background()

	result, err := s.handleCreateShipment(ctx, contractRequest(map[string]any{
		"title":    "MCP priority shipment",
		"priority": "medium",
	}))
	require.NoError(t, err)
	require.False(t, result.IsError, "create_shipment with priority must succeed: %s",
		resultTextForHarness(t, result))

	text := resultTextForHarness(t, result)
	var shipment map[string]any
	require.NoError(t, json.Unmarshal([]byte(text), &shipment),
		"create_shipment response must be valid JSON")
	assert.Equal(t, "medium", shipment["priority"],
		"MCP create_shipment priority param must be reflected in the created shipment")
}

// TestCreateShipmentMCP_PriorityParamRegistered verifies that the
// backlogit_create_shipment tool definition includes a "priority" parameter
// in its InputSchema. This guards against a handler that accepts the param
// without advertising it in the tool schema.
//
// Red harness: fails until U2 adds mcplib.WithString("priority", ...) to
// the backlogit_create_shipment tool registration in RegisterTools.
func TestCreateShipmentMCP_PriorityParamRegistered(t *testing.T) {
	s, _ := setupBugFixServer(t)

	var toolDef *mcplib.Tool
	for _, def := range s.ToolDefs() {
		def := def
		if def.Name == "backlogit_create_shipment" {
			toolDef = &def
			break
		}
	}
	require.NotNil(t, toolDef,
		"backlogit_create_shipment tool must be registered with the MCP server")

	_, hasPriority := toolDef.InputSchema.Properties["priority"]
	assert.True(t, hasPriority,
		"backlogit_create_shipment InputSchema must advertise a 'priority' parameter")
}

// TestU7b_ReadSurfaceDescriptionsCarryConformanceGuidance is a table-driven
// assertion over the two registered read-surface tool descriptions, read
// from the built tool set (147-F / U7b). backlogit_list_checkpoints must
// state the needs_quarantine filter-bypass and remediation_intent
// structured-record contract (depends on U6d, U6e). backlogit_get_checkpoint
// must state the conforming-false / schema-invalid-refused contract
// (depends on U6b).
func TestU7b_ReadSurfaceDescriptionsCarryConformanceGuidance(t *testing.T) {
	s, _ := setupBugFixServer(t)
	defs := map[string]string{}
	for _, def := range s.ToolDefs() {
		defs[def.Name] = def.Description
	}

	cases := []struct {
		tool      string
		fragments []string
	}{
		{
			tool: "backlogit_list_checkpoints",
			fragments: []string{
				"needs_quarantine true is not safely rewritable",
				"backlogit_quarantine_checkpoint",
				"regardless of the status, agent, shipment_id, feature_id, and max_age filters",
				"remediation_intent is a structured record",
			},
		},
		{
			tool: "backlogit_get_checkpoint",
			fragments: []string{
				"conforming false when it carries unmodeled top-level keys",
				"cannot be resolved or abandoned",
				"non_conforming_fields carries raw offender paths",
				"schema-invalid document is refused before any conformance verdict",
			},
		},
	}

	for _, tc := range cases {
		desc, ok := defs[tc.tool]
		require.True(t, ok, "%s must be registered", tc.tool)
		for _, fragment := range tc.fragments {
			assert.Contains(t, desc, fragment, "%s description missing expected fragment", tc.tool)
		}
	}
}

