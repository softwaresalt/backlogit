package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
)

// TestMCPClaimPredecessorNotShipped verifies backlogit_claim_shipment maps the
// core predecessor refusal to the dedicated shipment_predecessor_not_shipped code.
func TestMCPClaimPredecessorNotShipped(t *testing.T) {
	s, ws := setupShipmentErrorServer(t)
	ctx := context.Background()

	feat, err := core.CreateArtifact(ctx, ws, "Predecessor feature", "feature")
	require.NoError(t, err)
	predMember, err := core.CreateArtifact(ctx, ws, "Predecessor member", "task", core.WithParent(feat.ID))
	require.NoError(t, err)
	depMember, err := core.CreateArtifact(ctx, ws, "Dependent member", "task", core.WithParent(feat.ID))
	require.NoError(t, err)
	pred, err := core.CreateShipment(ctx, ws, "Predecessor shipment", []string{predMember.ID})
	require.NoError(t, err)
	dep, err := core.CreateShipment(ctx, ws, "Dependent shipment", []string{depMember.ID})
	require.NoError(t, err)
	require.NoError(t, core.AddDependency(ctx, ws, dep.ID, pred.ID, "blocks"))

	result, err := s.handleClaimShipment(ctx, shipmentRequest(map[string]any{"id": dep.ID}))
	require.NoError(t, err)
	require.True(t, result.IsError, "claim must be refused")
	text := resultText(t, result)
	var resp struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	require.NoError(t, json.Unmarshal([]byte(text), &resp), "result must be JSON: %s", text)
	assert.Equal(t, "shipment_predecessor_not_shipped", resp.Error)
	assert.Contains(t, resp.Message, pred.ID)
}
