package cli

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRegistryParity_NormalizeBlockedShipmentMapping(t *testing.T) {
	operations := loadRegistryOperations(t)
	operation, found := operations["normalize_blocked_shipment"]
	require.True(t, found, "normalizer MCP operation must be present in the backlog registry")
	require.Equal(t, "backlogit_normalize_blocked_shipment", operation.MCPTool)
	require.Empty(t, operation.CLICommand, "normalizer parity does not require a CLI command")
	require.Equal(t, map[string]string{
		"shipment_id":  "id",
		"snapshot_ref": "snapshot_ref",
		"by":           "by",
	}, operation.Params)
}
