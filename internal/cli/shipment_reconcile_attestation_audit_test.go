package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/events"
)

// unpinnedAttestationEnvelope parses but names a signing key no trust anchor
// declares, so loading fails before any workspace is opened.
const unpinnedAttestationEnvelope = `{"payloadType":"application/vnd.in-toto+json","payload":"e30=","signatures":[{"keyid":"unpinned-key","sig":"AA==","sig_algo":"ed25519"}]}`

func TestShipmentReconcileShipped_AttestationLoadFailureAudit(t *testing.T) {
	reconcileShippedForceNonInteractiveStdin(t)
	root := reconcileShippedSetupWorkspace(t)
	shipmentID := reconcileShippedBuildFixture(t, root)
	attestationPath := filepath.Join(root, "attestation.json")
	require.NoError(t, os.WriteFile(attestationPath, []byte(unpinnedAttestationEnvelope), 0o644))
	logPath := events.LogPathForItem(core.WorkspaceLogsRoot(root), shipmentID)

	readLog := func() string {
		data, err := os.ReadFile(logPath)
		if os.IsNotExist(err) {
			return ""
		}
		require.NoError(t, err)
		return string(data)
	}

	t.Run("dry run stays write-free", func(t *testing.T) {
		before := readLog()
		args := append(reconcileShippedValidArgs(shipmentID), "--dry-run", "--attestation", attestationPath)
		_, _, err := reconcileShippedRunCmd(t, root, args...)
		require.Error(t, err)
		assert.Equal(t, before, readLog())
	})

	t.Run("live run audits the rejection", func(t *testing.T) {
		args := append(reconcileShippedValidArgs(shipmentID), "--confirm", "reconcile-shipped "+shipmentID, "--attestation", attestationPath)
		_, _, err := reconcileShippedRunCmd(t, root, args...)
		require.Error(t, err)
		assert.Contains(t, readLog(), core.EventAttestationRejected)
		assert.Contains(t, readLog(), "unpinned-key")
	})
}
