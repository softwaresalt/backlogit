package mcp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
)

func TestExternalMCP_EnvironmentPathsOnly(t *testing.T) {
	ctx := context.Background()
	repo := custodyTestNewRepo(t)
	srv := NewServerForRoot(repo.root)
	inside := filepath.Join(repo.root, ".backlogit", "operator-pins.yaml")
	require.NoError(t, os.WriteFile(inside, []byte("pins: []\n"), 0o600))
	missing := filepath.Join(t.TempDir(), "missing-pins.yaml")

	t.Run("env_pins_path_inside_workspace_refused", func(t *testing.T) {
		t.Setenv(core.CustodyOperatorPinsEnv, inside)
		result := custodyTestInvoke(t, ctx, srv, "backlogit_custody_verify", map[string]any{"shipment_id": repo.stageShip})
		assert.Equal(t, "validation_failed", custodyTestErrorType(t, result))
		assert.True(t, strings.Contains(custodyTestText(t, result), "inside workspace root"), custodyTestText(t, result))
	})
	t.Run("env_missing_pins_file_fails_closed", func(t *testing.T) {
		t.Setenv(core.CustodyOperatorPinsEnv, missing)
		result := custodyTestInvoke(t, ctx, srv, "backlogit_custody_verify", map[string]any{"shipment_id": repo.stageShip})
		assert.Equal(t, "validation_failed", custodyTestErrorType(t, result))
	})
	t.Run("tool_arguments_cannot_choose_external_files", func(t *testing.T) {
		t.Setenv(core.CustodyOperatorPinsEnv, "")
		t.Setenv(core.CustodyNonceStoreEnv, "")
		result := custodyTestInvoke(t, ctx, srv, "backlogit_custody_verify", map[string]any{
			"shipment_id": repo.stageShip, "operator_pins_file": inside, "nonce_store_path": inside,
		})
		assert.False(t, result.IsError, "a remote caller must not choose the pins file or nonce store: %s", custodyTestText(t, result))
	})
	t.Run("env_nonce_store_inside_workspace_refused", func(t *testing.T) {
		t.Setenv(core.CustodyNonceStoreEnv, inside)
		result := custodyTestInvoke(t, ctx, srv, "backlogit_custody_accept", map[string]any{
			"shipment_id": repo.stageShip, "receipt_id": "rcpt-stage-ship-1", "by": "ship", "dry_run": true,
		})
		assert.Equal(t, "validation_failed", custodyTestErrorType(t, result))
		assert.True(t, strings.Contains(custodyTestText(t, result), "inside workspace root"), custodyTestText(t, result))
	})
}
