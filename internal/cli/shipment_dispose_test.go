package cli_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// The shipment dispose command is a Stage-only operation (G-B). These tests
// drive the real cobra command tree in process on temp-dir fixtures only; they
// never run the command against a real workspace.

// disposeCLIFixture creates a queued shipment and a decided deliberation that
// authorizes disposing it, returning (root, shipmentID, deliberationID).
func disposeCLIFixture(t *testing.T) (string, string, string) {
	t.Helper()
	root := setupCLIWorkspace(t)
	featID := cliAddFeature(t, root, "Dispose feature")
	taskID := cliAddTask(t, root, "Dispose member", featID)
	shipID := cliCreateShipment(t, root, "Dispose target", taskID)

	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)
	delib, err := core.CreateArtifact(ctx, ws, "Disposition authorization", "deliberation",
		core.WithStatus("done"),
		core.WithFields(map[string]any{"authorizes_disposition": shipID}))
	require.NoError(t, err)
	require.NoError(t, ws.Close())
	return root, shipID, delib.ID
}

func TestShipmentDispose_SuccessPrintsAbandonedShipment(t *testing.T) {
	root, shipID, delibID := disposeCLIFixture(t)

	out := runCLIStdout(t, root, "shipment", "dispose", shipID,
		"--reason", "superseded by a newer plan", "--by", "operator",
		"--authorization-ref", delibID, "--confirm")

	var m map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &m), out)
	assert.Equal(t, shipID, m["id"])
	assert.Equal(t, "abandoned", m["status"])
}

func TestShipmentDispose_MissingConfirmRequiresConfirmation(t *testing.T) {
	root, shipID, delibID := disposeCLIFixture(t)

	err := runCLIErr(t, root, "shipment", "dispose", shipID,
		"--reason", "superseded", "--by", "operator", "--authorization-ref", delibID)

	require.Error(t, err)
	assert.ErrorIs(t, err, blerrors.ErrConfirmationRequired)
}

func TestShipmentDispose_ValidationFailures(t *testing.T) {
	root, shipID, delibID := disposeCLIFixture(t)

	tests := []struct {
		name string
		args []string
	}{
		{"empty reason", []string{"--reason", "", "--by", "operator", "--authorization-ref", delibID, "--confirm"}},
		{"unbound authorization", []string{"--reason", "superseded", "--by", "operator", "--authorization-ref", "999-DL", "--confirm"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := runCLIErr(t, root, append([]string{"shipment", "dispose", shipID}, tc.args...)...)
			require.Error(t, err)
			assert.ErrorIs(t, err, blerrors.ErrValidation)
		})
	}
}

func TestShipmentDispose_StateRefusal(t *testing.T) {
	root, shipID, delibID := disposeCLIFixture(t)
	args := []string{"shipment", "dispose", shipID,
		"--reason", "superseded", "--by", "operator", "--authorization-ref", delibID, "--confirm"}

	runCLIStdout(t, root, args...)
	err := runCLIErr(t, root, args...)

	require.Error(t, err)
	assert.ErrorIs(t, err, blerrors.ErrShipmentDispositionRefused)
}
