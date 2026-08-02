package cli_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/cli"
	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
)

func TestNewDepCmd_HasSubcommands(t *testing.T) {
	// Act
	cmd := cli.NewDepCmd()

	// Assert
	require.NotNil(t, cmd)
	assert.Equal(t, "dep", cmd.Use)
	subNames := make([]string, 0, len(cmd.Commands()))
	for _, sub := range cmd.Commands() {
		subNames = append(subNames, sub.Name())
	}
	assert.Contains(t, subNames, "add")
	assert.Contains(t, subNames, "remove")
	assert.Contains(t, subNames, "list")
}

func TestNewDepAddCmd_RejectsMissingArgs(t *testing.T) {
	// Act
	cmd := cli.NewDepAddCmd()
	cmd.SetArgs([]string{})
	err := cmd.Execute()

	// Assert
	require.Error(t, err)
}

func TestNewDepListCmd_HasReverseFlag(t *testing.T) {
	// Act
	cmd := cli.NewDepListCmd()

	// Assert
	flag := cmd.Flags().Lookup("reverse")
	assert.NotNil(t, flag, "should have --reverse flag")
}

// --- 134.007-T: Failing CLI routing test for dep add with shipment blocks (U7 red harness) ---

// TestDepAddCLI_ShipmentBlocksRoutesToShipmentValidation verifies that
// `backlogit dep add <shipment> <non-shipment> --type blocks` returns an error
// when the CLI is routing through AddShipmentBlock validation.
//
// Before U4: `dep add` calls generic AddDependency which has no endpoint
// validation → command succeeds → test fails (expected error, got nil).
//
// After U4: `dep add` routes through AddShipmentBlock for shipment+blocks edges →
// AddShipmentBlock validates non-shipment endpoint → error → test passes.
//
// Red harness: fails until U4 wires the routing conditional in CLI dep add.
func TestDepAddCLI_ShipmentBlocksRoutesToShipmentValidation(t *testing.T) {
	root := setupShipmentDepWorkspace(t)
	ctx := context.Background()

	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)

	// Create one shipment and one feature (non-shipment).
	shipment, err := core.CreateShipment(ctx, ws, "Shipment endpoint", nil)
	require.NoError(t, err)
	feat, err := core.CreateArtifact(ctx, ws, "Non-shipment feature", "feature")
	require.NoError(t, err)
	ws.Close()

	// `dep add <shipment> <feature> --type blocks` must fail because the
	// feature is not a shipment (once the CLI routes through AddShipmentBlock).
	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	errBuf := new(bytes.Buffer)
	cmd.SetErr(errBuf)
	cmd.SetArgs([]string{"--cwd", root, "dep", "add", shipment.ID, feat.ID, "--type", "blocks"})

	err = cmd.Execute()
	require.Error(t, err,
		"dep add between a shipment and a non-shipment with --type blocks must fail "+
			"once the CLI routes through AddShipmentBlock validation (U4); "+
			"currently passes via generic AddDependency (U3 red harness)")
}

// TestDepAddCLI_NonBlocksTypeUsesGenericPath verifies that `dep add` with a
// non-blocks dep_type (e.g. relates_to) between two shipments uses the GENERIC
// AddDependency path, not AddShipmentBlock — so no endpoint validation is applied.
//
// This test must PASS both before and after U4: the routing conditional is
// blocks-only. It is a regression guard for the non-blocks non-routing path.
func TestDepAddCLI_NonBlocksTypeUsesGenericPath(t *testing.T) {
	root := setupShipmentDepWorkspace(t)
	ctx := context.Background()

	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)

	// Create one shipment and one feature (non-shipment).
	shipment, err := core.CreateShipment(ctx, ws, "Shipment for relates_to", nil)
	require.NoError(t, err)
	feat, err := core.CreateArtifact(ctx, ws, "Feature for relates_to", "feature")
	require.NoError(t, err)
	ws.Close()

	// `dep add <shipment> <feature> --type relates_to` must SUCCEED
	// because relates_to uses the generic AddDependency path regardless of types.
	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	errBuf := new(bytes.Buffer)
	cmd.SetErr(errBuf)
	cmd.SetArgs([]string{"--cwd", root, "dep", "add", shipment.ID, feat.ID, "--type", "relates_to"})

	err = cmd.Execute()
	require.NoError(t, err,
		"dep add with --type relates_to must not be routed through AddShipmentBlock; "+
			"stderr: %s", errBuf.String())
}

// setupShipmentDepWorkspace creates a minimal temp workspace for dep CLI tests
// that need shipment creation.
func setupShipmentDepWorkspace(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	backlogitDir := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogitDir, 0o755))
	require.NoError(t, config.WriteDefaults(backlogitDir))
	return root
}
