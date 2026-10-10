package cli_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/cli"
)

// TestUCXS5_ShipmentShipMemberValidationProgress pins CX U16a: `shipment ship`
// reports per-member validation progress on stderr while stdout keeps the
// unchanged JSON result envelope.
func TestUCXS5_ShipmentShipMemberValidationProgress(t *testing.T) {
	root := setupCLIWorkspace(t)
	featID := cliAddFeature(t, root, "Progress feature")
	taskOne := cliAddTask(t, root, "Progress task one", featID)
	taskTwo := cliAddTask(t, root, "Progress task two", featID)
	shipID := cliCreateShipment(t, root, "Progress shipment", taskOne+","+taskTwo)
	runCLIStdout(t, root, "shipment", "claim", shipID)
	runCLIStdout(t, root, "move", taskOne, "--status", "done")
	runCLIStdout(t, root, "move", taskTwo, "--status", "done")

	cmd := cli.NewRootCommand()
	out := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	cmd.SetOut(out)
	cmd.SetErr(errBuf)
	cmd.SetArgs([]string{"--cwd", root, "shipment", "ship", shipID,
		"--sha", "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", "--message", "merge", "--author", "ship"})
	require.NoError(t, cmd.Execute(), "shipment ship failed: %s", errBuf.String())

	assert.Contains(t, errBuf.String(), fmt.Sprintf("validating member 1/2: %s", taskOne),
		"shipment ship must report member 1 of 2 on stderr")
	assert.Contains(t, errBuf.String(), fmt.Sprintf("validating member 2/2: %s", taskTwo),
		"shipment ship must report member 2 of 2 on stderr")

	var envelope map[string]any
	require.NoError(t, json.Unmarshal(out.Bytes(), &envelope), "stdout must remain a single JSON envelope")
	assert.Equal(t, shipID, envelope["shipment_id"], "stdout envelope must identify the shipment")
	assert.Equal(t, "shipped", envelope["shipment_status"], "stdout envelope must report the shipped status")
}
