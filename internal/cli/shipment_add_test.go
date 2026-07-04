package cli_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/cli"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// runCLIErr executes a fresh root command against the workspace and returns the
// execution error (if any) instead of asserting success. Used to characterize
// failure paths such as the shared item-already-assigned sentinel.
func runCLIErr(t *testing.T, root string, args ...string) error {
	t.Helper()
	cmd := cli.NewRootCommand()
	out := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	cmd.SetOut(out)
	cmd.SetErr(errBuf)
	cmd.SetArgs(append([]string{"--cwd", root}, args...))
	return cmd.Execute()
}

// U3 (078.003-T): `backlogit shipment add <shipment-id> <item-id>` must mirror
// the MCP backlogit_add_to_shipment tool — positional args, shared core mutation,
// and an isomorphic {shipment_id, item_id, status:"added"} success shape.

// U3 scenario 1: happy-path add succeeds and its success JSON is output-shape
// isomorphic to the MCP handleAddToShipment result.
func TestShipmentAdd_HappyPath_OutputShapeParity(t *testing.T) {
	root := setupCLIWorkspace(t)
	featID := cliAddFeature(t, root, "Covering feature")
	shipID := cliCreateShipment(t, root, "Add target shipment", featID)
	taskID := cliAddTask(t, root, "Standalone task", featID)

	var m map[string]any
	require.NoError(t, json.Unmarshal(
		[]byte(runCLIStdout(t, root, "shipment", "add", shipID, taskID)), &m))

	// Isomorphic to MCP backlogit_add_to_shipment: {shipment_id, item_id, status:"added"}.
	assert.Equal(t, shipID, m["shipment_id"])
	assert.Equal(t, taskID, m["item_id"])
	assert.Equal(t, "added", m["status"])

	// The item is now a member of the shipment.
	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(runCLIStdout(t, root, "shipment", "get", shipID)), &got))
	custom, _ := got["custom_fields"].(map[string]any)
	itemsRaw, _ := custom["items"].([]any)
	items := make([]string, 0, len(itemsRaw))
	for _, it := range itemsRaw {
		if s, ok := it.(string); ok {
			items = append(items, s)
		}
	}
	assert.Contains(t, items, taskID, "added item must appear in the shipment manifest")
}

// U3 scenario 2: re-adding an item already in THIS shipment is an idempotent
// no-op success (core.AddItemToShipment returns nil).
func TestShipmentAdd_IdempotentReAdd(t *testing.T) {
	root := setupCLIWorkspace(t)
	featID := cliAddFeature(t, root, "Idempotent feature")
	shipID := cliCreateShipment(t, root, "Idempotent shipment", featID)
	taskID := cliAddTask(t, root, "Re-add task", featID)

	_ = runCLIStdout(t, root, "shipment", "add", shipID, taskID)

	// Second add of the same item to the same shipment succeeds as a no-op.
	var m map[string]any
	require.NoError(t, json.Unmarshal(
		[]byte(runCLIStdout(t, root, "shipment", "add", shipID, taskID)), &m))
	assert.Equal(t, "added", m["status"])
}

// U3 scenario 3: adding an item already assigned to ANOTHER shipment fails with
// the shared blerrors.ErrItemAlreadyAssigned sentinel (asserted via errors.Is,
// never string matching).
func TestShipmentAdd_ItemInAnotherShipment_SentinelError(t *testing.T) {
	root := setupCLIWorkspace(t)
	featA := cliAddFeature(t, root, "Feature A")
	featB := cliAddFeature(t, root, "Feature B")
	shipA := cliCreateShipment(t, root, "Shipment A", featA)
	shipB := cliCreateShipment(t, root, "Shipment B", featB)
	taskID := cliAddTask(t, root, "Contended task", featA)

	_ = runCLIStdout(t, root, "shipment", "add", shipA, taskID)

	err := runCLIErr(t, root, "shipment", "add", shipB, taskID)
	require.Error(t, err)
	assert.ErrorIs(t, err, blerrors.ErrItemAlreadyAssigned,
		"cross-shipment conflict must surface the shared sentinel, not a string match")
}

// U3: the add subcommand is registered and discoverable in `shipment --help`.
func TestShipmentAdd_RegisteredInShipmentGroup(t *testing.T) {
	cmd := cli.NewShipmentCmd()
	names := make([]string, 0)
	for _, sub := range cmd.Commands() {
		names = append(names, sub.Name())
	}
	assert.Contains(t, names, "add", "shipment group must register the add subcommand")
}
