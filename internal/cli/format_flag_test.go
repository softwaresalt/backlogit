package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFormatFlag_ListDefaultsToTable asserts that `backlogit list` has a
// --format flag defaulting to "table" (human-readable output).
func TestFormatFlag_ListDefaultsToTable(t *testing.T) {
	root := NewRootCommand()
	listCmd, _, err := root.Find([]string{"list"})
	require.NoError(t, err)
	require.NotNil(t, listCmd, "list command must be registered")

	flag := listCmd.Flags().Lookup("format")
	if flag == nil {
		flag = listCmd.InheritedFlags().Lookup("format")
	}
	require.NotNil(t, flag, "list command must have a --format flag (local or persistent)")
	assert.Equal(t, "table", flag.DefValue,
		"--format on 'list' must default to 'table' (human-readable default)")
}

// TestFormatFlag_GetDefaultsToTable asserts that `backlogit get` --format defaults to table.
func TestFormatFlag_GetDefaultsToTable(t *testing.T) {
	root := NewRootCommand()
	getCmd, _, err := root.Find([]string{"get"})
	require.NoError(t, err)
	require.NotNil(t, getCmd, "get command must be registered")

	flag := getCmd.Flags().Lookup("format")
	if flag == nil {
		flag = getCmd.InheritedFlags().Lookup("format")
	}
	require.NotNil(t, flag, "get command must have a --format flag")
	assert.Equal(t, "table", flag.DefValue,
		"--format on 'get' must default to 'table'")
}

// TestFormatFlag_QueueViewDefaultsToTable asserts --format on `queue view` defaults to table.
func TestFormatFlag_QueueViewDefaultsToTable(t *testing.T) {
	root := NewRootCommand()
	queueViewCmd, _, err := root.Find([]string{"queue", "view"})
	require.NoError(t, err)
	require.NotNil(t, queueViewCmd, "queue view command must be registered")

	flag := queueViewCmd.Flags().Lookup("format")
	if flag == nil {
		flag = queueViewCmd.InheritedFlags().Lookup("format")
	}
	require.NotNil(t, flag, "queue view command must have a --format flag")
	assert.Equal(t, "table", flag.DefValue,
		"--format on 'queue view' must default to 'table'")
}

// TestFormatFlag_StashListDefaultsToJSON asserts --format on `stash list` defaults to json
// to preserve agent pipeline contracts (PR-001 Option A).
func TestFormatFlag_StashListDefaultsToJSON(t *testing.T) {
	root := NewRootCommand()
	stashListCmd, _, err := root.Find([]string{"stash", "list"})
	require.NoError(t, err)
	require.NotNil(t, stashListCmd, "stash list command must be registered")

	flag := stashListCmd.Flags().Lookup("format")
	if flag == nil {
		flag = stashListCmd.InheritedFlags().Lookup("format")
	}
	require.NotNil(t, flag, "stash list must have a --format flag")
	assert.Equal(t, "json", flag.DefValue,
		"--format on 'stash list' must default to 'json' (PR-001 Option A: preserve agent contracts)")
}

// TestFormatFlag_ShipmentListDefaultsToJSON asserts --format on `shipment list` defaults to json.
func TestFormatFlag_ShipmentListDefaultsToJSON(t *testing.T) {
	root := NewRootCommand()
	shipmentListCmd, _, err := root.Find([]string{"shipment", "list"})
	require.NoError(t, err)
	require.NotNil(t, shipmentListCmd, "shipment list command must be registered")

	flag := shipmentListCmd.Flags().Lookup("format")
	if flag == nil {
		flag = shipmentListCmd.InheritedFlags().Lookup("format")
	}
	require.NotNil(t, flag, "shipment list must have a --format flag")
	assert.Equal(t, "json", flag.DefValue,
		"--format on 'shipment list' must default to 'json' (PR-001 Option A)")
}
